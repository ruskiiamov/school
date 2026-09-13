package server

import (
	"net/http"
	"strconv"
)

type rowEdit struct {
	id      int64
	name    string
	entered bool
	message string
}

func showInactive(r *http.Request) bool {
	return r.URL.Query().Get("inactive") == "1"
}

func editingID(r *http.Request, edit rowEdit) int64 {
	if edit.entered {
		return edit.id
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("edit"), 10, 64)
	if err != nil {
		return 0
	}

	return id
}

func catalogListURL(base string, r *http.Request) string {
	if showInactive(r) {
		return base + "?inactive=1"
	}

	return base
}
