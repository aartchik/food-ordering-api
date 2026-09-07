package models

import "database/sql"

type Models struct {
	Restaurants RestaurantModel
	MenuItems   MenuItemModel
	Carts       CartModel
	Orders      OrderModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Restaurants: RestaurantModel{DB: db},
		MenuItems:   MenuItemModel{DB: db},
		Carts:       CartModel{DB: db},
		Orders:      OrderModel{DB: db},
	}
}

type RestaurantModel struct {
	DB *sql.DB
}

type MenuItemModel struct {
	DB *sql.DB
}

type CartModel struct {
	DB *sql.DB
}

type OrderModel struct {
	DB *sql.DB
}
