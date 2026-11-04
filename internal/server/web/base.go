package web

import (
	"log/slog"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/buildinfo"
	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/school"
)

type Base struct {
	auth        *auth.Service
	school      *school.Service
	schoolName  string
	version     string
	cookie      config.Session
	behindProxy bool
	log         *slog.Logger
}

func New(cfg *config.Config, authService *auth.Service, schoolService *school.Service, log *slog.Logger) *Base {
	return &Base{
		auth:        authService,
		school:      schoolService,
		schoolName:  cfg.School.Name,
		version:     buildinfo.Version(),
		cookie:      cfg.Session,
		behindProxy: cfg.HTTP.BehindProxy,
		log:         log,
	}
}

func (b *Base) SchoolName() string {
	return b.schoolName
}
