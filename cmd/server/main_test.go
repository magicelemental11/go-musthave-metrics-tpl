package main

import (
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
)

func TestBackendFor(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.ServerConfig
		want string
	}{
		{"dsn wins over file", config.ServerConfig{DatabaseDSN: "dsn", FileStoragePath: "f.json"}, backendPostgres},
		{"dsn only", config.ServerConfig{DatabaseDSN: "dsn"}, backendPostgres},
		{"file without dsn", config.ServerConfig{FileStoragePath: "f.json"}, backendFile},
		{"nothing configured", config.ServerConfig{}, backendMemory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backendFor(tt.cfg); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
