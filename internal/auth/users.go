package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	maxFullNameLength = 100

	msgFullNameRequired = "Укажите ФИО"
	msgFullNameTooLong  = "ФИО длиннее 100 символов"
	msgLoginRequired    = "Укажите логин"
	msgLoginInvalid     = "Только латинские буквы, цифры, точка, дефис и подчёркивание, не длиннее 50 символов"
	msgLoginTaken       = "Такой логин уже есть"
)

type UserInput struct {
	FullName string
	Login    string
}

type NewUser struct {
	Role     Role
	FullName string
}

type Credentials struct {
	User     User
	Password string
}

type UserFilter struct {
	Role            Role
	Query           string
	IncludeInactive bool
}

func (s *Service) CreateUser(ctx context.Context, input NewUser) (Credentials, error) {
	errs := validation.Errors{}

	fullName := validateFullName(errs, input.FullName)
	if len(errs) > 0 {
		return Credentials{}, errs
	}

	login, err := s.freeLogin(ctx, SuggestLogin(fullName))
	if err != nil {
		return Credentials{}, err
	}

	password, err := generatePassword()
	if err != nil {
		return Credentials{}, err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return Credentials{}, err
	}

	id, err := s.users.Create(ctx, storage.User{
		Login:        login,
		PasswordHash: hash,
		FullName:     fullName,
		Role:         string(input.Role),
		Active:       true,
	})
	if err != nil {
		return Credentials{}, err
	}

	s.log.InfoContext(ctx, "user created",
		slog.Int64("id", id), slog.String("login", login), slog.String("role", string(input.Role)))

	user := User{ID: id, Login: login, FullName: fullName, Role: input.Role, Active: true}

	return Credentials{User: user, Password: password}, nil
}

func (s *Service) UpdateUser(ctx context.Context, id int64, input UserInput) error {
	errs := validation.Errors{}

	fullName := validateFullName(errs, input.FullName)

	login := normalizeLogin(input.Login)
	if login == "" {
		errs.Add("login", msgLoginRequired)
	} else if err := s.validateLogin(ctx, errs, login, id); err != nil {
		return err
	}

	if len(errs) > 0 {
		return errs
	}

	if err := s.users.Update(ctx, id, login, fullName); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	s.log.InfoContext(ctx, "user updated", slog.Int64("id", id), slog.String("login", login))

	return nil
}

func (s *Service) ResetPassword(ctx context.Context, id int64) (string, error) {
	if _, err := s.UserByID(ctx, id); err != nil {
		return "", err
	}

	password, err := generatePassword()
	if err != nil {
		return "", err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}

	if err := s.users.UpdatePasswordHash(ctx, id, hash); err != nil {
		return "", err
	}

	if err := s.sessions.DeleteByUser(ctx, id); err != nil {
		return "", err
	}

	s.log.InfoContext(ctx, "user password reset", slog.Int64("id", id))

	return password, nil
}

func (s *Service) SetUserActive(ctx context.Context, id int64, active bool) error {
	if err := s.users.SetActive(ctx, id, active); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	if !active {
		if err := s.sessions.DeleteByUser(ctx, id); err != nil {
			return err
		}
	}

	s.log.InfoContext(ctx, "user active flag changed", slog.Int64("id", id), slog.Bool("active", active))

	return nil
}

func (s *Service) UserByID(ctx context.Context, id int64) (User, error) {
	stored, err := s.users.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	return toUser(stored)
}

func (s *Service) Users(ctx context.Context, filter UserFilter) ([]User, error) {
	stored, err := s.users.ListByRole(ctx, string(filter.Role), filter.IncludeInactive)
	if err != nil {
		return nil, err
	}

	query := strings.ToLower(strings.TrimSpace(filter.Query))

	users := make([]User, 0, len(stored))
	for _, row := range stored {
		if query != "" && !strings.Contains(strings.ToLower(row.FullName), query) {
			continue
		}

		user, err := toUser(row)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	return users, nil
}

func (s *Service) freeLogin(ctx context.Context, base string) (string, error) {
	taken, err := s.users.LoginsWithPrefix(ctx, base)
	if err != nil {
		return "", err
	}

	used := make(map[string]struct{}, len(taken))
	for _, login := range taken {
		used[login] = struct{}{}
	}

	if _, ok := used[base]; !ok {
		return base, nil
	}

	for n := 2; ; n++ {
		candidate := base + strconv.Itoa(n)
		if _, ok := used[candidate]; !ok {
			return candidate, nil
		}
	}
}

func (s *Service) validateLogin(ctx context.Context, errs validation.Errors, login string, excludeID int64) error {
	if !validLogin(login) {
		errs.Add("login", msgLoginInvalid)
		return nil
	}

	exists, err := s.users.LoginExists(ctx, login, excludeID)
	if err != nil {
		return fmt.Errorf("check login: %w", err)
	}

	if exists {
		errs.Add("login", msgLoginTaken)
	}

	return nil
}

func validateFullName(errs validation.Errors, raw string) string {
	fullName := validation.NormalizeSpaces(raw)

	switch {
	case fullName == "":
		errs.Add("full_name", msgFullNameRequired)
	case utf8.RuneCountInString(fullName) > maxFullNameLength:
		errs.Add("full_name", msgFullNameTooLong)
	}

	return fullName
}
