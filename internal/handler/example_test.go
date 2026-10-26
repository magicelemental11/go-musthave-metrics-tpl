package handler_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

type okPinger struct{}

func (okPinger) PingContext(context.Context) error { return nil }

func newRouter() http.Handler {
	h := handler.NewHandler(repository.NewMemStorage(), okPinger{})
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	r.Post("/update", h.UpdateMetricJSON)
	r.Post("/updates/", h.UpdateMetricsBatch)
	r.Post("/value", h.GetMetricJSON)
	r.Get("/value/{type}/{name}", h.GetMetric)
	r.Get("/ping", h.Ping)
	return r
}

func do(router http.Handler, method, path, body string) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	out, _ := io.ReadAll(rec.Body)
	fmt.Println(strings.TrimSpace(fmt.Sprintf("%d %s", rec.Code, out)))
}

func ExampleHandler_UpdateMetric() {
	router := newRouter()
	do(router, http.MethodPost, "/update/gauge/Alloc/1.5", "")
	do(router, http.MethodPost, "/update/gauge/Alloc/abc", "")
	do(router, http.MethodPost, "/update/histogram/Alloc/1", "")
	// Output:
	// 200
	// 400 invalid gauge value
	// 400 unknown metric type
}

func ExampleHandler_GetMetric() {
	router := newRouter()
	do(router, http.MethodPost, "/update/gauge/Alloc/1.5", "")
	do(router, http.MethodGet, "/value/gauge/Alloc", "")
	do(router, http.MethodGet, "/value/gauge/Missing", "")
	// Output:
	// 200
	// 200 1.5
	// 404 gauge not found
}

func ExampleHandler_UpdateMetricJSON() {
	router := newRouter()
	do(router, http.MethodPost, "/update", `{"id":"PollCount","type":"counter","delta":2}`)
	do(router, http.MethodPost, "/update", `{"id":"PollCount","type":"counter","delta":3}`)
	// Output:
	// 200 {"id":"PollCount","type":"counter","delta":2}
	// 200 {"id":"PollCount","type":"counter","delta":5}
}

func ExampleHandler_UpdateMetricsBatch() {
	router := newRouter()
	do(router, http.MethodPost, "/updates/", `[{"id":"Alloc","type":"gauge","value":1.5},{"id":"PollCount","type":"counter","delta":1}]`)
	do(router, http.MethodPost, "/updates/", `[]`)
	// Output:
	// 200
	// 400 {"error":"empty batch"}
}

func ExampleHandler_GetMetricJSON() {
	router := newRouter()
	do(router, http.MethodPost, "/update/gauge/Alloc/1.5", "")
	do(router, http.MethodPost, "/value", `{"id":"Alloc","type":"gauge"}`)
	// Output:
	// 200
	// 200 {"id":"Alloc","type":"gauge","value":1.5}
}

func ExampleHandler_Ping() {
	router := newRouter()
	do(router, http.MethodGet, "/ping", "")
	// Output:
	// 200
}
