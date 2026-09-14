package auth

import (
	"context"
	"errors"
	"log/slog"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	minPasswordLength = 8
	maxPasswordBytes  = 72

	msgCurrentRequired = "Укажите текущий пароль"
	msgCurrentWrong    = "Неверный текущий пароль"
	msgNewRequired     = "Укажите новый пароль"
	msgNewTooShort     = "Пароль короче 8 символов"
	msgNewTooLong      = "Слишком длинный пароль"
	msgRepeatMismatch  = "Пароли не совпадают"
)

type PasswordChange struct {
	Current string
	New     string
	Repeat  string
}

func (s *Service) ChangePassword(ctx context.Context, id int64, keepSessionID string, input PasswordChange) error {
	stored, err := s.users.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	errs := validation.Errors{}

	switch {
	case input.Current == "":
		errs.Add("current", msgCurrentRequired)
	case bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(input.Current)) != nil:
		errs.Add("current", msgCurrentWrong)
	}

	validateNewPassword(errs, input.New)

	if input.Repeat != input.New {
		errs.Add("repeat", msgRepeatMismatch)
	}

	if len(errs) > 0 {
		return errs
	}

	hash, err := hashPassword(input.New)
	if err != nil {
		return err
	}

	if err := s.users.UpdatePasswordHash(ctx, id, hash); err != nil {
		return err
	}

	if err := s.sessions.DeleteByUserExcept(ctx, id, keepSessionID); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "user changed own password", slog.Int64("id", id))

	return nil
}

func validateNewPassword(errs validation.Errors, password string) {
	switch {
	case password == "":
		errs.Add("new", msgNewRequired)
	case utf8.RuneCountInString(password) < minPasswordLength:
		errs.Add("new", msgNewTooShort)
	case len(password) > maxPasswordBytes:
		errs.Add("new", msgNewTooLong)
	}
}
