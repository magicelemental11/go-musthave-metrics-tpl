package repository

import (
	"context"
	"errors"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

var ErrNotFound = errors.New("metric not found")

type Storage interface {
	GetGauge(ctx context.Context, name string) (float64, error)
	GetCounter(ctx context.Context, name string) (int64, error)
	UpdateGauge(ctx context.Context, name string, value float64) error
	UpdateCounter(ctx context.Context, name string, value int64) (int64, error)
	GetAllMetrics(ctx context.Context) (map[string]float64, map[string]int64, error)
	UpdateBatch(ctx context.Context, metrics []model.Metrics) error
}
