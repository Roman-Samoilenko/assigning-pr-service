package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"prreviewer/internal/handlers"
	"prreviewer/internal/repo"
	"prreviewer/internal/service"
)

const (
	defaultPort        = "8080"
	defaultDBURL       = "postgres://app:app@localhost:5432/prreviewer?sslmode=disable"
	requestTimeout     = 5 * time.Second
	serverReadTimeout  = 10 * time.Second
	serverWriteTimeout = 10 * time.Second
	serverIdleTimeout  = 60 * time.Second
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("DATABASE_URL not set, using default")
		dbURL = defaultDBURL
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = defaultPort
	}

	log.Println("Starting application initialization")
	runMigrations(dbURL)

	log.Println("Connecting to database")
	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Database connection established")

	repo := repo.New(db)
	svc := service.New(repo, rng)
	h := handlers.New(svc)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(requestTimeout))

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	router.Post("/team/add", h.TeamAdd)
	router.Get("/team/get", h.TeamGet)
	router.Post("/team/deactivate", h.TeamDeactivate)
	router.Post("/users/setIsActive", h.UsersSetIsActive)
	router.Get("/users/getReview", h.UsersGetReview)
	router.Post("/pullRequest/create", h.PRCreate)
	router.Post("/pullRequest/merge", h.PRMerge)
	router.Post("/pullRequest/reassign", h.PRReassign)
	router.Get("/stats", h.Stats)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}

	log.Printf("Server starting on :%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func runMigrations(dbURL string) {
	log.Println("Running database migrations")
	m, err := migrate.New("file:///migrations", dbURL)
	if err != nil {
		log.Printf("Migration init error: %v", err)
		return
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Printf("Migration up error: %v", err)
	} else if errors.Is(err, migrate.ErrNoChange) {
		log.Println("No new migrations to apply")
	} else {
		log.Println("Migrations applied successfully")
	}
}
