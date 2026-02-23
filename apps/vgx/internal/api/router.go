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
	dashboard := &handlers.DashboardHandler{Pool: pool}
	rules := &handlers.RulesHandler{Pool: pool}
	docs := &handlers.DocumentsHandler{Pool: pool, Cfg: cfg}
	scans := &handlers.ScansHandler{Pool: pool}

	// Unauthenticated
	r.Get("/api/v1/health", health.Check)

	// Authenticated API routes (Clerk middleware will be added when configured)
	r.Route("/api/v1", func(r chi.Router) {
		// Dashboard
		r.Get("/dashboard/summary", dashboard.Summary)

		// Scans
		r.Get("/scans", scans.List)
		r.Post("/scans", scans.Create)
		r.Get("/scans/{scanID}", scans.Get)

		// Compliance rules
		r.Get("/rules", rules.List)
		r.Get("/rules/{ruleID}", handlers.Placeholder("get rule"))

		// Documents + ingestion
		r.Get("/documents", docs.List)
		r.Post("/documents", docs.Upload)
		r.Get("/documents/{docID}", handlers.Placeholder("get document"))
		r.Get("/documents/{docID}/rules", handlers.Placeholder("list proposed rules"))
		r.Post("/documents/{docID}/rules/{ruleID}/approve", handlers.Placeholder("approve rule"))
		r.Post("/documents/{docID}/rules/{ruleID}/reject", handlers.Placeholder("reject rule"))
	})

	// Serve embedded React SPA for all non-API routes
	r.Handle("/*", embed.SPAHandler())

	return r
}
