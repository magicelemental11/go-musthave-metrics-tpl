package config

import "testing"

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
		cfg, _ := LoadServer([]string{"-a", "localhost:9090"})
		if cfg.Address != "localhost:9090" {
			t.Fatalf("got %q", cfg.Address)
		}
	})
	t.Run("env over flag", func(t *testing.T) {
		t.Setenv("ADDRESS", "localhost:7070")
		cfg, _ := LoadServer([]string{"-a", "localhost:9090"})
		if cfg.Address != "localhost:7070" {
			t.Fatalf("got %q", cfg.Address)
		}
	})
}

func clearServerEnv(t *testing.T) {
	for _, name := range []string{"ADDRESS", "STORE_INTERVAL", "RESTORE"} {
		t.Setenv(name, "")
	}
}

func TestLoadServerStorageDefaults(t *testing.T) {
	clearServerEnv(t)
	cfg, err := LoadServer(nil)
	want := ServerConfig{Address: "localhost:8080", StoreInterval: 300, FileStoragePath: "metrics-db.json", Restore: true}
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
	t.Run("flags", func(t *testing.T) {
		t.Setenv("ADDRESS", "")
		t.Setenv("POLL_INTERVAL", "")
		t.Setenv("REPORT_INTERVAL", "")
		cfg, err := LoadAgent([]string{"-a", "h:1", "-p", "3", "-r", "5"})
		if err != nil || cfg != (AgentConfig{"h:1", 3, 5}) {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("env over flags", func(t *testing.T) {
		t.Setenv("ADDRESS", "h:2")
		t.Setenv("POLL_INTERVAL", "7")
		t.Setenv("REPORT_INTERVAL", "20")
		cfg, err := LoadAgent([]string{"-a", "h:1", "-p", "3", "-r", "5"})
		if err != nil || cfg != (AgentConfig{"h:2", 7, 20}) {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("invalid env", func(t *testing.T) {
		t.Setenv("POLL_INTERVAL", "abc")
		if _, err := LoadAgent(nil); err == nil {
			t.Fatal("expected error")
		}
	})
}
