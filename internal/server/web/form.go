package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/validation"
)

func PathID(r *http.Request) (int64, bool) {
	return PathValue(r, "id")
}

func PathValue(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}

func FormValue(r *http.Request, name string) string {
	return strings.TrimSpace(r.PostFormValue(name))
}

func FormInt64(r *http.Request, name string) int64 {
	value, err := strconv.ParseInt(FormValue(r, name), 10, 64)
	if err != nil || value < 0 {
		return 0
	}

	return value
}

func FormErrors(err error) (validation.Errors, bool) {
	var errs validation.Errors
	if errors.As(err, &errs) {
		return errs, true
	}

	return nil, false
}

func (b *Base) ServerError(w http.ResponseWriter, r *http.Request, message string, err error) {
	b.log.ErrorContext(r.Context(), message, slog.Any("error", err))
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (b *Base) HandleServiceError(w http.ResponseWriter, r *http.Request, message string, err error) {
	if errors.Is(err, school.ErrNotFound) || errors.Is(err, auth.ErrNotFound) ||
		errors.Is(err, journal.ErrNotFound) || errors.Is(err, journal.ErrForbidden) {
		http.NotFound(w, r)
		return
	}

	b.ServerError(w, r, message, err)
}
