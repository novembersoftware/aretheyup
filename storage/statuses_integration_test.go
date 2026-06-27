package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/novembersoftware/aretheyup/services"
	"github.com/novembersoftware/aretheyup/structs"
	"gorm.io/gorm"
)

func newStatusIntegrationStore(t *testing.T) (*Storage, *gorm.DB) {
	t.Helper()

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN is not set; skipping Postgres-backed storage test")
	}

	db, err := services.NewDB(dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	schema := fmt.Sprintf("org10_test_%d", time.Now().UnixNano())
	quotedSchema := pq.QuoteIdentifier(schema)
	if err := db.Exec("CREATE SCHEMA " + quotedSchema).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if err := db.Exec("SET search_path TO " + quotedSchema).Error; err != nil {
		t.Fatalf("set test search_path: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Exec("DROP SCHEMA IF EXISTS " + quotedSchema + " CASCADE").Error
		_ = sqlDB.Close()
	})

	if err := services.MigrateDB(db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return New(db, nil), db
}

func createStatusTestService(t *testing.T, db *gorm.DB) structs.Service {
	t.Helper()

	now := time.Now().UTC()
	service := structs.Service{
		Slug:        fmt.Sprintf("service-%d", now.UnixNano()),
		Name:        "Status Test Service",
		Category:    "test",
		HomepageURL: "https://example.com",
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	return service
}
