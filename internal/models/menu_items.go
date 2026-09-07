package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"food-ordering-api/internal/validator"
)

type MenuItem struct {
	ID             int64     `json:"id"`
	RestaurantID   int64     `json:"restaurant_id"`
	PartnerItemID  string    `json:"partner_item_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	PriceKopecks   int       `json:"price_kopecks"`
	IsAvailable    bool      `json:"is_available"`
	PreparationMin int       `json:"preparation_min"`
	Version        int32     `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type MenuItemInput struct {
	PartnerItemID  string `json:"partner_item_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	PriceKopecks   int    `json:"price_kopecks"`
	IsAvailable    bool   `json:"is_available"`
	PreparationMin int    `json:"preparation_min"`
}

func (m MenuItemModel) UpsertForRestaurant(restaurantID int64, items []*MenuItemInput) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt := `
		INSERT INTO menu_items (
			restaurant_id,
			partner_item_id,
			name,
			description,
			price_kopecks,
			is_available,
			preparation_min
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (restaurant_id, partner_item_id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			price_kopecks = EXCLUDED.price_kopecks,
			is_available = EXCLUDED.is_available,
			preparation_min = EXCLUDED.preparation_min,
			version = menu_items.version + 1,
			updated_at = now()`

	for _, item := range items {
		if item == nil {
			continue
		}

		_, err := tx.ExecContext(ctx, stmt,
			restaurantID,
			item.PartnerItemID,
			item.Name,
			item.Description,
			item.PriceKopecks,
			item.IsAvailable,
			item.PreparationMin,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m MenuItemModel) Get(id int64) (*MenuItem, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id, restaurant_id, partner_item_id, name, description, price_kopecks,
			is_available, preparation_min, version, created_at, updated_at
		FROM menu_items
		WHERE id = $1`

	var item MenuItem

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.RestaurantID,
		&item.PartnerItemID,
		&item.Name,
		&item.Description,
		&item.PriceKopecks,
		&item.IsAvailable,
		&item.PreparationMin,
		&item.Version,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &item, nil
}

func (m MenuItemModel) GetAllForRestaurant(restaurantID int64, availableOnly bool) ([]*MenuItem, error) {
	query := `
		SELECT id, restaurant_id, partner_item_id, name, description, price_kopecks,
			is_available, preparation_min, version, created_at, updated_at
		FROM menu_items
		WHERE restaurant_id = $1
		  AND ($2 = false OR is_available = true)
		ORDER BY name ASC, id ASC`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, restaurantID, availableOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []*MenuItem{}

	for rows.Next() {
		var item MenuItem

		err := rows.Scan(
			&item.ID,
			&item.RestaurantID,
			&item.PartnerItemID,
			&item.Name,
			&item.Description,
			&item.PriceKopecks,
			&item.IsAvailable,
			&item.PreparationMin,
			&item.Version,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func ValidateMenuItemInput(v *validator.Validator, item *MenuItemInput) {
	if item == nil {
		v.AddError("items", "must not contain null items")
		return
	}

	v.Check(validator.NotBlank(item.PartnerItemID), "partner_item_id", "must be provided")
	v.Check(validator.MaxChars(item.PartnerItemID, 120), "partner_item_id", "must not be more than 120 characters long")
	v.Check(validator.NotBlank(item.Name), "name", "must be provided")
	v.Check(validator.MaxChars(item.Name, 120), "name", "must not be more than 120 characters long")
	v.Check(validator.MaxChars(item.Description, 500), "description", "must not be more than 500 characters long")
	v.Check(item.PriceKopecks > 0, "price_kopecks", "must be greater than zero")
	v.Check(item.PreparationMin >= 0 && item.PreparationMin <= 240, "preparation_min", "must be between 0 and 240")
}
