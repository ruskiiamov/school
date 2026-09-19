package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ruskiiamov/school/internal/storage"
)

const (
	DeviceTTL = 365 * 24 * time.Hour

	deviceTokenBytes  = 32
	maxDevicesPerUser = 10
)

type LoginAttempt struct {
	Login       string
	Password    string
	DeviceToken string
	IP          string
}

type LoginResult struct {
	Session     Session
	KnownDevice bool
}

func (s *Service) LoginFrom(ctx context.Context, attempt LoginAttempt) (LoginResult, error) {
	stored, err := s.users.ByLogin(ctx, attempt.Login)
	found := err == nil

	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return LoginResult{}, err
	}

	device := ""

	if found && attempt.DeviceToken != "" {
		hash := deviceHash(attempt.DeviceToken)

		known, err := s.devices.Exists(ctx, hash, stored.ID)
		if err != nil {
			return LoginResult{}, err
		}

		if known {
			device = hash
		}
	}

	if !s.allowAttempt(attempt, device) {
		return LoginResult{}, ErrTooManyAttempts
	}

	if !found {
		equalizePasswordTiming(attempt.Password)
		return LoginResult{}, ErrInvalidCredentials
	}

	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(attempt.Password)) != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	if !stored.Active {
		return LoginResult{}, ErrInvalidCredentials
	}

	s.forgetFailures(ctx, attempt, device)

	session, err := s.createSession(ctx, stored.ID)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{Session: session, KnownDevice: device != ""}, nil
}

func (s *Service) RememberDevice(ctx context.Context, userID int64) (string, error) {
	buf := make([]byte, deviceTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate device token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(buf)

	if err := s.devices.Create(ctx, deviceHash(token), userID, maxDevicesPerUser); err != nil {
		return "", err
	}

	return token, nil
}

func (s *Service) allowAttempt(attempt LoginAttempt, device string) bool {
	if device != "" {
		return s.deviceAttempts.allow(device)
	}

	if attempt.IP != "" && !s.ipAttempts.allow(attempt.IP) {
		return false
	}

	return s.loginAttempts.allow(attempt.Login)
}

func (s *Service) forgetFailures(ctx context.Context, attempt LoginAttempt, device string) {
	if device == "" {
		s.loginAttempts.reset(attempt.Login)
		s.ipAttempts.refund(attempt.IP)

		return
	}

	s.deviceAttempts.reset(device)

	if err := s.devices.Touch(ctx, device); err != nil {
		s.log.ErrorContext(ctx, "touch login device", slog.Any("error", err))
	}
}

func (s *Service) cleanupDevices(ctx context.Context) {
	unused, err := s.devices.DeleteUnusedSince(ctx, time.Now().Add(-DeviceTTL))
	if err != nil {
		s.log.ErrorContext(ctx, "delete unused login devices", slog.Any("error", err))
		return
	}

	orphaned, err := s.devices.DeleteOrphaned(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "delete orphaned login devices", slog.Any("error", err))
		return
	}

	if unused+orphaned > 0 {
		s.log.InfoContext(ctx, "login devices cleaned up",
			slog.Int64("unused", unused), slog.Int64("orphaned", orphaned))
	}
}

func deviceHash(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
