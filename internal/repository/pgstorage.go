package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

type PgStorage struct {
	db *sql.DB
}

func NewPgStorage(db *sql.DB) *PgStorage {
	return &PgStorage{db: db}
}

func (p *PgStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	var value float64
	err := p.db.QueryRowContext(ctx,
		`SELECT value FROM metrics WHERE id = $1 AND type = $2`, name, model.Gauge).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get gauge %q: %w", name, err)
	}
	return value, nil
}

func (p *PgStorage) GetCounter(ctx context.Context, name string) (int64, error) {
	var delta int64
	err := p.db.QueryRowContext(ctx,
		`SELECT delta FROM metrics WHERE id = $1 AND type = $2`, name, model.Counter).Scan(&delta)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get counter %q: %w", name, err)
	}
	return delta, nil
}

func (p *PgStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO metrics (id, type, value) VALUES ($1, $2, $3)
		ON CONFLICT (id, type) DO UPDATE SET value = EXCLUDED.value`,
		name, model.Gauge, value)
	if err != nil {
		return fmt.Errorf("update gauge %q: %w", name, err)
	}
	return nil
}

func (p *PgStorage) UpdateCounter(ctx context.Context, name string, value int64) (int64, error) {
	var total int64
	err := p.db.QueryRowContext(ctx, `
		INSERT INTO metrics (id, type, delta) VALUES ($1, $2, $3)
		ON CONFLICT (id, type) DO UPDATE SET delta = metrics.delta + EXCLUDED.delta
		RETURNING delta`,
		name, model.Counter, value).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("update counter %q: %w", name, err)
	}
	return total, nil
}

func (p *PgStorage) GetAllMetrics(ctx context.Context) (map[string]float64, map[string]int64, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT id, type, delta, value FROM metrics`)
	if err != nil {
		return nil, nil, fmt.Errorf("get all metrics: %w", err)
	}
	defer rows.Close()

	gauges := make(map[string]float64)
	counters := make(map[string]int64)
	for rows.Next() {
		var (
			id, mtype string
			delta     sql.NullInt64
			value     sql.NullFloat64
		)
		if err := rows.Scan(&id, &mtype, &delta, &value); err != nil {
			return nil, nil, fmt.Errorf("scan metric: %w", err)
		}
		switch mtype {
		case model.Gauge:
			gauges[id] = value.Float64
		case model.Counter:
			counters[id] = delta.Int64
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("get all metrics: %w", err)
	}
	return gauges, counters, nil
}
