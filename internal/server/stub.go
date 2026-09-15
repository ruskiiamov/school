package server

import (
	"net/http"

	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

func (s *Server) diaryStub(w http.ResponseWriter, r *http.Request) {
	s.base.Render(w, r, pages.Stub(view.StubPage{Shell: s.base.Shell(r, "Дневник", "/diary"), Heading: "Дневник"}))
}
