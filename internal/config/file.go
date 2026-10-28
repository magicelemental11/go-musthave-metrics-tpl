package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

type duration time.Duration

func (d *duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"1s\": %w", err)
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = duration(parsed)
	return nil
}

type serverFile struct {
	Address       *string   `json:"address"`
	Restore       *bool     `json:"restore"`
	StoreInterval *duration `json:"store_interval"`
	StoreFile     *string   `json:"store_file"`
	DatabaseDSN   *string   `json:"database_dsn"`
	CryptoKey     *string   `json:"crypto_key"`
	Key           *string   `json:"key"`
	AuditFile     *string   `json:"audit_file"`
	AuditURL      *string   `json:"audit_url"`
	PprofAddress  *string   `json:"pprof_address"`
	TrustedSubnet *string   `json:"trusted_subnet"`
}

type agentFile struct {
	Address        *string   `json:"address"`
	ReportInterval *duration `json:"report_interval"`
	PollInterval   *duration `json:"poll_interval"`
	CryptoKey      *string   `json:"crypto_key"`
	Key            *string   `json:"key"`
	RateLimit      *int      `json:"rate_limit"`
}

// loadFile decodes the JSON config at path into dst. An empty path leaves dst untouched.
func loadFile(path string, dst any) error {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("config file: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err = dec.Decode(dst); err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}
	return nil
}

// setFlags returns the names of the flags passed explicitly, so that a flag equal to its default still beats the file.
func setFlags(fs *flag.FlagSet) map[string]bool {
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

// fromFile replaces *dst with the file value unless the flag was passed explicitly.
func fromFile[T any](set map[string]bool, flagName string, dst *T, fileValue *T) {
	if fileValue != nil && !set[flagName] {
		*dst = *fileValue
	}
}

func configFlag(fs *flag.FlagSet) *string {
	var path string
	fs.StringVar(&path, "c", "", "path to the JSON config file")
	fs.StringVar(&path, "config", "", "path to the JSON config file")
	return &path
}
