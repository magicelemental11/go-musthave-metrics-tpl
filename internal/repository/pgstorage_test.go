package repository

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func newTestPg(t *testing.T) *PgStorage {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`TRUNCATE metrics`); err != nil {
		t.Fatal(err)
	}
	return NewPgStorage(db)
}

func TestPgStorageGauge(t *testing.T) {
	s := newTestPg(t)
	if _, err := s.GetGauge(ctx, "g"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	for _, v := range []float64{1.5, 2.25} {
		if err := s.UpdateGauge(ctx, "g", v); err != nil {
			t.Fatal(err)
		}
	}
	if v, err := s.GetGauge(ctx, "g"); err != nil || v != 2.25 {
		t.Fatalf("got %v, %v", v, err)
	}
}

func TestPgStorageCounterAccumulates(t *testing.T) {
	s := newTestPg(t)
	if total, err := s.UpdateCounter(ctx, "c", 3); err != nil || total != 3 {
		t.Fatalf("got %v, %v", total, err)
	}
	if total, err := s.UpdateCounter(ctx, "c", 4); err != nil || total != 7 {
		t.Fatalf("got %v, %v", total, err)
	}
	if v, err := s.GetCounter(ctx, "c"); err != nil || v != 7 {
		t.Fatalf("got %v, %v", v, err)
	}
}

func TestPgStorageGetAll(t *testing.T) {
	s := newTestPg(t)
	s.UpdateGauge(ctx, "g", 1.5)
	s.UpdateCounter(ctx, "c", 2)
	gauges, counters, err := s.GetAllMetrics(ctx)
	if err != nil || gauges["g"] != 1.5 || counters["c"] != 2 || len(gauges) != 1 || len(counters) != 1 {
		t.Fatalf("got %v %v %v", gauges, counters, err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := newTestPg(t)
	if err := Migrate(s.db); err != nil {
		t.Fatal(err)
	}
}
