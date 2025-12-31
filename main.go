package main

import (
	"fmt"
	"net/http"
)

func main() {
	const fileRoot = "."
	const port = "8080"
	serveMux := http.NewServeMux()
	fs := http.FileServer(http.Dir(fileRoot))
	serveMux.Handle("/app/", http.StripPrefix("/app", fs))
	serveMux.HandleFunc("/healthz", readyHandler)
	server := http.Server{
		Handler: serveMux,
		Addr:    ":" + port,
	}
	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("error with listen and serve: %v\n", err)
	}
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
