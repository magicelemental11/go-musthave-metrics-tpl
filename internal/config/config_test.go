package config

import (
	"os"
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

func clearServerEnv(t *testing.T) {
	for _, name := range []string{"ADDRESS", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE", "DATABASE_DSN", "KEY", "AUDIT_FILE", "AUDIT_URL"} {
		t.Setenv(name, "") // запоминает исходное значение для восстановления
		os.Unsetenv(name)
	}
}

func TestLoadServerStorageDefaults(t *testing.T) {
	clearServerEnv(t)
	cfg, err := LoadServer(nil)
	want := ServerConfig{Address: "localhost:8080", StoreInterval: 300 * time.Second, FileStoragePath: "metrics-db.json", Restore: true}
	if err != nil || cfg != want {
		t.Fatalf("got %+v, %v", cfg, err)
	}
}

func TestLoadServerStorageFlags(t *testing.T) {
	clearServerEnv(t)
	cfg, err := LoadServer([]string{"-i", "0", "-f", "/tmp/m.json", "-r=false"})
	want := ServerConfig{Address: "localhost:8080", StoreInterval: 0, FileStoragePath: "/tmp/m.json", Restore: false}
	if err != nil || cfg != want {
		t.Fatalf("got %+v, %v", cfg, err)
	}
}

func TestLoadServerStorageEnvOverFlags(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("STORE_INTERVAL", "0")
	t.Setenv("FILE_STORAGE_PATH", "/env/m.json")
	t.Setenv("RESTORE", "false")
	cfg, err := LoadServer([]string{"-i", "10", "-f", "/flag/m.json", "-r=true"})
	want := ServerConfig{Address: "localhost:8080", StoreInterval: 0, FileStoragePath: "/env/m.json", Restore: false}
	if err != nil || cfg != want {
		t.Fatalf("got %+v, %v", cfg, err)
	}
}

func TestLoadServerStorageInvalidEnv(t *testing.T) {
	clearServerEnv(t)
	t.Setenv("STORE_INTERVAL", "-1")
	if _, err := LoadServer(nil); err == nil {
		t.Error("expected error for negative STORE_INTERVAL")
	}
	t.Setenv("STORE_INTERVAL", "")
	t.Setenv("RESTORE", "maybe")
	if _, err := LoadServer(nil); err == nil {
		t.Error("expected error for invalid RESTORE")
	}
}

func TestLoadAgentPriority(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("ADDRESS", "")
		t.Setenv("POLL_INTERVAL", "")
		t.Setenv("REPORT_INTERVAL", "")
		t.Setenv("KEY", "")
		cfg, err := LoadAgent(nil)
		want := AgentConfig{Address: "localhost:8080", PollInterval: 2 * time.Second, ReportInterval: 10 * time.Second, RateLimit: 1}
		if err != nil || cfg != want {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("flags", func(t *testing.T) {
		t.Setenv("ADDRESS", "")
		t.Setenv("POLL_INTERVAL", "")
		t.Setenv("REPORT_INTERVAL", "")
		t.Setenv("KEY", "")
		cfg, err := LoadAgent([]string{"-a", "h:1", "-p", "3", "-r", "5"})
		want := AgentConfig{Address: "h:1", PollInterval: 3 * time.Second, ReportInterval: 5 * time.Second, RateLimit: 1}
		if err != nil || cfg != want {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("env over flags", func(t *testing.T) {
		t.Setenv("ADDRESS", "h:2")
		t.Setenv("POLL_INTERVAL", "7")
		t.Setenv("REPORT_INTERVAL", "20")
		cfg, err := LoadAgent([]string{"-a", "h:1", "-p", "3", "-r", "5"})
		want := AgentConfig{Address: "h:2", PollInterval: 7 * time.Second, ReportInterval: 20 * time.Second, RateLimit: 1}
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
			t.Setenv("KEY", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if _, err := LoadAgent(tt.args); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadServerDatabaseDSN(t *testing.T) {
	clearServerEnv(t)
	cfg, err := LoadServer(nil)
	if err != nil || cfg.DatabaseDSN != "" {
		t.Fatalf("default: got %+v, %v", cfg, err)
	}
	cfg, err = LoadServer([]string{"-d", "flag-dsn"})
	if err != nil || cfg.DatabaseDSN != "flag-dsn" {
		t.Fatalf("flag: got %+v, %v", cfg, err)
	}
	t.Setenv("DATABASE_DSN", "env-dsn")
	cfg, err = LoadServer([]string{"-d", "flag-dsn"})
	if err != nil || cfg.DatabaseDSN != "env-dsn" {
		t.Fatalf("env: got %+v, %v", cfg, err)
	}
}

func TestLoadKey(t *testing.T) {
	clearServerEnv(t)
	t.Run("flag", func(t *testing.T) {
		s, err := LoadServer([]string{"-k", "flagkey"})
		if err != nil || s.Key != "flagkey" {
			t.Fatalf("server: %+v, %v", s, err)
		}
		a, err := LoadAgent([]string{"-k", "flagkey"})
		if err != nil || a.Key != "flagkey" {
			t.Fatalf("agent: %+v, %v", a, err)
		}
	})
	t.Run("env over flag", func(t *testing.T) {
		t.Setenv("KEY", "envkey")
		s, err := LoadServer([]string{"-k", "flagkey"})
		if err != nil || s.Key != "envkey" {
			t.Fatalf("server: %+v, %v", s, err)
		}
		a, err := LoadAgent([]string{"-k", "flagkey"})
		if err != nil || a.Key != "envkey" {
			t.Fatalf("agent: %+v, %v", a, err)
		}
	})
}

func TestLoadAgentRateLimit(t *testing.T) {
	t.Run("flag", func(t *testing.T) {
		t.Setenv("RATE_LIMIT", "")
		cfg, err := LoadAgent([]string{"-l", "4"})
		if err != nil || cfg.RateLimit != 4 {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("env over flag", func(t *testing.T) {
		t.Setenv("RATE_LIMIT", "7")
		cfg, err := LoadAgent([]string{"-l", "4"})
		if err != nil || cfg.RateLimit != 7 {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		t.Setenv("RATE_LIMIT", "")
		if _, err := LoadAgent([]string{"-l", "0"}); err == nil {
			t.Fatal("expected error for 0")
		}
		t.Setenv("RATE_LIMIT", "abc")
		if _, err := LoadAgent(nil); err == nil {
			t.Fatal("expected error for abc")
		}
	})
}

func TestLoadServerAudit(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		clearServerEnv(t)
		cfg, err := LoadServer(nil)
		if err != nil || cfg.AuditFile != "" || cfg.AuditURL != "" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("flags", func(t *testing.T) {
		clearServerEnv(t)
		cfg, err := LoadServer([]string{"--audit-file", "/tmp/a.log", "--audit-url", "http://a/b"})
		if err != nil || cfg.AuditFile != "/tmp/a.log" || cfg.AuditURL != "http://a/b" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("env over flags", func(t *testing.T) {
		clearServerEnv(t)
		t.Setenv("AUDIT_FILE", "/env/a.log")
		t.Setenv("AUDIT_URL", "http://env/")
		cfg, err := LoadServer([]string{"--audit-file", "/tmp/a.log", "--audit-url", "http://a/b"})
		if err != nil || cfg.AuditFile != "/env/a.log" || cfg.AuditURL != "http://env/" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
}
