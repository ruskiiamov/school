package server

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

func (s *Server) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := component.Render(r.Context(), w); err != nil {
		s.log.ErrorContext(r.Context(), "render page",
			slog.Any("error", err),
			slog.String("path", r.URL.Path))
	}
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)

		return
	}

	http.Redirect(w, r, target, http.StatusSeeOther)
}
