package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func main() {
	address := flag.String("a", "localhost:8080", "server address")
	flag.Parse()

	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage)

	r := chi.NewRouter()

	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

	r.Post("/update/{type}/{name}/", http.NotFound)
	r.Post("/update/{type}/", http.NotFound)
	r.Post("/update/", http.NotFound)

	r.Get("/value/{type}/{name}", h.GetMetric)

	r.Get("/", h.GetAllMetrics)

	log.Println("Server address:", *address)
	http.ListenAndServe(*address, r)
}
