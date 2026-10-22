package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

type errWriter func(w http.ResponseWriter, status int, message string)

func plainError(w http.ResponseWriter, status int, message string) {
	http.Error(w, message, status)
}

func storageError(w http.ResponseWriter, write errWriter, op string, err error, notFound string) {
	if errors.Is(err, repository.ErrNotFound) {
		write(w, http.StatusNotFound, notFound)
		return
	}
	log.Printf("%s: %v", op, err)
	write(w, http.StatusInternalServerError, "storage error")
}
