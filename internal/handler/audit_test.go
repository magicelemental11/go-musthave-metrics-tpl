package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/audit"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

type fakePublisher struct{ events []audit.Event }

func (f *fakePublisher) Publish(_ context.Context, e audit.Event) { f.events = append(f.events, e) }

func newAuditRouter(s repository.Storage, p Publisher) http.Handler {
	h := NewHandler(s, nil).WithAudit(p)
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	r.Post("/update", h.UpdateMetricJSON)
	r.Post("/updates/", h.UpdateMetricsBatch)
	return r
}

func postFrom(r http.Handler, path, body, remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestAuditPublishedOnSuccess(t *testing.T) {
	tests := []struct {
		name, path, body string
		want             []string
	}{
		{"url gauge", "/update/gauge/Alloc/1.5", "", []string{"Alloc"}},
		{"url counter", "/update/counter/PollCount/2", "", []string{"PollCount"}},
		{"json", "/update", `{"id":"Frees","type":"gauge","value":3}`, []string{"Frees"}},
		{"batch", "/updates/", `[{"id":"a","type":"gauge","value":1},{"id":"b","type":"counter","delta":2}]`, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub := &fakePublisher{}
			before := time.Now().Unix()
			rr := postFrom(newAuditRouter(repository.NewMemStorage(), pub), tt.path, tt.body, "192.168.0.42:5555", nil)
			if rr.Code != http.StatusOK {
				t.Fatalf("code=%d body=%s", rr.Code, rr.Body)
			}
			if len(pub.events) != 1 {
				t.Fatalf("events=%+v", pub.events)
			}
			e := pub.events[0]
			if !reflect.DeepEqual(e.Metrics, tt.want) || e.IPAddress != "192.168.0.42" || e.TS < before {
				t.Fatalf("got %+v", e)
			}
		})
	}
}

func TestAuditIPFromXRealIP(t *testing.T) {
	pub := &fakePublisher{}
	postFrom(newAuditRouter(repository.NewMemStorage(), pub), "/update/gauge/a/1", "", "10.0.0.1:1", map[string]string{"X-Real-IP": "203.0.113.7"})
	if len(pub.events) != 1 || pub.events[0].IPAddress != "203.0.113.7" {
		t.Fatalf("events=%+v", pub.events)
	}
}

func TestAuditIPFromRemoteAddr(t *testing.T) {
	pub := &fakePublisher{}
	postFrom(newAuditRouter(repository.NewMemStorage(), pub), "/update/gauge/a/1", "", "10.0.0.1:1234", nil)
	if len(pub.events) != 1 || pub.events[0].IPAddress != "10.0.0.1" {
		t.Fatalf("events=%+v", pub.events)
	}
}

func TestAuditNotPublishedOnFailure(t *testing.T) {
	tests := []struct {
		name, path, body string
		storage          repository.Storage
	}{
		{"bad url value", "/update/gauge/a/x", "", repository.NewMemStorage()},
		{"unknown type", "/update/histogram/a/1", "", repository.NewMemStorage()},
		{"bad json", "/update", `{`, repository.NewMemStorage()},
		{"missing value", "/update", `{"id":"a","type":"gauge"}`, repository.NewMemStorage()},
		{"empty batch", "/updates/", `[]`, repository.NewMemStorage()},
		{"invalid batch", "/updates/", `[{"id":"a","type":"gauge"}]`, repository.NewMemStorage()},
		{"storage error url", "/update/gauge/a/1", "", failingStorage{err: errors.New("db")}},
		{"storage error json", "/update", `{"id":"a","type":"gauge","value":1}`, failingStorage{err: errors.New("db")}},
		{"storage error batch", "/updates/", `[{"id":"a","type":"gauge","value":1}]`, failingStorage{err: errors.New("db")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub := &fakePublisher{}
			rr := postFrom(newAuditRouter(tt.storage, pub), tt.path, tt.body, "1.1.1.1:1", nil)
			if rr.Code == http.StatusOK {
				t.Fatalf("expected failure, got 200")
			}
			if len(pub.events) != 0 {
				t.Fatalf("events=%+v", pub.events)
			}
		})
	}
}

func TestWithoutAuditStillWorks(t *testing.T) {
	h := NewHandler(repository.NewMemStorage(), nil)
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	if rr := postFrom(r, "/update/gauge/a/1", "", "1.1.1.1:1", nil); rr.Code != http.StatusOK {
		t.Fatalf("code=%d", rr.Code)
	}
}

type ctxPublisher struct{ err error }

func (c *ctxPublisher) Publish(ctx context.Context, _ audit.Event) { c.err = ctx.Err() }

func TestAuditContextSurvivesClientCancel(t *testing.T) {
	pub := &ctxPublisher{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/a/1", nil).WithContext(ctx)
	newAuditRouter(repository.NewMemStorage(), pub).ServeHTTP(httptest.NewRecorder(), req)
	if pub.err != nil {
		t.Fatalf("publish ctx err=%v", pub.err)
	}
}
