package models

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"food-ordering-api/internal/validator"
)

type Cart struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CartItem struct {
	ID           int64     `json:"id"`
	CartID       int64     `json:"cart_id"`
	MenuItemID   int64     `json:"menu_item_id"`
	Quantity     int       `json:"quantity"`
	Name         string    `json:"name"`
	PriceKopecks int       `json:"price_kopecks"`
	CreatedAt    time.Time `json:"created_at"`
}

type CartView struct {
	Cart         *Cart       `json:"cart"`
	Items        []*CartItem `json:"items"`
	TotalKopecks int         `json:"total_kopecks"`
}

type CartItemInput struct {
	MenuItemID int64 `json:"menu_item_id"`
	Quantity   int   `json:"quantity"`
}

func (m CartModel) Insert(userID string) (*CartView, error) {
	query := `
		INSERT INTO carts (user_id)
		VALUES (NULLIF($1, ''))
		RETURNING id, COALESCE(user_id, ''), created_at, updated_at`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cart := &Cart{}
	err := m.DB.QueryRowContext(ctx, query, userID).Scan(
		&cart.ID,
		&cart.UserID,
		&cart.CreatedAt,
		&cart.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &CartView{
		Cart:  cart,
		Items: []*CartItem{},
	}, nil
}

func (m CartModel) Get(id int64) (*CartView, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id, COALESCE(user_id, ''), created_at, updated_at
		FROM carts
		WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cart := &Cart{}
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&cart.ID,
		&cart.UserID,
		&cart.CreatedAt,
		&cart.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	items, total, err := m.getItems(ctx, id)
	if err != nil {
		return nil, err
	}

	return &CartView{
		Cart:         cart,
		Items:        items,
		TotalKopecks: total,
	}, nil
}

func (m CartModel) AddItem(cartID, menuItemID int64, quantity int) (*CartView, error) {
	if cartID < 1 || menuItemID < 1 || quantity < 1 || quantity > 99 {
		return nil, ErrInvalidInput
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("rollback transaction: %v", err)
		}
	}()

	var lockedCartID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM carts WHERE id = $1 FOR UPDATE`, cartID).Scan(&lockedCartID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	item, err := getAvailableMenuItemForCart(ctx, tx, menuItemID)
	if err != nil {
		return nil, err
	}

	err = checkCartCanAcceptRestaurant(ctx, tx, cartID, item.RestaurantID)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO cart_items (cart_id, menu_item_id, quantity, price_kopecks_snapshot, name_snapshot)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cart_id, menu_item_id) DO UPDATE SET
			quantity = cart_items.quantity + EXCLUDED.quantity,
			price_kopecks_snapshot = EXCLUDED.price_kopecks_snapshot,
			name_snapshot = EXCLUDED.name_snapshot
		WHERE cart_items.quantity + EXCLUDED.quantity <= 99`

	result, err := tx.ExecContext(ctx, query, cartID, menuItemID, quantity, item.PriceKopecks, item.Name)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, ErrInvalidInput
	}

	_, err = tx.ExecContext(ctx, `UPDATE carts SET updated_at = now() WHERE id = $1`, cartID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return m.Get(cartID)
}

func (m CartModel) UpdateItem(cartID, menuItemID int64, quantity int) (*CartView, error) {
	if cartID < 1 || menuItemID < 1 || quantity < 1 || quantity > 99 {
		return nil, ErrInvalidInput
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("rollback transaction: %v", err)
		}
	}()

	var lockedCartID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM carts WHERE id = $1 FOR UPDATE`, cartID).Scan(&lockedCartID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	item, err := getAvailableMenuItemForCart(ctx, tx, menuItemID)
	if err != nil {
		return nil, err
	}

	query := `
		UPDATE cart_items
		SET quantity = $3,
			name_snapshot = $4,
			price_kopecks_snapshot = $5
		WHERE cart_id = $1 AND menu_item_id = $2`

	result, err := tx.ExecContext(ctx, query, cartID, menuItemID, quantity, item.Name, item.PriceKopecks)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, ErrRecordNotFound
	}

	_, err = tx.ExecContext(ctx, `UPDATE carts SET updated_at = now() WHERE id = $1`, cartID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return m.Get(cartID)
}

func (m CartModel) DeleteItem(cartID, menuItemID int64) (*CartView, error) {
	if cartID < 1 || menuItemID < 1 {
		return nil, ErrInvalidInput
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("rollback transaction: %v", err)
		}
	}()

	query := `
		DELETE FROM cart_items
		WHERE cart_id = $1 AND menu_item_id = $2`

	result, err := tx.ExecContext(ctx, query, cartID, menuItemID)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, ErrRecordNotFound
	}

	_, err = tx.ExecContext(ctx, `UPDATE carts SET updated_at = now() WHERE id = $1`, cartID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return m.Get(cartID)
}

func ValidateCartItemInput(v *validator.Validator, input *CartItemInput) {
	v.Check(input.MenuItemID > 0, "menu_item_id", "must be provided")
	v.Check(input.Quantity >= 1 && input.Quantity <= 99, "quantity", "must be between 1 and 99")
}

func (m CartModel) getItems(ctx context.Context, cartID int64) ([]*CartItem, int, error) {
	query := `
		SELECT id, cart_id, menu_item_id, quantity, name_snapshot, price_kopecks_snapshot, created_at
		FROM cart_items
		WHERE cart_id = $1
		ORDER BY created_at ASC, id ASC`

	rows, err := m.DB.QueryContext(ctx, query, cartID)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("close query rows: %v", err)
		}
	}()

	items := []*CartItem{}
	total := 0

	for rows.Next() {
		item := &CartItem{}
		err := rows.Scan(
			&item.ID,
			&item.CartID,
			&item.MenuItemID,
			&item.Quantity,
			&item.Name,
			&item.PriceKopecks,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		total += item.Quantity * item.PriceKopecks
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

type cartTx interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getAvailableMenuItemForCart(ctx context.Context, tx cartTx, menuItemID int64) (*MenuItem, error) {
	query := `
		SELECT id, restaurant_id, partner_item_id, name, description, price_kopecks,
			is_available, preparation_min, version, created_at, updated_at
		FROM menu_items
		WHERE id = $1`

	item := &MenuItem{}
	err := tx.QueryRowContext(ctx, query, menuItemID).Scan(
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
	if !item.IsAvailable {
		return nil, ErrItemUnavailable
	}

	return item, nil
}

func checkCartCanAcceptRestaurant(ctx context.Context, tx cartTx, cartID, restaurantID int64) error {
	var cartExists bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM carts WHERE id = $1)`, cartID).Scan(&cartExists)
	if err != nil {
		return err
	}
	if !cartExists {
		return ErrRecordNotFound
	}

	var mixed bool
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM cart_items ci
			JOIN menu_items mi ON mi.id = ci.menu_item_id
			WHERE ci.cart_id = $1 AND mi.restaurant_id <> $2
		)`
	err = tx.QueryRowContext(ctx, query, cartID, restaurantID).Scan(&mixed)
	if err != nil {
		return err
	}
	if mixed {
		return ErrMixedCart
	}

	return nil
}
