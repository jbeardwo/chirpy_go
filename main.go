package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	const fileRoot = "."
	const port = "8080"
	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
	}
	serveMux := http.NewServeMux()
	fs := http.FileServer(http.Dir(fileRoot))
	serveMux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", fs)))
	serveMux.HandleFunc("GET /api/healthz", readyHandler)
	serveMux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	serveMux.HandleFunc("POST /admin/reset", apiCfg.resetMetricsHandler)
	serveMux.HandleFunc("POST /api/validate_chirp", apiCfg.validateChirpHandler)
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

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	msg := fmt.Sprintf(
		`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`,
		cfg.fileserverHits.Load())
	_, _ = w.Write([]byte(msg))
}

func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	type errReturnVals struct {
		Error string `json:"error"`
	}
	type validReturnVals struct {
		Valid bool `json:"valid"`
	}
	w.Header().Set("Content-Type", "application/json")
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respBody := errReturnVals{
			Error: "Something went wrong",
		}
		dat, _ := json.Marshal(respBody)
		w.WriteHeader(500)
		_, _ = w.Write(dat)
		return
	}
	if len(params.Body) > 140 {
		respBody := errReturnVals{
			Error: "Chirp is too long",
		}
		dat, _ := json.Marshal(respBody)
		w.WriteHeader(400)
		_, _ = w.Write(dat)
		return
	}
	respBody := validReturnVals{
		Valid: true,
	}
	dat, _ := json.Marshal(respBody)
	w.WriteHeader(200)
	_, _ = w.Write(dat)
}
