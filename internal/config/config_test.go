package config

import (
	"testing"
	"time"
)

func TestLoadServerPriority(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("ADDRESS", "")
		cfg, err := LoadServer(nil)
		if err != nil || cfg.Address != "localhost:8080" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("flag over default", func(t *testing.T) {
		t.Setenv("ADDRESS", "")
		cfg, err := LoadServer([]string{"-a", "localhost:9090"})
		if err != nil || cfg.Address != "localhost:9090" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("env over flag", func(t *testing.T) {
		t.Setenv("ADDRESS", "localhost:7070")
		cfg, err := LoadServer([]string{"-a", "localhost:9090"})
		if err != nil || cfg.Address != "localhost:7070" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
}

func TestLoadAgentPriority(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("ADDRESS", "")
		t.Setenv("POLL_INTERVAL", "")
		t.Setenv("REPORT_INTERVAL", "")
		cfg, err := LoadAgent(nil)
		want := AgentConfig{"localhost:8080", 2 * time.Second, 10 * time.Second}
		if err != nil || cfg != want {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("flags", func(t *testing.T) {
		t.Setenv("ADDRESS", "")
		t.Setenv("POLL_INTERVAL", "")
		t.Setenv("REPORT_INTERVAL", "")
		cfg, err := LoadAgent([]string{"-a", "h:1", "-p", "3", "-r", "5"})
		want := AgentConfig{"h:1", 3 * time.Second, 5 * time.Second}
		if err != nil || cfg != want {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("env over flags", func(t *testing.T) {
		t.Setenv("ADDRESS", "h:2")
		t.Setenv("POLL_INTERVAL", "7")
		t.Setenv("REPORT_INTERVAL", "20")
		cfg, err := LoadAgent([]string{"-a", "h:1", "-p", "3", "-r", "5"})
		want := AgentConfig{"h:2", 7 * time.Second, 20 * time.Second}
		if err != nil || cfg != want {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
}

func TestLoadAgentInvalid(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{"env not a number", nil, map[string]string{"POLL_INTERVAL": "abc"}},
		{"env zero", nil, map[string]string{"REPORT_INTERVAL": "0"}},
		{"env negative", nil, map[string]string{"POLL_INTERVAL": "-5"}},
		{"flag zero poll", []string{"-p", "0"}, nil},
		{"flag negative report", []string{"-r", "-1"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ADDRESS", "")
			t.Setenv("POLL_INTERVAL", "")
			t.Setenv("REPORT_INTERVAL", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if _, err := LoadAgent(tt.args); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
