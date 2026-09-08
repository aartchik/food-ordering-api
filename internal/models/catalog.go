package models

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"food-ordering-api/internal/validator"
	"github.com/lib/pq"
)

func (m CatalogModel) Update(partnerID string, input *RestaurantCatalogInput) (*Restaurant, []*MenuItem, error) {
	v := validator.New()
	ValidateRestaurantCatalogInput(v, input)
	if !v.Valid() || partnerID == "" {
		return nil, nil, ErrInvalidInput
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("rollback transaction: %v", err)
		}
	}()

	restaurant, err := upsertCatalogRestaurant(ctx, tx, partnerID, input.Restaurant)
	if err != nil {
		return nil, nil, err
	}

	partnerItemIDs := make([]string, 0, len(input.Items))
	for _, item := range input.Items {
		partnerItemIDs = append(partnerItemIDs, item.PartnerItemID)
		if err := upsertCatalogItem(ctx, tx, restaurant.ID, item); err != nil {
			return nil, nil, err
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE menu_items
		SET is_available = false, version = version + 1, updated_at = now()
		WHERE restaurant_id = $1
		  AND NOT (partner_item_id = ANY($2))
		  AND is_available = true`, restaurant.ID, pq.Array(partnerItemIDs))
	if err != nil {
		return nil, nil, err
	}

	items, err := catalogItems(ctx, tx, restaurant.ID)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	return restaurant, items, nil
}

type catalogTx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func upsertCatalogRestaurant(ctx context.Context, tx catalogTx, partnerID string, input RestaurantInput) (*Restaurant, error) {
	query := `
		INSERT INTO restaurants (partner_id, name, description, address, is_open)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (partner_id) DO UPDATE SET
			name = EXCLUDED.name, description = EXCLUDED.description,
			address = EXCLUDED.address, is_open = EXCLUDED.is_open,
			version = restaurants.version + 1, updated_at = now()
		RETURNING id, partner_id, name, description, address, is_open, version, created_at, updated_at`
	restaurant := &Restaurant{}
	err := tx.QueryRowContext(ctx, query, partnerID, input.Name, input.Description, input.Address, input.IsOpen).Scan(
		&restaurant.ID, &restaurant.PartnerID, &restaurant.Name, &restaurant.Description,
		&restaurant.Address, &restaurant.IsOpen, &restaurant.Version,
		&restaurant.CreatedAt, &restaurant.UpdatedAt,
	)
	return restaurant, err
}

func upsertCatalogItem(ctx context.Context, tx catalogTx, restaurantID int64, item *MenuItemInput) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO menu_items (restaurant_id, partner_item_id, name, description, price_kopecks, is_available, preparation_min)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (restaurant_id, partner_item_id) DO UPDATE SET
			name = EXCLUDED.name, description = EXCLUDED.description,
			price_kopecks = EXCLUDED.price_kopecks, is_available = EXCLUDED.is_available,
			preparation_min = EXCLUDED.preparation_min,
			version = menu_items.version + 1, updated_at = now()`,
		restaurantID, item.PartnerItemID, item.Name, item.Description,
		item.PriceKopecks, item.IsAvailable, item.PreparationMin,
	)
	return err
}

func catalogItems(ctx context.Context, tx catalogTx, restaurantID int64) ([]*MenuItem, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, restaurant_id, partner_item_id, name, description, price_kopecks,
			is_available, preparation_min, version, created_at, updated_at
		FROM menu_items WHERE restaurant_id = $1 ORDER BY partner_item_id`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("close query rows: %v", err)
		}
	}()

	items := []*MenuItem{}
	for rows.Next() {
		item := &MenuItem{}
		if err := rows.Scan(&item.ID, &item.RestaurantID, &item.PartnerItemID, &item.Name,
			&item.Description, &item.PriceKopecks, &item.IsAvailable, &item.PreparationMin,
			&item.Version, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
