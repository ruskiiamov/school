package web

import (
	"log/slog"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/config"
)

type Base struct {
	auth        *auth.Service
	schoolName  string
	cookie      config.Session
	behindProxy bool
	log         *slog.Logger
}

func New(cfg *config.Config, authService *auth.Service, log *slog.Logger) *Base {
	return &Base{
		auth:        authService,
		schoolName:  cfg.School.Name,
		cookie:      cfg.Session,
		behindProxy: cfg.HTTP.BehindProxy,
		log:         log,
	}
}

func (b *Base) SchoolName() string {
	return b.schoolName
}
