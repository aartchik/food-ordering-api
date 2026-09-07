package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"food-ordering-api/internal/validator"
)

type Restaurant struct {
	ID          int64     `json:"id"`
	PartnerID   string    `json:"partner_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Address     string    `json:"address"`
	IsOpen      bool      `json:"is_open"`
	Version     int32     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RestaurantCatalogInput struct {
	Restaurant Restaurant       `json:"restaurant"`
	Items      []*MenuItemInput `json:"items"`
}

type RestaurantListFilter struct {
	Query    string
	OnlyOpen bool
	Filters
}

func (m RestaurantModel) UpsertByPartnerID(partnerID string, restaurant *Restaurant) error {
	query := `
		INSERT INTO restaurants (partner_id, name, description, address, is_open)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (partner_id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			address = EXCLUDED.address,
			is_open = EXCLUDED.is_open,
			version = restaurants.version + 1,
			updated_at = now()
		RETURNING id, partner_id, name, description, address, is_open, version, created_at, updated_at`

	args := []any{
		partnerID,
		restaurant.Name,
		restaurant.Description,
		restaurant.Address,
		restaurant.IsOpen,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.DB.QueryRowContext(ctx, query, args...).Scan(
		&restaurant.ID,
		&restaurant.PartnerID,
		&restaurant.Name,
		&restaurant.Description,
		&restaurant.Address,
		&restaurant.IsOpen,
		&restaurant.Version,
		&restaurant.CreatedAt,
		&restaurant.UpdatedAt,
	)
}

func (m RestaurantModel) Get(id int64) (*Restaurant, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id, partner_id, name, description, address, is_open, version, created_at, updated_at
		FROM restaurants
		WHERE id = $1`

	var restaurant Restaurant

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&restaurant.ID,
		&restaurant.PartnerID,
		&restaurant.Name,
		&restaurant.Description,
		&restaurant.Address,
		&restaurant.IsOpen,
		&restaurant.Version,
		&restaurant.CreatedAt,
		&restaurant.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &restaurant, nil
}

func (m RestaurantModel) GetAll(input RestaurantListFilter) ([]*Restaurant, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(*) OVER(), id, partner_id, name, description, address, is_open, version, created_at, updated_at
		FROM restaurants
		WHERE ($1 = '' OR to_tsvector('simple', name || ' ' || description) @@ plainto_tsquery('simple', $1))
		  AND ($2 = false OR is_open = true)
		ORDER BY %s %s, id ASC
		LIMIT $3 OFFSET $4`, input.SortColumn(), input.SortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, input.Query, input.OnlyOpen, input.Limit(), input.Offset())
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	restaurants := []*Restaurant{}

	for rows.Next() {
		var restaurant Restaurant

		err := rows.Scan(
			&totalRecords,
			&restaurant.ID,
			&restaurant.PartnerID,
			&restaurant.Name,
			&restaurant.Description,
			&restaurant.Address,
			&restaurant.IsOpen,
			&restaurant.Version,
			&restaurant.CreatedAt,
			&restaurant.UpdatedAt,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		restaurants = append(restaurants, &restaurant)
	}

	if err := rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := CalculateMetadata(totalRecords, input.Page, input.PageSize)
	return restaurants, metadata, nil
}

func ValidateRestaurant(v *validator.Validator, restaurant *Restaurant) {
	v.Check(validator.NotBlank(restaurant.Name), "name", "must be provided")
	v.Check(validator.MaxChars(restaurant.Name, 120), "name", "must not be more than 120 characters long")
	v.Check(validator.MaxChars(restaurant.Description, 500), "description", "must not be more than 500 characters long")
	v.Check(validator.MaxChars(restaurant.Address, 300), "address", "must not be more than 300 characters long")
}

func ValidateRestaurantCatalogInput(v *validator.Validator, input *RestaurantCatalogInput) {
	ValidateRestaurant(v, &input.Restaurant)
	v.Check(len(input.Items) > 0, "items", "must contain at least one menu item")
	v.Check(len(input.Items) <= 500, "items", "must not contain more than 500 menu items")

	partnerItemIDs := make([]string, 0, len(input.Items))
	for _, item := range input.Items {
		ValidateMenuItemInput(v, item)
		if item == nil {
			v.AddError("items", "must not contain null items")
			continue
		}
		partnerItemIDs = append(partnerItemIDs, item.PartnerItemID)
		if !validator.NotBlank(item.PartnerItemID) {
			v.AddError("items", "item partner ids must be provided")
		}
	}

	v.Check(validator.Unique(partnerItemIDs), "items", "item partner ids must be unique")
}
