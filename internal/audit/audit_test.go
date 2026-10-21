package audit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

type recorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *recorder) Notify(_ context.Context, e Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

type failing struct{}

func (failing) Notify(context.Context, Event) error { return errors.New("boom") }

func TestSubjectDeliversToAllDespiteFailure(t *testing.T) {
	a, b := &recorder{}, &recorder{}
	s := &Subject{}
	s.Attach(a)
	s.Attach(failing{})
	s.Attach(b)

	s.Publish(context.Background(), Event{TS: 1, Metrics: []string{"Alloc"}, IPAddress: "1.2.3.4"})

	for _, r := range []*recorder{a, b} {
		if len(r.events) != 1 || r.events[0].Metrics[0] != "Alloc" {
			t.Fatalf("got %+v", r.events)
		}
	}
}

func TestSubjectWithoutObservers(t *testing.T) {
	(&Subject{}).Publish(context.Background(), Event{})
}

func TestNilSubjectPublish(t *testing.T) {
	var s *Subject
	s.Publish(context.Background(), Event{})
}

func TestFileObserverAppendsLines(t *testing.T) {
	path := t.TempDir() + "/audit.log"
	if err := os.WriteFile(path, []byte("{\"old\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := NewFileObserver(path)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	ctx := context.Background()
	if err := o.Notify(ctx, Event{TS: 1, Metrics: []string{"a", "b"}, IPAddress: "10.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	if err := o.Notify(ctx, Event{TS: 2, Metrics: []string{"c"}, IPAddress: "10.0.0.2"}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines=%q", lines)
	}
	var e Event
	if err := json.Unmarshal([]byte(lines[1]), &e); err != nil || e.TS != 1 || len(e.Metrics) != 2 || e.IPAddress != "10.0.0.1" {
		t.Fatalf("got %+v, %v", e, err)
	}
	if !strings.Contains(lines[2], `"ip_address":"10.0.0.2"`) {
		t.Fatalf("line=%q", lines[2])
	}
}

func TestFileObserverConcurrent(t *testing.T) {
	path := t.TempDir() + "/audit.log"
	o, err := NewFileObserver(path)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o.Notify(context.Background(), Event{TS: 1, Metrics: []string{"m"}, IPAddress: "1.1.1.1"})
		}()
	}
	wg.Wait()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 50 {
		t.Fatalf("lines=%d", len(lines))
	}
	for _, l := range lines {
		var e Event
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatalf("bad line %q: %v", l, err)
		}
	}
}

func TestFileObserverBadPath(t *testing.T) {
	if _, err := NewFileObserver(t.TempDir() + "/missing/dir/a.log"); err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPObserverPostsEvent(t *testing.T) {
	var got Event
	var method, ctype string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, ctype = r.Method, r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()

	want := Event{TS: 5, Metrics: []string{"Alloc", "Frees"}, IPAddress: "192.168.0.42"}
	if err := NewHTTPObserver(srv.URL).Notify(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || ctype != "application/json" {
		t.Fatalf("method=%s ctype=%s", method, ctype)
	}
	if got.TS != want.TS || got.IPAddress != want.IPAddress || len(got.Metrics) != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestHTTPObserverNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := NewHTTPObserver(srv.URL).Notify(context.Background(), Event{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSubjectCloseClosesFileObservers(t *testing.T) {
	o, err := NewFileObserver(t.TempDir() + "/audit.log")
	if err != nil {
		t.Fatal(err)
	}
	s := &Subject{}
	s.Attach(&recorder{})
	s.Attach(o)

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := o.Notify(context.Background(), Event{}); err == nil {
		t.Fatal("expected error writing to closed file")
	}
}
