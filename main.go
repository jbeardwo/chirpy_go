package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jbeardwo/chirpy_go/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func main() {
	const fileRoot = "."
	const port = "8080"

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL must be set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)

	platform := os.Getenv("PLATFORM")
	if platform == "" {
		log.Fatal("PLATFORM must be set")
	}

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		platform:       platform,
	}

	serveMux := http.NewServeMux()
	fs := http.FileServer(http.Dir(fileRoot))
	serveMux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", fs)))
	serveMux.HandleFunc("GET /api/healthz", readyHandler)
	serveMux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	serveMux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
	serveMux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	serveMux.HandleFunc("POST /api/chirps", apiCfg.createChirpHandler)
	serveMux.HandleFunc("GET /api/chirps", apiCfg.getAllChirpsHandler)

	server := http.Server{
		Handler: serveMux,
		Addr:    ":" + port,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errReturnVals struct {
		Error string `json:"error"`
	}
	respBody := errReturnVals{
		Error: msg,
	}
	dat, _ := json.Marshal(respBody)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(dat)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(dat)
}

func replaceBadWords(in string) string {
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Split(in, " ")
	for i, word := range words {
		for _, badWord := range badWords {
			if strings.ToLower(word) == badWord {
				words[i] = "****"
			}
		}
	}
	return strings.Join(words, " ")
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
	msg := `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited ` + fmt.Sprintf("%d", cfg.fileserverHits.Load()) + ` times!</p>
  </body>
</html>`
	_, _ = w.Write([]byte(msg))
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(w, 403, "Forbidden: dev access only")
		return
	}
	cfg.fileserverHits.Store(0)
	err := cfg.db.DeleteUsers(r.Context())
	if err != nil {
		respondWithError(w, 500, "Could not delete users")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}
	if len(params.Body) == 0 {
		respondWithError(w, 400, "Chirp body is required")
		return
	}
	cleanChirp := replaceBadWords(params.Body)
	chirpParams := database.CreateChirpParams{
		Body:   cleanChirp,
		UserID: params.UserID,
	}
	dbChirp, err := cfg.db.CreateChirp(r.Context(), chirpParams)
	if err != nil {
		respondWithError(w, 500, "Could not create chirp")
		return
	}
	chirp := Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}
	respondWithJSON(w, 201, chirp)
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, "Error decoding body")
		return
	}
	dbUser, err := cfg.db.CreateUser(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, 500, "Could not create user")
		return
	}
	user := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}
	respondWithJSON(w, 201, user)
}

func (cfg *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		respondWithError(w, 500, "Error getting chirps")
		return
	}

	chirps := []Chirp{}
	for _, dbChirp := range dbChirps {
		chirps = append(chirps, Chirp{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		})
	}
	respondWithJSON(w, 200, chirps)
}
