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
	for _, name := range []string{"ADDRESS", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE", "DATABASE_DSN", "KEY", "AUDIT_FILE", "AUDIT_URL", "PPROF_ADDRESS"} {
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

func TestLoadServerPprof(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		clearServerEnv(t)
		cfg, err := LoadServer(nil)
		if err != nil || cfg.PprofAddress != "" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("flag", func(t *testing.T) {
		clearServerEnv(t)
		cfg, err := LoadServer([]string{"--pprof-address", "localhost:6060"})
		if err != nil || cfg.PprofAddress != "localhost:6060" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("env over flag", func(t *testing.T) {
		clearServerEnv(t)
		t.Setenv("PPROF_ADDRESS", "localhost:7070")
		cfg, err := LoadServer([]string{"--pprof-address", "localhost:6060"})
		if err != nil || cfg.PprofAddress != "localhost:7070" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
}

func TestCryptoKey(t *testing.T) {
	loaders := map[string]func(args []string) (string, error){
		"server": func(args []string) (string, error) {
			cfg, err := LoadServer(args)
			return cfg.CryptoKey, err
		},
		"agent": func(args []string) (string, error) {
			cfg, err := LoadAgent(args)
			return cfg.CryptoKey, err
		},
	}
	for name, load := range loaders {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CRYPTO_KEY", "")
			if got, err := load(nil); err != nil || got != "" {
				t.Fatalf("default: got %q, %v", got, err)
			}
			if got, err := load([]string{"-crypto-key", "flag.pem"}); err != nil || got != "flag.pem" {
				t.Fatalf("flag: got %q, %v", got, err)
			}
			t.Setenv("CRYPTO_KEY", "env.pem")
			if got, err := load([]string{"-crypto-key", "flag.pem"}); err != nil || got != "env.pem" {
				t.Fatalf("env over flag: got %q, %v", got, err)
			}
		})
	}
}

func TestLoadServerTrustedSubnet(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		args    []string
		file    string
		want    string
		wantErr bool
	}{
		{name: "default is empty", want: ""},
		{name: "flag", args: []string{"-t", "192.168.1.0/24"}, want: "192.168.1.0/24"},
		{name: "env", env: "10.0.0.0/8", want: "10.0.0.0/8"},
		{name: "env over flag", env: "10.0.0.0/8", args: []string{"-t", "192.168.1.0/24"}, want: "10.0.0.0/8"},
		{name: "file", file: `{"trusted_subnet": "172.16.0.0/12"}`, want: "172.16.0.0/12"},
		{name: "flag over file", file: `{"trusted_subnet": "172.16.0.0/12"}`, args: []string{"-t", "192.168.1.0/24"}, want: "192.168.1.0/24"},
		{name: "invalid flag", args: []string{"-t", "10.0.0.1"}, wantErr: true},
		{name: "invalid env", env: "garbage", wantErr: true},
		{name: "invalid file", file: `{"trusted_subnet": "10.0.0.0/33"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t, serverEnv...)
			if tt.env != "" {
				t.Setenv("TRUSTED_SUBNET", tt.env)
			}
			args := tt.args
			if tt.file != "" {
				args = append([]string{"-c", writeConfig(t, tt.file)}, args...)
			}
			cfg, err := LoadServer(args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", cfg)
				}
				return
			}
			if err != nil || cfg.TrustedSubnet != tt.want {
				t.Fatalf("got %q, %v; want %q", cfg.TrustedSubnet, err, tt.want)
			}
		})
	}
}
