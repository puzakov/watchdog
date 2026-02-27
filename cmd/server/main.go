package main

import (
	"flag"
	"net/http"

	"github.com/puzakov/watchdog/internal/handler"
	"github.com/puzakov/watchdog/internal/service"
)

func main() {
	var addr string

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.Parse()

	storage := service.NewMemStorage()
	h := handler.NewHandler(storage)

	err := http.ListenAndServe(addr, h)
	if err != nil {
		panic(err)
	}
}
