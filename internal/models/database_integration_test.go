//go:build integration

package models

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lib/pq"
)

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL must point to a PostgreSQL test database")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("test_%x", rand.Text())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+pq.QuoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+pq.QuoteIdentifier(schema)+" CASCADE"); err != nil {
			t.Errorf("cleanup schema: %v", err)
		}
	})
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(10)
	t.Cleanup(func() { _ = db.Close() })
	applyTestMigrations(t, db, "up")
	return db
}

func applyTestMigrations(t *testing.T, db *sql.DB, direction string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*."+direction+".sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("find migrations: %v", err)
	}
	if direction == "down" {
		for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
			paths[i], paths[j] = paths[j], paths[i]
		}
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = db.ExecContext(ctx, string(data))
		cancel()
		if err != nil {
			t.Fatalf("migration %s: %v", path, err)
		}
	}
}

func testCatalog(t *testing.T, m Models, partner string) (*Restaurant, *MenuItem) {
	t.Helper()
	r := &Restaurant{Name: "Bakery", Address: "Main street", IsOpen: true}
	if err := m.Restaurants.UpsertByPartnerID(partner, r); err != nil {
		t.Fatal(err)
	}
	if err := m.MenuItems.UpsertForRestaurant(r.ID, []*MenuItemInput{{PartnerItemID: "bread", Name: "Bread", PriceKopecks: 15000, IsAvailable: true}}); err != nil {
		t.Fatal(err)
	}
	items, err := m.MenuItems.GetAllForRestaurant(r.ID, true)
	if err != nil || len(items) != 1 {
		t.Fatalf("menu: %v, %v", items, err)
	}
	return r, items[0]
}

func testCheckout(t *testing.T, m Models, itemID int64) *CheckoutInput {
	t.Helper()
	cart, err := m.Carts.Insert("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Carts.AddItem(cart.Cart.ID, itemID, 2); err != nil {
		t.Fatal(err)
	}
	return &CheckoutInput{CartID: cart.Cart.ID, Customer: Customer{Name: "Anna", Phone: "+79990000000", Address: "Main street"}, IdempotencyKey: "checkout-1"}
}

func TestMigrationsRoundTrip(t *testing.T) {
	db := testDatabase(t)
	applyTestMigrations(t, db, "down")
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema()`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("tables after down: %d, %v", count, err)
	}
	applyTestMigrations(t, db, "up")
	testCatalog(t, NewModels(db), "partner")
}
