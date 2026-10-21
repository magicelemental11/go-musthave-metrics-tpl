package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/retry"
)

const (
	upsertGaugeSQL = `
		INSERT INTO metrics (id, type, value) VALUES ($1, $2, $3)
		ON CONFLICT (id, type) DO UPDATE SET value = EXCLUDED.value`
	upsertCounterSQL = `
		INSERT INTO metrics (id, type, delta) VALUES ($1, $2, $3)
		ON CONFLICT (id, type) DO UPDATE SET delta = metrics.delta + EXCLUDED.delta`
)

// PgStorage is a Storage backed by PostgreSQL. Queries are retried on connection errors.
type PgStorage struct {
	db     *sql.DB
	delays []time.Duration
}

// NewPgStorage creates a PgStorage on top of db. The schema is created by Migrate.
func NewPgStorage(db *sql.DB) *PgStorage {
	return &PgStorage{db: db, delays: retry.DefaultDelays}
}

func (p *PgStorage) do(ctx context.Context, fn func() error) error {
	return retry.Do(ctx, p.delays, fn, isPgConnErr)
}

// GetGauge returns the gauge value or ErrNotFound.
func (p *PgStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	var value float64
	err := p.do(ctx, func() error {
		return p.db.QueryRowContext(ctx,
			`SELECT value FROM metrics WHERE id = $1 AND type = $2`, name, model.Gauge).Scan(&value)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get gauge %q: %w", name, err)
	}
	return value, nil
}

// GetCounter returns the counter value or ErrNotFound.
func (p *PgStorage) GetCounter(ctx context.Context, name string) (int64, error) {
	var delta int64
	err := p.do(ctx, func() error {
		return p.db.QueryRowContext(ctx,
			`SELECT delta FROM metrics WHERE id = $1 AND type = $2`, name, model.Counter).Scan(&delta)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get counter %q: %w", name, err)
	}
	return delta, nil
}

// UpdateGauge sets the gauge to value.
func (p *PgStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	err := p.do(ctx, func() error {
		_, err := p.db.ExecContext(ctx, upsertGaugeSQL, name, model.Gauge, value)
		return err
	})
	if err != nil {
		return fmt.Errorf("update gauge %q: %w", name, err)
	}
	return nil
}

// UpdateCounter adds value to the counter and returns the new total.
func (p *PgStorage) UpdateCounter(ctx context.Context, name string, value int64) (int64, error) {
	var total int64
	err := p.do(ctx, func() error {
		return p.db.QueryRowContext(ctx, upsertCounterSQL+` RETURNING delta`, name, model.Counter, value).Scan(&total)
	})
	if err != nil {
		return 0, fmt.Errorf("update counter %q: %w", name, err)
	}
	return total, nil
}

// GetAllMetrics returns all gauges and counters.
func (p *PgStorage) GetAllMetrics(ctx context.Context) (map[string]float64, map[string]int64, error) {
	var gauges map[string]float64
	var counters map[string]int64
	err := p.do(ctx, func() (err error) {
		gauges, counters, err = p.queryAll(ctx)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return gauges, counters, nil
}

func (p *PgStorage) queryAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
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

// UpdateBatch validates all metrics and then applies them in a single transaction.
func (p *PgStorage) UpdateBatch(ctx context.Context, metrics []model.Metrics) error {
	for _, m := range metrics {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("invalid metric %q: %w", m.ID, err)
		}
	}
	return p.do(ctx, func() error { return p.applyBatch(ctx, metrics) })
}

func (p *PgStorage) applyBatch(ctx context.Context, metrics []model.Metrics) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin batch: %w", err)
	}
	defer tx.Rollback()

	for _, m := range metrics {
		switch m.MType {
		case model.Gauge:
			_, err = tx.ExecContext(ctx, upsertGaugeSQL, m.ID, m.MType, *m.Value)
		case model.Counter:
			_, err = tx.ExecContext(ctx, upsertCounterSQL, m.ID, m.MType, *m.Delta)
		}
		if err != nil {
			return fmt.Errorf("update %s %q: %w", m.MType, m.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit batch: %w", err)
	}
	return nil
}
