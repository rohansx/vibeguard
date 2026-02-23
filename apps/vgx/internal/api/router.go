package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vibeguard/vgx/internal/api/handlers"
	"github.com/vibeguard/vgx/internal/config"
	"github.com/vibeguard/vgx/internal/embed"
)

func NewRouter(cfg *config.Config, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	health := &handlers.HealthHandler{}
	sync := handlers.NewSyncHandler(pool, cfg)

	// Unauthenticated
	r.Get("/api/v1/health", health.Check)

	// Team sync API (authenticated — Clerk middleware added in Phase 2)
	r.Route("/api/v1", func(r chi.Router) {
		// Repository registry
		r.Get("/repositories", sync.ListRepositories)
		r.Post("/repositories", sync.RegisterRepository)
		r.Get("/repositories/{repoID}", sync.GetRepository)

		// SPG state (team sync — mirrors local graph metadata)
		r.Get("/repositories/{repoID}/taint-paths", sync.ListTaintPaths)
		r.Get("/repositories/{repoID}/attack-surface", sync.GetAttackSurface)
		r.Post("/repositories/{repoID}/calibrate", sync.RecordCalibration)
	})

	// Serve embedded SPA for all non-API routes
	r.Handle("/*", embed.SPAHandler())

	return r
}
