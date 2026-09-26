package http

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

type HealthChecker func(ctx context.Context) error

func NewRouter(handler *PromoHandler, jwtSecret string, log zerolog.Logger, dbCheck HealthChecker) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(RequestID)
	r.Use(Logger(log))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := dbCheck(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Route("/api/promos", func(r chi.Router) {
		r.Use(Auth(jwtSecret))

		// Le siège seul gère les codes.
		r.Group(func(r chi.Router) {
			r.Post("/", handler.Create)
			r.Get("/", handler.List)
			r.Put("/{id}", handler.Update)
			r.Delete("/{id}", handler.Delete)
		})

		// Tout utilisateur authentifié peut vérifier/consommer un code.
		r.Get("/preview", handler.Preview)
		r.Post("/redeem", handler.Redeem)
	})

	return r
}
