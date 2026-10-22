package repository

import (
	"database/sql"
	"errors"
	"math"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
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

func TestPgStorageUpdateBatch(t *testing.T) {
	s := newTestPg(t)
	g1, g2 := 1.5, 2.5
	c1, c2 := int64(3), int64(4)
	batch := []model.Metrics{
		{ID: "g", MType: model.Gauge, Value: &g1},
		{ID: "g", MType: model.Gauge, Value: &g2},
		{ID: "c", MType: model.Counter, Delta: &c1},
		{ID: "c", MType: model.Counter, Delta: &c2},
	}
	if err := s.UpdateBatch(ctx, batch); err != nil {
		t.Fatal(err)
	}
	if v, err := s.GetGauge(ctx, "g"); err != nil || v != 2.5 {
		t.Errorf("gauge = %v, %v", v, err)
	}
	if v, err := s.GetCounter(ctx, "c"); err != nil || v != 7 {
		t.Errorf("counter = %v, %v", v, err)
	}
}

func TestPgStorageUpdateBatchRollsBack(t *testing.T) {
	s := newTestPg(t)
	g := 1.5
	max, one := int64(math.MaxInt64), int64(1)
	batch := []model.Metrics{
		{ID: "g", MType: model.Gauge, Value: &g},
		{ID: "c", MType: model.Counter, Delta: &max},
		{ID: "c", MType: model.Counter, Delta: &one},
	}
	if err := s.UpdateBatch(ctx, batch); err == nil {
		t.Fatal("expected counter overflow error")
	}
	if _, err := s.GetGauge(ctx, "g"); !errors.Is(err, ErrNotFound) {
		t.Errorf("failed batch must be rolled back, got %v", err)
	}
	if _, err := s.GetCounter(ctx, "c"); !errors.Is(err, ErrNotFound) {
		t.Errorf("failed batch must be rolled back, got %v", err)
	}
}

func TestPgStorageRetriesWhenDatabaseIsDown(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://user:pw@127.0.0.1:1/metrics?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	delay := 20 * time.Millisecond
	s := NewPgStorage(db)
	s.delays = []time.Duration{delay, delay, delay}

	start := time.Now()
	err = s.UpdateGauge(ctx, "g", 1)
	if err == nil {
		t.Fatal("expected error when database is down")
	}
	if elapsed := time.Since(start); elapsed < 3*delay {
		t.Errorf("expected 3 retry pauses (>= %v), took %v", 3*delay, elapsed)
	}
}
