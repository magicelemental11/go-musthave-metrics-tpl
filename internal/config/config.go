package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultAddress         = "localhost:8080"
	defaultStoreInterval   = 300
	defaultFileStoragePath = "metrics-db.json"
)

type ServerConfig struct {
	Address         string
	StoreInterval   time.Duration // 0 — синхронная запись
	FileStoragePath string        // пустая строка отключает сохранение
	Restore         bool
	DatabaseDSN     string // пустая строка — БД не используется
}

type AgentConfig struct {
	Address        string
	PollInterval   time.Duration
	ReportInterval time.Duration
}

// LoadServer: приоритет — переменная окружения, флаг, значение по умолчанию.
func LoadServer(args []string) (ServerConfig, error) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	address := fs.String("a", defaultAddress, "server address")
	storeInterval := fs.Int("i", defaultStoreInterval, "store interval in seconds (0 - synchronous)")
	filePath := fs.String("f", defaultFileStoragePath, "file storage path (empty - disabled)")
	restore := fs.Bool("r", true, "restore metrics from file on start")
	dsn := fs.String("d", "", "database DSN (empty - disabled)")
	if err := fs.Parse(args); err != nil {
		return ServerConfig{}, err
	}

	interval, err := seconds("STORE_INTERVAL", *storeInterval, 0)
	if err != nil {
		return ServerConfig{}, err
	}
	restoreValue := *restore
	if v := os.Getenv("RESTORE"); v != "" {
		restoreValue, err = strconv.ParseBool(v)
		if err != nil {
			return ServerConfig{}, fmt.Errorf("RESTORE must be true or false, got %q", v)
		}
	}
	filePathValue := *filePath
	if v, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		filePathValue = v
	}

	return ServerConfig{
		Address:         envString("ADDRESS", *address),
		StoreInterval:   interval,
		FileStoragePath: filePathValue,
		Restore:         restoreValue,
		DatabaseDSN:     envString("DATABASE_DSN", *dsn),
	}, nil
}

// LoadAgent: приоритет — переменная окружения, флаг, значение по умолчанию.
func LoadAgent(args []string) (AgentConfig, error) {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	address := fs.String("a", defaultAddress, "server address")
	poll := fs.Int("p", 2, "poll interval in seconds")
	report := fs.Int("r", 10, "report interval in seconds")
	if err := fs.Parse(args); err != nil {
		return AgentConfig{}, err
	}

	pollInterval, err := seconds("POLL_INTERVAL", *poll, 1)
	if err != nil {
		return AgentConfig{}, err
	}
	reportInterval, err := seconds("REPORT_INTERVAL", *report, 1)
	if err != nil {
		return AgentConfig{}, err
	}

	return AgentConfig{
		Address:        envString("ADDRESS", *address),
		PollInterval:   pollInterval,
		ReportInterval: reportInterval,
	}, nil
}

func envString(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// seconds берёт интервал из переменной окружения name, а если её нет — из флага.
// Значение не должно быть меньше min секунд.
func seconds(name string, flagValue, min int) (time.Duration, error) {
	n := flagValue
	if v := os.Getenv(name); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer number of seconds, got %q", name, v)
		}
		n = parsed
	}
	if n < min {
		return 0, fmt.Errorf("%s (or its flag) must be at least %d seconds, got %d", name, min, n)
	}
	return time.Duration(n) * time.Second, nil
}
