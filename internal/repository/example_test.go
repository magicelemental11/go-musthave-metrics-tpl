package repository_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func ExampleMemStorage() {
	ctx := context.Background()
	s := repository.NewMemStorage()

	s.UpdateGauge(ctx, "Alloc", 1.5)
	s.UpdateCounter(ctx, "PollCount", 2)
	total, _ := s.UpdateCounter(ctx, "PollCount", 3)
	fmt.Println(total)

	gauge, _ := s.GetGauge(ctx, "Alloc")
	fmt.Println(gauge)

	_, err := s.GetGauge(ctx, "Missing")
	fmt.Println(errors.Is(err, repository.ErrNotFound))
	// Output:
	// 5
	// 1.5
	// true
}
