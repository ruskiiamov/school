package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/validation"
)

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}

func formValue(r *http.Request, name string) string {
	return strings.TrimSpace(r.PostFormValue(name))
}

func formInt64(r *http.Request, name string) int64 {
	value, err := strconv.ParseInt(formValue(r, name), 10, 64)
	if err != nil || value < 0 {
		return 0
	}

	return value
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, message string, err error) {
	s.log.ErrorContext(r.Context(), message, slog.Any("error", err))
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (s *Server) handleServiceError(w http.ResponseWriter, r *http.Request, message string, err error) {
	if errors.Is(err, school.ErrNotFound) || errors.Is(err, auth.ErrNotFound) {
		http.NotFound(w, r)
		return
	}

	s.serverError(w, r, message, err)
}

func formErrors(err error) (validation.Errors, bool) {
	var errs validation.Errors
	if errors.As(err, &errs) {
		return errs, true
	}

	return nil, false
}
