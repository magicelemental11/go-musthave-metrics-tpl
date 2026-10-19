package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

const defaultAddress = "localhost:8080"

type ServerConfig struct {
	Address string
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
	if err := fs.Parse(args); err != nil {
		return ServerConfig{}, err
	}

	return ServerConfig{Address: envString("ADDRESS", *address)}, nil
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
