package school

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgPenaltyNameRequired = "Введите название"
	msgPenaltyNameTooLong  = "Название: не больше 30 символов"
	msgPenaltyUnitRequired = "Введите единицу"
	msgPenaltyUnitTooLong  = "Единица: не больше 20 символов"
	msgPenaltyFullRange    = "Полный штраф: целое число от 1 до 1000"

	maxPenaltyNameLength = 30
	maxPenaltyUnitLength = 20
	maxPenaltyFull       = 1000

	settingPenaltyEnabled = "penalty.enabled"
	settingPenaltyName    = "penalty.name"
	settingPenaltyUnit    = "penalty.unit"
	settingPenaltyFull    = "penalty.full"
)

type PenaltySettings struct {
	Enabled bool
	Name    string
	Unit    string
	Full    int
}

type PenaltySettingsInput struct {
	Enabled bool
	Name    string
	Unit    string
	Full    string
}

func DefaultPenaltySettings() PenaltySettings {
	return PenaltySettings{Name: "Штраф", Unit: "баллы", Full: 20}
}

func (s *Service) LoadSettings(ctx context.Context) error {
	values, err := s.settings.All(ctx)
	if err != nil {
		return err
	}

	penalty := DefaultPenaltySettings()

	if value, ok := values[settingPenaltyEnabled]; ok {
		penalty.Enabled = value == "1"
	}

	if value, ok := values[settingPenaltyName]; ok {
		penalty.Name = value
	}

	if value, ok := values[settingPenaltyUnit]; ok {
		penalty.Unit = value
	}

	if value, ok := values[settingPenaltyFull]; ok {
		if penalty.Full, err = strconv.Atoi(value); err != nil {
			return fmt.Errorf("parse %s: %w", settingPenaltyFull, err)
		}
	}

	s.settingsMu.Lock()
	s.penalty = penalty
	s.settingsMu.Unlock()

	return nil
}

func (s *Service) PenaltySettings() PenaltySettings {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()

	return s.penalty
}

func (s *Service) SavePenaltySettings(ctx context.Context, input PenaltySettingsInput) error {
	penalty, err := validatePenaltySettings(input)
	if err != nil {
		return err
	}

	enabled := "0"
	if penalty.Enabled {
		enabled = "1"
	}

	err = s.settings.Set(ctx, map[string]string{
		settingPenaltyEnabled: enabled,
		settingPenaltyName:    penalty.Name,
		settingPenaltyUnit:    penalty.Unit,
		settingPenaltyFull:    strconv.Itoa(penalty.Full),
	})
	if err != nil {
		return err
	}

	s.settingsMu.Lock()
	s.penalty = penalty
	s.settingsMu.Unlock()

	s.log.InfoContext(ctx, "penalty settings saved", slog.Bool("enabled", penalty.Enabled), slog.String("name", penalty.Name),
		slog.String("unit", penalty.Unit), slog.Int("full", penalty.Full))

	return nil
}

func validatePenaltySettings(input PenaltySettingsInput) (PenaltySettings, error) {
	penalty := PenaltySettings{
		Enabled: input.Enabled,
		Name:    validation.NormalizeSpaces(input.Name),
		Unit:    validation.NormalizeSpaces(input.Unit),
	}

	errs := validation.Errors{}

	switch {
	case penalty.Name == "":
		errs.Add("name", msgPenaltyNameRequired)
	case utf8.RuneCountInString(penalty.Name) > maxPenaltyNameLength:
		errs.Add("name", msgPenaltyNameTooLong)
	}

	switch {
	case penalty.Unit == "":
		errs.Add("unit", msgPenaltyUnitRequired)
	case utf8.RuneCountInString(penalty.Unit) > maxPenaltyUnitLength:
		errs.Add("unit", msgPenaltyUnitTooLong)
	}

	full, err := strconv.Atoi(input.Full)
	if err != nil || full < 1 || full > maxPenaltyFull {
		errs.Add("full", msgPenaltyFullRange)
	}

	penalty.Full = full

	if len(errs) > 0 {
		return PenaltySettings{}, errs
	}

	return penalty, nil
}
