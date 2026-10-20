package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func main() {
	cfg, err := config.LoadServer(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage)

	r := chi.NewRouter()

	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

	r.Post("/update/{type}/{name}/", http.NotFound)
	r.Post("/update/{type}/", http.NotFound)
	r.Post("/update/", http.NotFound)

	r.Get("/value/{type}/{name}", h.GetMetric)

	r.Get("/", h.GetAllMetrics)

	log.Println("Server address:", cfg.Address)
	http.ListenAndServe(cfg.Address, r)
}
