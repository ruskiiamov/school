package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/storage"
)

const sessionIDBytes = 32

type userRepo interface {
	ByLogin(ctx context.Context, login string) (storage.User, error)
	Create(ctx context.Context, user storage.User) (int64, error)
	UpdatePasswordHash(ctx context.Context, id int64, hash string) error
}

type sessionRepo interface {
	Create(ctx context.Context, session storage.Session) error
	ByID(ctx context.Context, id string) (storage.Session, storage.User, error)
	Delete(ctx context.Context, id string) error
	DeleteByUser(ctx context.Context, userID int64) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
	DeleteOrphaned(ctx context.Context) (int64, error)
}

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

type Service struct {
	users    userRepo
	sessions sessionRepo
	ttl      time.Duration
	log      *slog.Logger
}

func NewService(users userRepo, sessions sessionRepo, ttl time.Duration, log *slog.Logger) *Service {
	return &Service{users: users, sessions: sessions, ttl: ttl, log: log}
}

func (s *Service) Login(ctx context.Context, login, password string) (Session, error) {
	stored, err := s.users.ByLogin(ctx, login)
	if errors.Is(err, storage.ErrNotFound) {
		equalizePasswordTiming(password)
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(password)) != nil {
		return Session{}, ErrInvalidCredentials
	}

	return s.createSession(ctx, stored.ID)
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}

	return s.sessions.Delete(ctx, sessionID)
}

func (s *Service) Authenticate(ctx context.Context, sessionID string) (User, error) {
	if sessionID == "" {
		return User{}, ErrUnauthenticated
	}

	session, stored, err := s.sessions.ByID(ctx, sessionID)
	if errors.Is(err, storage.ErrNotFound) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, err
	}

	if !time.Now().Before(session.ExpiresAt) {
		if err := s.sessions.Delete(ctx, sessionID); err != nil {
			s.log.Error("delete expired session", slog.Any("error", err))
		}

		return User{}, ErrUnauthenticated
	}

	return toUser(stored)
}

func (s *Service) EnsureAdmin(ctx context.Context, cfg config.Admin) error {
	stored, err := s.users.ByLogin(ctx, cfg.Login)
	if errors.Is(err, storage.ErrNotFound) {
		return s.createAdmin(ctx, cfg)
	}
	if err != nil {
		return err
	}

	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(cfg.Password)) == nil {
		return nil
	}

	hash, err := hashPassword(cfg.Password)
	if err != nil {
		return err
	}

	if err := s.users.UpdatePasswordHash(ctx, stored.ID, hash); err != nil {
		return err
	}

	if err := s.sessions.DeleteByUser(ctx, stored.ID); err != nil {
		return err
	}

	s.log.Info("admin password updated from config", slog.String("login", cfg.Login))

	return nil
}

func (s *Service) RunSessionCleanup(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	s.cleanupSessions(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupSessions(ctx)
		}
	}
}

func (s *Service) createAdmin(ctx context.Context, cfg config.Admin) error {
	hash, err := hashPassword(cfg.Password)
	if err != nil {
		return err
	}

	id, err := s.users.Create(ctx, storage.User{
		Login:        cfg.Login,
		PasswordHash: hash,
		FullName:     cfg.FullName,
		Role:         string(RoleAdmin),
	})
	if err != nil {
		return err
	}

	s.log.Info("admin user created", slog.Int64("id", id), slog.String("login", cfg.Login))

	return nil
}

func (s *Service) createSession(ctx context.Context, userID int64) (Session, error) {
	id, err := newSessionID()
	if err != nil {
		return Session{}, err
	}

	now := time.Now()
	session := storage.Session{
		ID:        id,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return Session{}, err
	}

	return Session{ID: session.ID, UserID: userID, ExpiresAt: session.ExpiresAt}, nil
}

func (s *Service) cleanupSessions(ctx context.Context) {
	expired, err := s.sessions.DeleteExpired(ctx, time.Now())
	if err != nil {
		s.log.Error("delete expired sessions", slog.Any("error", err))
		return
	}

	orphaned, err := s.sessions.DeleteOrphaned(ctx)
	if err != nil {
		s.log.Error("delete orphaned sessions", slog.Any("error", err))
		return
	}

	if expired+orphaned > 0 {
		s.log.Info("sessions cleaned up",
			slog.Int64("expired", expired), slog.Int64("orphaned", orphaned))
	}
}

func toUser(stored storage.User) (User, error) {
	role, err := ParseRole(stored.Role)
	if err != nil {
		return User{}, fmt.Errorf("user %d: %w", stored.ID, err)
	}

	return User{
		ID:       stored.ID,
		Login:    stored.Login,
		FullName: stored.FullName,
		Role:     role,
	}, nil
}

func newSessionID() (string, error) {
	buf := make([]byte, sessionIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

var timingEqualizerHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("no such user"), bcrypt.DefaultCost)
	if err != nil {
		return nil
	}

	return hash
})

func equalizePasswordTiming(password string) {
	_ = bcrypt.CompareHashAndPassword(timingEqualizerHash(), []byte(password))
}
