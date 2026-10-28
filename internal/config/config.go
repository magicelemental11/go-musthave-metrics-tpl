// Package config loads server and agent settings from environment variables and flags.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

const (
	defaultAddress         = "localhost:8080"
	defaultStoreInterval   = 300
	defaultFileStoragePath = "metrics-db.json"
)

// ServerConfig holds the server settings.
type ServerConfig struct {
	Address         string
	StoreInterval   time.Duration // 0 means synchronous save
	FileStoragePath string        // empty disables saving
	Restore         bool
	DatabaseDSN     string // empty means no database
	Key             string // empty disables signing
	CryptoKey       string // path to the RSA private key, empty disables decryption
	AuditFile       string
	AuditURL        string
	PprofAddress    string
	TrustedSubnet   string // CIDR of agents allowed to send metrics, empty allows everyone
}

// AgentConfig holds the agent settings.
type AgentConfig struct {
	Address        string
	PollInterval   time.Duration
	ReportInterval time.Duration
	Key            string
	CryptoKey      string // path to the RSA public key, empty disables encryption
	RateLimit      int
}

// LoadServer parses args into a ServerConfig. Precedence: environment variable, flag, config file, default.
func LoadServer(args []string) (ServerConfig, error) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	address := fs.String("a", defaultAddress, "server address")
	storeInterval := fs.Int("i", defaultStoreInterval, "store interval in seconds (0 - synchronous)")
	filePath := fs.String("f", defaultFileStoragePath, "file storage path (empty - disabled)")
	restore := fs.Bool("r", true, "restore metrics from file on start")
	dsn := fs.String("d", "", "database DSN (empty - disabled)")
	key := fs.String("k", "", "signing key (empty - disabled)")
	cryptoKey := fs.String("crypto-key", "", "path to the RSA private key (empty - disabled)")
	auditFile := fs.String("audit-file", "", "audit log file path (empty - disabled)")
	auditURL := fs.String("audit-url", "", "audit receiver URL (empty - disabled)")
	pprofAddress := fs.String("pprof-address", "", "pprof listen address (empty - disabled)")
	trustedSubnet := fs.String("t", "", "trusted agent subnet in CIDR notation (empty - no check)")
	configPath := configFlag(fs)
	if err := fs.Parse(args); err != nil {
		return ServerConfig{}, err
	}

	var file serverFile
	if err := loadFile(envString("CONFIG", *configPath), &file); err != nil {
		return ServerConfig{}, err
	}
	set := setFlags(fs)
	fromFile(set, "a", address, file.Address)
	fromFile(set, "r", restore, file.Restore)
	fromFile(set, "f", filePath, file.StoreFile)
	fromFile(set, "d", dsn, file.DatabaseDSN)
	fromFile(set, "k", key, file.Key)
	fromFile(set, "crypto-key", cryptoKey, file.CryptoKey)
	fromFile(set, "audit-file", auditFile, file.AuditFile)
	fromFile(set, "audit-url", auditURL, file.AuditURL)
	fromFile(set, "pprof-address", pprofAddress, file.PprofAddress)
	fromFile(set, "t", trustedSubnet, file.TrustedSubnet)

	storeEvery, err := interval("STORE_INTERVAL", *storeInterval, set["i"], file.StoreInterval, 0)
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
	trustedSubnetValue := envString("TRUSTED_SUBNET", *trustedSubnet)
	if _, err = subnet.New(trustedSubnetValue); err != nil {
		return ServerConfig{}, err
	}
	filePathValue := *filePath
	if v, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		filePathValue = v
	}

	return ServerConfig{
		Address:         envString("ADDRESS", *address),
		StoreInterval:   storeEvery,
		FileStoragePath: filePathValue,
		Restore:         restoreValue,
		DatabaseDSN:     envString("DATABASE_DSN", *dsn),
		Key:             envString("KEY", *key),
		CryptoKey:       envString("CRYPTO_KEY", *cryptoKey),
		AuditFile:       envString("AUDIT_FILE", *auditFile),
		AuditURL:        envString("AUDIT_URL", *auditURL),
		PprofAddress:    envString("PPROF_ADDRESS", *pprofAddress),
		TrustedSubnet:   trustedSubnetValue,
	}, nil
}

// LoadAgent parses args into an AgentConfig. Precedence: environment variable, flag, config file, default.
func LoadAgent(args []string) (AgentConfig, error) {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	address := fs.String("a", defaultAddress, "server address")
	poll := fs.Int("p", 2, "poll interval in seconds")
	report := fs.Int("r", 10, "report interval in seconds")
	key := fs.String("k", "", "signing key (empty - disabled)")
	cryptoKey := fs.String("crypto-key", "", "path to the RSA public key (empty - disabled)")
	limit := fs.Int("l", 1, "max concurrent requests to the server")
	configPath := configFlag(fs)
	if err := fs.Parse(args); err != nil {
		return AgentConfig{}, err
	}

	var file agentFile
	if err := loadFile(envString("CONFIG", *configPath), &file); err != nil {
		return AgentConfig{}, err
	}
	set := setFlags(fs)
	fromFile(set, "a", address, file.Address)
	fromFile(set, "k", key, file.Key)
	fromFile(set, "crypto-key", cryptoKey, file.CryptoKey)
	fromFile(set, "l", limit, file.RateLimit)

	pollInterval, err := interval("POLL_INTERVAL", *poll, set["p"], file.PollInterval, 1)
	if err != nil {
		return AgentConfig{}, err
	}
	reportInterval, err := interval("REPORT_INTERVAL", *report, set["r"], file.ReportInterval, 1)
	if err != nil {
		return AgentConfig{}, err
	}

	rateLimit := *limit
	if v := os.Getenv("RATE_LIMIT"); v != "" {
		rateLimit, err = strconv.Atoi(v)
		if err != nil {
			return AgentConfig{}, fmt.Errorf("RATE_LIMIT must be an integer, got %q", v)
		}
	}
	if rateLimit < 1 {
		return AgentConfig{}, fmt.Errorf("RATE_LIMIT (or -l) must be at least 1, got %d", rateLimit)
	}

	return AgentConfig{
		Address:        envString("ADDRESS", *address),
		PollInterval:   pollInterval,
		ReportInterval: reportInterval,
		Key:            envString("KEY", *key),
		CryptoKey:      envString("CRYPTO_KEY", *cryptoKey),
		RateLimit:      rateLimit,
	}, nil
}

func envString(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// interval resolves an interval: environment variable (seconds), then flag (seconds), then file, then the flag default.
// The result must not be less than min seconds.
func interval(name string, flagValue int, flagSet bool, fileValue *duration, min int) (time.Duration, error) {
	d := time.Duration(flagValue) * time.Second
	if v := os.Getenv(name); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer number of seconds, got %q", name, v)
		}
		d = time.Duration(n) * time.Second
	} else if !flagSet && fileValue != nil {
		d = time.Duration(*fileValue)
	}
	if d < time.Duration(min)*time.Second {
		return 0, fmt.Errorf("%s (or its flag or config file entry) must be at least %d seconds, got %s", name, min, d)
	}
	return d, nil
}
