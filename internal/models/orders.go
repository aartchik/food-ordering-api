package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"food-ordering-api/internal/validator"
	"github.com/lib/pq"
)

type Customer struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	Comment string `json:"comment,omitempty"`
}

type Order struct {
	ID             int64     `json:"id"`
	CartID         int64     `json:"cart_id"`
	RestaurantID   int64     `json:"restaurant_id"`
	PartnerID      string    `json:"partner_id"`
	PartnerOrderID string    `json:"partner_order_id,omitempty"`
	Status         string    `json:"status"`
	Customer       Customer  `json:"customer"`
	TotalKopecks   int       `json:"total_kopecks"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	Version        int32     `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type OrderItem struct {
	ID            int64     `json:"id"`
	OrderID       int64     `json:"order_id"`
	MenuItemID    int64     `json:"menu_item_id"`
	PartnerItemID string    `json:"partner_item_id"`
	Name          string    `json:"name"`
	Quantity      int       `json:"quantity"`
	PriceKopecks  int       `json:"price_kopecks"`
	CreatedAt     time.Time `json:"created_at"`
}

type OrderView struct {
	Order *Order       `json:"order"`
	Items []*OrderItem `json:"items"`
}

type CheckoutInput struct {
	CartID         int64    `json:"cart_id"`
	Customer       Customer `json:"customer"`
	IdempotencyKey string   `json:"idempotency_key"`
}

type OrderStatusUpdateInput struct {
	Status         string `json:"status"`
	PartnerOrderID string `json:"partner_order_id"`
	Version        int32  `json:"version"`
}

type PartnerOrderListFilter struct {
	Status string
	Filters
}

func (m OrderModel) CreateFromCart(input *CheckoutInput) (*OrderView, error) {
	v := validator.New()
	ValidateCheckoutInput(v, input)
	if !v.Valid() {
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

	// Serialize checkout requests before looking up the idempotency key.
	var cartID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM carts WHERE id = $1 FOR UPDATE`, input.CartID).Scan(&cartID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	existingOrderID, err := findOrderByIdempotencyKey(ctx, tx, input.CartID, input.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if existingOrderID > 0 {
		err = tx.Commit()
		if err != nil {
			return nil, err
		}
		return m.Get(existingOrderID)
	}

	items, restaurantID, partnerID, total, err := checkoutItemsFromCart(ctx, tx, input.CartID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrCartIsEmpty
	}

	order, err := insertOrder(ctx, tx, input, restaurantID, partnerID, total)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		err = insertOrderItem(ctx, tx, order.ID, item)
		if err != nil {
			return nil, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &OrderView{
		Order: order,
		Items: items,
	}, nil
}

func (m OrderModel) Get(id int64) (*OrderView, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	order, err := m.getOrder(ctx, id)
	if err != nil {
		return nil, err
	}

	items, err := m.getItems(ctx, id)
	if err != nil {
		return nil, err
	}

	return &OrderView{
		Order: order,
		Items: items,
	}, nil
}

func (m OrderModel) GetForPartner(partnerID string, id int64) (*OrderView, error) {
	if partnerID == "" || id < 1 {
		return nil, ErrRecordNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	order, err := m.getOrderForPartner(ctx, partnerID, id)
	if err != nil {
		return nil, err
	}
	items, err := m.getItems(ctx, id)
	if err != nil {
		return nil, err
	}
	return &OrderView{Order: order, Items: items}, nil
}

func (m OrderModel) GetAllForPartner(partnerID string, input PartnerOrderListFilter) ([]*OrderView, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(*) OVER(), id, cart_id, restaurant_id, partner_id, COALESCE(partner_order_id, ''),
			status, customer_name, customer_phone, delivery_address, COALESCE(customer_comment, ''),
			total_kopecks, idempotency_key, version, created_at, updated_at
		FROM orders
		WHERE partner_id = $1 AND ($2 = '' OR status = $2)
		ORDER BY %s %s, id ASC
		LIMIT $3 OFFSET $4`, input.SortColumn(), input.SortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := m.DB.QueryContext(ctx, query, partnerID, input.Status, input.Limit(), input.Offset())
	if err != nil {
		return nil, Metadata{}, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("close query rows: %v", err)
		}
	}()

	totalRecords := 0
	orders := []*Order{}
	orderIDs := []int64{}
	for rows.Next() {
		order := &Order{}
		if err := rows.Scan(&totalRecords, &order.ID, &order.CartID, &order.RestaurantID,
			&order.PartnerID, &order.PartnerOrderID, &order.Status, &order.Customer.Name,
			&order.Customer.Phone, &order.Customer.Address, &order.Customer.Comment,
			&order.TotalKopecks, &order.IdempotencyKey, &order.Version,
			&order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, Metadata{}, err
		}
		orders = append(orders, order)
		orderIDs = append(orderIDs, order.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, Metadata{}, err
	}
	if err := rows.Close(); err != nil {
		return nil, Metadata{}, err
	}

	itemsByOrderID, err := m.getItemsForOrders(ctx, orderIDs)
	if err != nil {
		return nil, Metadata{}, err
	}
	views := make([]*OrderView, 0, len(orders))
	for _, order := range orders {
		items := itemsByOrderID[order.ID]
		if items == nil {
			items = []*OrderItem{}
		}
		views = append(views, &OrderView{Order: order, Items: items})
	}

	return views, CalculateMetadata(totalRecords, input.Page, input.PageSize), nil
}

func (m OrderModel) GetAllByCustomerPhone(phone string, filters Filters) ([]*OrderView, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(*) OVER(), id
		FROM orders
		WHERE customer_phone = $1
		ORDER BY %s %s, id ASC
		LIMIT $2 OFFSET $3`, filters.SortColumn(), filters.SortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, phone, filters.Limit(), filters.Offset())
	if err != nil {
		return nil, Metadata{}, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("close query rows: %v", err)
		}
	}()

	totalRecords := 0
	orderIDs := []int64{}

	for rows.Next() {
		var orderID int64
		err := rows.Scan(&totalRecords, &orderID)
		if err != nil {
			return nil, Metadata{}, err
		}
		orderIDs = append(orderIDs, orderID)
	}

	if err := rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	orders := make([]*OrderView, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		order, err := m.Get(orderID)
		if err != nil {
			return nil, Metadata{}, err
		}
		orders = append(orders, order)
	}

	metadata := CalculateMetadata(totalRecords, filters.Page, filters.PageSize)
	return orders, metadata, nil
}

func (m OrderModel) UpdateStatusForPartner(partnerID string, id int64, input *OrderStatusUpdateInput) (*OrderView, error) {
	if partnerID == "" || id < 1 {
		return nil, ErrRecordNotFound
	}

	v := validator.New()
	ValidateOrderStatusUpdateInput(v, input)
	if !v.Valid() {
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

	var currentStatus string
	var currentVersion int32
	err = tx.QueryRowContext(ctx, `
		SELECT status, version FROM orders
		WHERE id = $1 AND partner_id = $2
		FOR UPDATE`, id, partnerID).Scan(&currentStatus, &currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}
	if currentVersion != input.Version {
		return nil, ErrEditConflict
	}
	if !CanTransitionOrderStatus(currentStatus, input.Status) {
		return nil, ErrInvalidTransition
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE orders
		SET status = $3,
			partner_order_id = COALESCE(NULLIF($4, ''), partner_order_id),
			version = version + 1,
			updated_at = now()
		WHERE id = $1 AND partner_id = $2`, id, partnerID, input.Status, input.PartnerOrderID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return m.GetForPartner(partnerID, id)
}

func ValidateCheckoutInput(v *validator.Validator, input *CheckoutInput) {
	if input == nil {
		v.AddError("body", "must be provided")
		return
	}

	v.Check(input.CartID > 0, "cart_id", "must be provided")
	v.Check(validator.NotBlank(input.Customer.Name), "customer.name", "must be provided")
	v.Check(validator.MaxChars(input.Customer.Name, 120), "customer.name", "must not be more than 120 characters long")
	v.Check(validator.NotBlank(input.Customer.Phone), "customer.phone", "must be provided")
	v.Check(validator.MaxChars(input.Customer.Phone, 32), "customer.phone", "must not be more than 32 characters long")
	v.Check(validator.NotBlank(input.Customer.Address), "customer.address", "must be provided")
	v.Check(validator.MaxChars(input.Customer.Address, 300), "customer.address", "must not be more than 300 characters long")
	v.Check(validator.MaxChars(input.Customer.Comment, 500), "customer.comment", "must not be more than 500 characters long")
	v.Check(validator.NotBlank(input.IdempotencyKey), "idempotency_key", "must be provided")
	v.Check(validator.MaxChars(input.IdempotencyKey, 120), "idempotency_key", "must not be more than 120 characters long")
}

func ValidateOrderStatusUpdateInput(v *validator.Validator, input *OrderStatusUpdateInput) {
	if input == nil {
		v.AddError("body", "must be provided")
		return
	}

	ValidateOrderStatus(v, input.Status)
	v.Check(validator.MaxChars(input.PartnerOrderID, 120), "partner_order_id", "must not be more than 120 characters long")
	v.Check(input.Version > 0, "version", "must be provided")
	if input.Status == OrderStatusAccepted {
		v.Check(validator.NotBlank(input.PartnerOrderID), "partner_order_id", "must be provided when accepting an order")
	}
}

func (m OrderModel) getOrder(ctx context.Context, id int64) (*Order, error) {
	query := `
		SELECT id, cart_id, restaurant_id, partner_id, COALESCE(partner_order_id, ''),
			status, customer_name, customer_phone, delivery_address, COALESCE(customer_comment, ''),
			total_kopecks, idempotency_key, version, created_at, updated_at
		FROM orders
		WHERE id = $1`

	order := &Order{}
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&order.ID,
		&order.CartID,
		&order.RestaurantID,
		&order.PartnerID,
		&order.PartnerOrderID,
		&order.Status,
		&order.Customer.Name,
		&order.Customer.Phone,
		&order.Customer.Address,
		&order.Customer.Comment,
		&order.TotalKopecks,
		&order.IdempotencyKey,
		&order.Version,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return order, nil
}

func (m OrderModel) getOrderForPartner(ctx context.Context, partnerID string, id int64) (*Order, error) {
	query := `
		SELECT id, cart_id, restaurant_id, partner_id, COALESCE(partner_order_id, ''),
			status, customer_name, customer_phone, delivery_address, COALESCE(customer_comment, ''),
			total_kopecks, idempotency_key, version, created_at, updated_at
		FROM orders
		WHERE id = $1 AND partner_id = $2`
	order := &Order{}
	err := m.DB.QueryRowContext(ctx, query, id, partnerID).Scan(
		&order.ID, &order.CartID, &order.RestaurantID, &order.PartnerID,
		&order.PartnerOrderID, &order.Status, &order.Customer.Name, &order.Customer.Phone,
		&order.Customer.Address, &order.Customer.Comment, &order.TotalKopecks,
		&order.IdempotencyKey, &order.Version, &order.CreatedAt, &order.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	return order, err
}

func (m OrderModel) getItemsForOrders(ctx context.Context, orderIDs []int64) (map[int64][]*OrderItem, error) {
	itemsByOrderID := make(map[int64][]*OrderItem, len(orderIDs))
	if len(orderIDs) == 0 {
		return itemsByOrderID, nil
	}
	rows, err := m.DB.QueryContext(ctx, `
		SELECT id, order_id, menu_item_id, partner_item_id, name_snapshot,
			quantity, price_kopecks_snapshot, created_at
		FROM order_items WHERE order_id = ANY($1)
		ORDER BY order_id, created_at, id`, pq.Array(orderIDs))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("close query rows: %v", err)
		}
	}()
	for rows.Next() {
		item := &OrderItem{}
		if err := rows.Scan(&item.ID, &item.OrderID, &item.MenuItemID, &item.PartnerItemID,
			&item.Name, &item.Quantity, &item.PriceKopecks, &item.CreatedAt); err != nil {
			return nil, err
		}
		itemsByOrderID[item.OrderID] = append(itemsByOrderID[item.OrderID], item)
	}
	return itemsByOrderID, rows.Err()
}

func (m OrderModel) getItems(ctx context.Context, orderID int64) ([]*OrderItem, error) {
	query := `
		SELECT id, order_id, menu_item_id, partner_item_id, name_snapshot,
			quantity, price_kopecks_snapshot, created_at
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at ASC, id ASC`

	rows, err := m.DB.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("close query rows: %v", err)
		}
	}()

	items := []*OrderItem{}
	for rows.Next() {
		item := &OrderItem{}
		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.MenuItemID,
			&item.PartnerItemID,
			&item.Name,
			&item.Quantity,
			&item.PriceKopecks,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

type orderTx interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func findOrderByIdempotencyKey(ctx context.Context, tx orderTx, cartID int64, idempotencyKey string) (int64, error) {
	query := `
		SELECT id
		FROM orders
		WHERE cart_id = $1 AND idempotency_key = $2`

	var orderID int64
	err := tx.QueryRowContext(ctx, query, cartID, idempotencyKey).Scan(&orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	return orderID, nil
}

func checkoutItemsFromCart(ctx context.Context, tx orderTx, cartID int64) ([]*OrderItem, int64, string, int, error) {
	query := `
		SELECT mi.restaurant_id, r.partner_id, ci.menu_item_id, mi.partner_item_id,
			ci.name_snapshot, ci.quantity, ci.price_kopecks_snapshot, mi.is_available
		FROM cart_items ci
		JOIN menu_items mi ON mi.id = ci.menu_item_id
		JOIN restaurants r ON r.id = mi.restaurant_id
		WHERE ci.cart_id = $1
		ORDER BY ci.created_at ASC, ci.id ASC`

	rows, err := tx.QueryContext(ctx, query, cartID)
	if err != nil {
		return nil, 0, "", 0, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("close query rows: %v", err)
		}
	}()

	var restaurantID int64
	var partnerID string
	total := 0
	items := []*OrderItem{}

	for rows.Next() {
		item := &OrderItem{}
		var itemRestaurantID int64
		var itemPartnerID string
		var isAvailable bool

		err := rows.Scan(
			&itemRestaurantID,
			&itemPartnerID,
			&item.MenuItemID,
			&item.PartnerItemID,
			&item.Name,
			&item.Quantity,
			&item.PriceKopecks,
			&isAvailable,
		)
		if err != nil {
			return nil, 0, "", 0, err
		}
		if !isAvailable {
			return nil, 0, "", 0, ErrItemUnavailable
		}

		if restaurantID == 0 {
			restaurantID = itemRestaurantID
			partnerID = itemPartnerID
		}
		if restaurantID != itemRestaurantID {
			return nil, 0, "", 0, ErrMixedCart
		}

		total += item.Quantity * item.PriceKopecks
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, "", 0, err
	}

	return items, restaurantID, partnerID, total, nil
}

func insertOrder(ctx context.Context, tx orderTx, input *CheckoutInput, restaurantID int64, partnerID string, total int) (*Order, error) {
	query := `
		INSERT INTO orders (
			cart_id,
			restaurant_id,
			partner_id,
			status,
			customer_name,
			customer_phone,
			delivery_address,
			customer_comment,
			total_kopecks,
			idempotency_key
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10)
		RETURNING id, cart_id, restaurant_id, partner_id, COALESCE(partner_order_id, ''),
			status, customer_name, customer_phone, delivery_address, COALESCE(customer_comment, ''),
			total_kopecks, idempotency_key, version, created_at, updated_at`

	order := &Order{}
	err := tx.QueryRowContext(ctx, query,
		input.CartID,
		restaurantID,
		partnerID,
		OrderStatusPendingPartner,
		input.Customer.Name,
		input.Customer.Phone,
		input.Customer.Address,
		input.Customer.Comment,
		total,
		input.IdempotencyKey,
	).Scan(
		&order.ID,
		&order.CartID,
		&order.RestaurantID,
		&order.PartnerID,
		&order.PartnerOrderID,
		&order.Status,
		&order.Customer.Name,
		&order.Customer.Phone,
		&order.Customer.Address,
		&order.Customer.Comment,
		&order.TotalKopecks,
		&order.IdempotencyKey,
		&order.Version,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return order, nil
}

func insertOrderItem(ctx context.Context, tx orderTx, orderID int64, item *OrderItem) error {
	query := `
		INSERT INTO order_items (
			order_id,
			menu_item_id,
			partner_item_id,
			name_snapshot,
			quantity,
			price_kopecks_snapshot
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, order_id, created_at`

	return tx.QueryRowContext(ctx, query,
		orderID,
		item.MenuItemID,
		item.PartnerItemID,
		item.Name,
		item.Quantity,
		item.PriceKopecks,
	).Scan(
		&item.ID,
		&item.OrderID,
		&item.CreatedAt,
	)
}
