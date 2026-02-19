package main

import (
	"net/http"

	"github.com/puzakov/watchdog/internal/handler"
	"github.com/puzakov/watchdog/internal/service"
)

func main() {
	storage := service.NewMemStorage()
	h := handler.NewHandler(storage)

	err := http.ListenAndServe(`:8080`, h)
	if err != nil {
		panic(err)
	}
}
