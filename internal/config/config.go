package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

const (
	defaultAddress         = "localhost:8080"
	defaultStoreInterval   = 300
	defaultFileStoragePath = "metrics-db.json"
)

type ServerConfig struct {
	Address         string
	StoreInterval   int    // секунды; 0 означает синхронную запись
	FileStoragePath string // пустая строка отключает сохранение
	Restore         bool
}

type AgentConfig struct {
	Address        string
	PollInterval   int
	ReportInterval int
}

// Приоритет: переменная окружения -> флаг -> значение по умолчанию.
func LoadServer(args []string) (ServerConfig, error) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	address := fs.String("a", defaultAddress, "server address")
	storeInterval := fs.Int("i", defaultStoreInterval, "store interval in seconds (0 - synchronous)")
	filePath := fs.String("f", defaultFileStoragePath, "file storage path (empty - disabled)")
	restore := fs.Bool("r", true, "restore metrics from file on start")
	if err := fs.Parse(args); err != nil {
		return ServerConfig{}, err
	}

	cfg := ServerConfig{
		Address:         *address,
		StoreInterval:   *storeInterval,
		FileStoragePath: *filePath,
		Restore:         *restore,
	}
	if v, ok := os.LookupEnv("ADDRESS"); ok && v != "" {
		cfg.Address = v
	}
	if v, ok := os.LookupEnv("STORE_INTERVAL"); ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return ServerConfig{}, fmt.Errorf("STORE_INTERVAL must be a non-negative integer (seconds), got %q", v)
		}
		cfg.StoreInterval = n
	}
	if v, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		cfg.FileStoragePath = v
	}
	if v, ok := os.LookupEnv("RESTORE"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return ServerConfig{}, fmt.Errorf("RESTORE must be true or false, got %q", v)
		}
		cfg.Restore = b
	}
	return cfg, nil
}

func LoadAgent(args []string) (AgentConfig, error) {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	address := fs.String("a", defaultAddress, "server address")
	poll := fs.Int("p", 2, "poll interval in seconds")
	report := fs.Int("r", 10, "report interval in seconds")
	if err := fs.Parse(args); err != nil {
		return AgentConfig{}, err
	}

	cfg := AgentConfig{Address: *address, PollInterval: *poll, ReportInterval: *report}
	if v, ok := os.LookupEnv("ADDRESS"); ok && v != "" {
		cfg.Address = v
	}
	if err := envSeconds("POLL_INTERVAL", &cfg.PollInterval); err != nil {
		return AgentConfig{}, err
	}
	if err := envSeconds("REPORT_INTERVAL", &cfg.ReportInterval); err != nil {
		return AgentConfig{}, err
	}
	return cfg, nil
}

func envSeconds(name string, dst *int) error {
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fmt.Errorf("%s must be a positive integer (seconds), got %q", name, v)
	}
	*dst = n
	return nil
}
