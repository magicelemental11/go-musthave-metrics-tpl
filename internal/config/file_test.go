package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func clearEnv(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
}

var serverEnv = []string{"CONFIG", "ADDRESS", "STORE_INTERVAL", "RESTORE", "FILE_STORAGE_PATH", "DATABASE_DSN", "CRYPTO_KEY", "KEY", "AUDIT_FILE", "AUDIT_URL", "PPROF_ADDRESS", "TRUSTED_SUBNET", "GRPC_ADDRESS"}
var agentEnv = []string{"CONFIG", "ADDRESS", "POLL_INTERVAL", "REPORT_INTERVAL", "CRYPTO_KEY", "KEY", "RATE_LIMIT", "GRPC_ADDRESS"}

func TestServerConfigFile(t *testing.T) {
	file := `{
		"address": "file:1",
		"restore": false,
		"store_interval": "1s",
		"store_file": "/file.db",
		"database_dsn": "file-dsn",
		"crypto_key": "file.pem",
		"key": "file-key",
		"audit_file": "file-audit",
		"audit_url": "http://audit",
		"pprof_address": "file:6060",
		"trusted_subnet": "10.0.0.0/8"
	}`

	t.Run("file only", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		cfg, err := LoadServer([]string{"-c", writeConfig(t, file)})
		if err != nil {
			t.Fatal(err)
		}
		want := ServerConfig{
			Address: "file:1", StoreInterval: time.Second, FileStoragePath: "/file.db", Restore: false,
			DatabaseDSN: "file-dsn", Key: "file-key", CryptoKey: "file.pem",
			AuditFile: "file-audit", AuditURL: "http://audit", PprofAddress: "file:6060",
			TrustedSubnet: "10.0.0.0/8",
		}
		if cfg != want {
			t.Fatalf("got %+v, want %+v", cfg, want)
		}
	})

	t.Run("long flag", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		cfg, err := LoadServer([]string{"-config", writeConfig(t, file)})
		if err != nil || cfg.Address != "file:1" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("env CONFIG", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		t.Setenv("CONFIG", writeConfig(t, file))
		cfg, err := LoadServer(nil)
		if err != nil || cfg.Address != "file:1" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("flag over file even when equal to default", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		cfg, err := LoadServer([]string{"-c", writeConfig(t, file), "-a", "localhost:8080", "-r=true"})
		if err != nil || cfg.Address != "localhost:8080" || !cfg.Restore {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("env over flag over file", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		t.Setenv("ADDRESS", "env:3")
		cfg, err := LoadServer([]string{"-c", writeConfig(t, file), "-a", "flag:2"})
		if err != nil || cfg.Address != "env:3" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("false in file beats default true", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		cfg, err := LoadServer([]string{"-c", writeConfig(t, `{"restore": false}`)})
		if err != nil || cfg.Restore {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("missing options keep defaults", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		cfg, err := LoadServer([]string{"-c", writeConfig(t, `{}`)})
		if err != nil || cfg.Address != defaultAddress || cfg.StoreInterval != defaultStoreInterval*time.Second {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("sub-second store interval is kept", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		cfg, err := LoadServer([]string{"-c", writeConfig(t, `{"store_interval": "500ms"}`)})
		if err != nil || cfg.StoreInterval != 500*time.Millisecond {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("zero store interval means synchronous", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		cfg, err := LoadServer([]string{"-c", writeConfig(t, `{"store_interval": "0s"}`)})
		if err != nil || cfg.StoreInterval != 0 {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("empty store_file disables saving", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		cfg, err := LoadServer([]string{"-c", writeConfig(t, `{"store_file": ""}`)})
		if err != nil || cfg.FileStoragePath != "" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
}

func TestAgentConfigFile(t *testing.T) {
	file := `{"address": "file:1", "report_interval": "5s", "poll_interval": "3s", "crypto_key": "file.pem", "key": "file-key", "rate_limit": 4}`

	t.Run("file only", func(t *testing.T) {
		clearEnv(t, agentEnv...)
		cfg, err := LoadAgent([]string{"-c", writeConfig(t, file)})
		if err != nil {
			t.Fatal(err)
		}
		want := AgentConfig{
			Address: "file:1", ReportInterval: 5 * time.Second, PollInterval: 3 * time.Second,
			CryptoKey: "file.pem", Key: "file-key", RateLimit: 4,
		}
		if cfg != want {
			t.Fatalf("got %+v, want %+v", cfg, want)
		}
	})

	t.Run("flag over file even when equal to default", func(t *testing.T) {
		clearEnv(t, agentEnv...)
		cfg, err := LoadAgent([]string{"-config", writeConfig(t, file), "-p", "2"})
		if err != nil || cfg.PollInterval != 2*time.Second || cfg.ReportInterval != 5*time.Second {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("env over file", func(t *testing.T) {
		clearEnv(t, agentEnv...)
		t.Setenv("REPORT_INTERVAL", "7")
		t.Setenv("CONFIG", writeConfig(t, file))
		cfg, err := LoadAgent(nil)
		if err != nil || cfg.ReportInterval != 7*time.Second {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
}

func TestConfigFileErrors(t *testing.T) {
	tests := map[string]string{
		"invalid json":     `{"address": `,
		"unknown field":    `{"adress": "x"}`,
		"bad duration":     `{"store_interval": "abc"}`,
		"number duration":  `{"store_interval": 5}`,
		"negative":         `{"store_interval": "-1s"}`,
		"wrong type":       `{"restore": "yes"}`,
		"agent key on srv": `{"poll_interval": "1s"}`,
	}
	for name, content := range tests {
		t.Run("server "+name, func(t *testing.T) {
			clearEnv(t, serverEnv...)
			if _, err := LoadServer([]string{"-c", writeConfig(t, content)}); err == nil {
				t.Fatal("expected error")
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		path := filepath.Join(t.TempDir(), "nope.json")
		if _, err := LoadServer([]string{"-c", path}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("agent poll_interval below minimum", func(t *testing.T) {
		clearEnv(t, agentEnv...)
		if _, err := LoadAgent([]string{"-c", writeConfig(t, `{"poll_interval": "500ms"}`)}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("agent rate_limit below minimum", func(t *testing.T) {
		clearEnv(t, agentEnv...)
		if _, err := LoadAgent([]string{"-c", writeConfig(t, `{"rate_limit": 0}`)}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestServerPriorityOverFile(t *testing.T) {
	file := `{"restore": false, "store_interval": "9s"}`

	t.Run("env over file for restore and interval", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		t.Setenv("RESTORE", "true")
		t.Setenv("STORE_INTERVAL", "4")
		cfg, err := LoadServer([]string{"-c", writeConfig(t, file)})
		if err != nil || !cfg.Restore || cfg.StoreInterval != 4*time.Second {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("flag equal to default over file for interval", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		cfg, err := LoadServer([]string{"-c", writeConfig(t, file), "-i", "300"})
		if err != nil || cfg.StoreInterval != 300*time.Second {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("env CONFIG over -c", func(t *testing.T) {
		clearEnv(t, serverEnv...)
		t.Setenv("CONFIG", writeConfig(t, `{"address": "env-file:1"}`))
		cfg, err := LoadServer([]string{"-c", writeConfig(t, `{"address": "flag-file:2"}`)})
		if err != nil || cfg.Address != "env-file:1" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
}

func TestAgentPriorityOverFile(t *testing.T) {
	file := `{"address": "file:1", "report_interval": "5s", "poll_interval": "3s", "rate_limit": 4}`

	t.Run("env over flag over file", func(t *testing.T) {
		clearEnv(t, agentEnv...)
		t.Setenv("ADDRESS", "env:3")
		t.Setenv("POLL_INTERVAL", "6")
		t.Setenv("RATE_LIMIT", "8")
		cfg, err := LoadAgent([]string{"-c", writeConfig(t, file), "-a", "flag:2", "-p", "5", "-l", "7"})
		if err != nil || cfg.Address != "env:3" || cfg.PollInterval != 6*time.Second || cfg.RateLimit != 8 {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("flags equal to defaults over file", func(t *testing.T) {
		clearEnv(t, agentEnv...)
		cfg, err := LoadAgent([]string{"-c", writeConfig(t, file), "-r", "10", "-l", "1"})
		if err != nil || cfg.ReportInterval != 10*time.Second || cfg.RateLimit != 1 || cfg.PollInterval != 3*time.Second {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})

	t.Run("env CONFIG over -c", func(t *testing.T) {
		clearEnv(t, agentEnv...)
		t.Setenv("CONFIG", writeConfig(t, `{"address": "env-file:1"}`))
		cfg, err := LoadAgent([]string{"-c", writeConfig(t, `{"address": "flag-file:2"}`)})
		if err != nil || cfg.Address != "env-file:1" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
}

func TestAgentConfigFileErrors(t *testing.T) {
	tests := map[string]string{
		"invalid json":         `{"poll_interval": `,
		"unknown field":        `{"adress": "x"}`,
		"bad poll duration":    `{"poll_interval": "abc"}`,
		"bad report duration":  `{"report_interval": "abc"}`,
		"report below minimum": `{"report_interval": "500ms"}`,
		"server only field":    `{"store_interval": "1s"}`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			clearEnv(t, agentEnv...)
			if _, err := LoadAgent([]string{"-c", writeConfig(t, content)}); err == nil {
				t.Fatal("expected error")
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		clearEnv(t, agentEnv...)
		if _, err := LoadAgent([]string{"-c", filepath.Join(t.TempDir(), "nope.json")}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestConfigFileErrorMentionsPath(t *testing.T) {
	clearEnv(t, serverEnv...)

	missing := filepath.Join(t.TempDir(), "nope.json")
	_, err := LoadServer([]string{"-c", missing})
	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("missing file: error %v does not mention %s", err, missing)
	}

	for name, content := range map[string]string{"invalid json": `{"address": `, "bad duration": `{"store_interval": "abc"}`} {
		path := writeConfig(t, content)
		_, err := LoadServer([]string{"-c", path})
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("%s: error %v does not mention %s", name, err, path)
		}
	}
}
