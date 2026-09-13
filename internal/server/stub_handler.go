package server

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

func (s *Server) journalStub(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, pages.Stub(view.StubPage{Shell: s.shell(r, "Журнал", "/journal"), Heading: "Журнал"}))
}

func (s *Server) diaryStub(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, pages.Stub(view.StubPage{Shell: s.shell(r, "Дневник", "/diary"), Heading: "Дневник"}))
}
