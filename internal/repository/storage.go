// Package repository contains metric storages: in-memory, file-backed and PostgreSQL.
package repository

import (
	"context"
	"errors"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

// ErrNotFound is returned by getters when the metric does not exist.
var ErrNotFound = errors.New("metric not found")

// Storage is the metric store used by the handlers.
// UpdateCounter adds to the stored value and returns the new total;
// UpdateGauge replaces the value.
type Storage interface {
	GetGauge(ctx context.Context, name string) (float64, error)
	GetCounter(ctx context.Context, name string) (int64, error)
	UpdateGauge(ctx context.Context, name string, value float64) error
	UpdateCounter(ctx context.Context, name string, value int64) (int64, error)
	GetAllMetrics(ctx context.Context) (map[string]float64, map[string]int64, error)
	UpdateBatch(ctx context.Context, metrics []model.Metrics) error
}
