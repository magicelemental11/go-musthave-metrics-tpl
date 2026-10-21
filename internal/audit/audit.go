// Package audit publishes events about metric updates to registered observers.
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Event describes one successful update request.
type Event struct {
	TS        int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// Observer receives published events. Observers that implement io.Closer are closed by Subject.Close.
type Observer interface {
	Notify(ctx context.Context, e Event) error
}

// Subject fans an Event out to attached observers.
type Subject struct {
	observers []Observer
}

// Attach registers o. It is not safe to call concurrently with Publish.
func (s *Subject) Attach(o Observer) {
	s.observers = append(s.observers, o)
}

// Publish notifies every observer in order. Observer errors are logged and do not stop delivery.
// Publish on a nil Subject does nothing.
func (s *Subject) Publish(ctx context.Context, e Event) {
	if s == nil {
		return
	}
	for _, o := range s.observers {
		if err := o.Notify(ctx, e); err != nil {
			log.Printf("audit: %v", err)
		}
	}
}

// Close closes all observers that implement io.Closer and joins their errors.
func (s *Subject) Close() error {
	var errs []error
	for _, o := range s.observers {
		if c, ok := o.(io.Closer); ok {
			errs = append(errs, c.Close())
		}
	}
	return errors.Join(errs...)
}

// FileObserver appends each event as a JSON line to a file.
type FileObserver struct {
	mu sync.Mutex
	f  *os.File
}

// NewFileObserver opens path for appending, creating it if needed.
func NewFileObserver(path string) (*FileObserver, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open audit file: %w", err)
	}
	return &FileObserver{f: f}, nil
}

// Notify writes e as one JSON line.
func (o *FileObserver) Notify(_ context.Context, e Event) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	o.mu.Lock()
	defer o.mu.Unlock()
	if _, err := o.f.Write(line); err != nil {
		return fmt.Errorf("write audit file: %w", err)
	}
	return nil
}

// Close closes the file.
func (o *FileObserver) Close() error {
	return o.f.Close()
}

// HTTPObserver POSTs each event as JSON to a URL.
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver creates an HTTPObserver with a 2 second request timeout.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{url: url, client: &http.Client{Timeout: 2 * time.Second}}
}

// Notify sends e and returns an error unless the receiver responds 2xx.
func (o *HTTPObserver) Notify(ctx context.Context, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("send audit event: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("audit receiver responded %d", resp.StatusCode)
	}
	return nil
}
