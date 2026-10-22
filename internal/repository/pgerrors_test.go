package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsPgConnErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"connection failure", &pgconn.PgError{Code: pgerrcode.ConnectionFailure}, true},
		{"connection exception class", &pgconn.PgError{Code: pgerrcode.ConnectionException}, true},
		{"wrapped connection failure", fmt.Errorf("update: %w", &pgconn.PgError{Code: pgerrcode.ConnectionDoesNotExist}), true},
		{"admin shutdown", &pgconn.PgError{Code: pgerrcode.AdminShutdown}, true},
		{"crash shutdown", &pgconn.PgError{Code: pgerrcode.CrashShutdown}, true},
		{"cannot connect now", &pgconn.PgError{Code: pgerrcode.CannotConnectNow}, true},
		{"query canceled", &pgconn.PgError{Code: pgerrcode.QueryCanceled}, false},
		{"unique violation", &pgconn.PgError{Code: pgerrcode.UniqueViolation}, false},
		{"syntax error", &pgconn.PgError{Code: pgerrcode.SyntaxError}, false},
		{"plain error", errors.New("boom"), false},
		{"bad conn", fmt.Errorf("query: %w", driver.ErrBadConn), true},
		{"unexpected eof", fmt.Errorf("query: %w", io.ErrUnexpectedEOF), true},
		{"network error", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, true},
		{"context canceled", context.Canceled, false},
		{"context deadline", fmt.Errorf("query: %w", context.DeadlineExceeded), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPgConnErr(tt.err); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
