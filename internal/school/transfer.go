package school

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	graduatingNumber = 11

	msgTransferEmpty   = "Выберите хотя бы один класс"
	msgTransferStudent = "Ученик не может быть переведён в этот класс"
)

type TransferPlan struct {
	FromYear int
	ToYear   int
	Classes  []TransferClass
}

type TransferClass struct {
	Class      Class
	NewName    string
	Graduating bool
	Exists     bool
	Students   []TransferStudent
}

type TransferStudent struct {
	ID           int64
	Active       bool
	CurrentClass string
}

func (c TransferClass) Eligible(studentID int64) bool {
	for _, student := range c.Students {
		if student.ID == studentID {
			return student.Active && student.CurrentClass == ""
		}
	}

	return false
}

type TransferInput struct {
	Classes []TransferClassInput
}

type TransferClassInput struct {
	ClassID    int64
	Transfer   bool
	Name       string
	StudentIDs []int64
}

func (s *Service) CanTransfer(ctx context.Context) (bool, error) {
	sources, err := s.transferSources(ctx)
	if err != nil {
		return false, err
	}

	return len(sources) > 0, nil
}

func (s *Service) TransferPlan(ctx context.Context) (TransferPlan, error) {
	year := s.CurrentYear()
	plan := TransferPlan{FromYear: year - 1, ToYear: year}

	sources, err := s.transferSources(ctx)
	if err != nil {
		return TransferPlan{}, err
	}

	if len(sources) == 0 {
		return plan, nil
	}

	existing, err := s.classes.ListByYear(ctx, year, true)
	if err != nil {
		return TransferPlan{}, err
	}

	names := make(map[string]bool, len(existing))
	for _, class := range existing {
		names[class.Name] = true
	}

	current, err := s.StudentClasses(ctx, year)
	if err != nil {
		return TransferPlan{}, err
	}

	for _, source := range sources {
		members, err := s.classStudents.Members(ctx, source.ID)
		if err != nil {
			return TransferPlan{}, err
		}

		item := TransferClass{
			Class:      toClass(source),
			NewName:    NextClassName(source.Name),
			Graduating: leadingNumber(source.Name) == graduatingNumber,
		}
		item.Exists = names[item.NewName]

		for _, member := range members {
			item.Students = append(item.Students, TransferStudent{
				ID:           member.StudentID,
				Active:       member.Active,
				CurrentClass: current[member.StudentID].Name,
			})
		}

		plan.Classes = append(plan.Classes, item)
	}

	return plan, nil
}

func (s *Service) Transfer(ctx context.Context, input TransferInput) error {
	plan, err := s.TransferPlan(ctx)
	if err != nil {
		return err
	}

	planned := make(map[int64]TransferClass, len(plan.Classes))
	for _, item := range plan.Classes {
		planned[item.Class.ID] = item
	}

	errs := validation.Errors{}
	names := map[string]bool{}

	var created []storage.NewClass

	for _, item := range input.Classes {
		source, ok := planned[item.ClassID]
		if !ok {
			return ErrNotFound
		}

		if !item.Transfer {
			continue
		}

		key := strconv.FormatInt(item.ClassID, 10)

		name := validation.NormalizeSpaces(item.Name)
		validateName(errs, name)

		if message, found := errs["name"]; found {
			delete(errs, "name")
			errs.Add("name-"+key, message)

			continue
		}

		if names[name] {
			errs.Add("name-"+key, msgClassNameTaken)
			continue
		}

		exists, err := s.classes.NameExists(ctx, plan.ToYear, name, 0)
		if err != nil {
			return err
		}

		if exists {
			errs.Add("name-"+key, msgClassNameTaken)
			continue
		}

		names[name] = true

		class := storage.NewClass{Name: name}

		for _, studentID := range item.StudentIDs {
			if !source.Eligible(studentID) {
				errs.Add("students-"+key, msgTransferStudent)
				break
			}

			class.StudentIDs = append(class.StudentIDs, studentID)
		}

		created = append(created, class)
	}

	if len(errs) > 0 {
		return errs
	}

	if len(created) == 0 {
		return validation.Errors{"form": msgTransferEmpty}
	}

	if err := s.classes.Transfer(ctx, plan.ToYear, created); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "classes transferred", slog.Int("year", plan.ToYear), slog.Int("created", len(created)))

	return nil
}

func (s *Service) transferSources(ctx context.Context) ([]storage.Class, error) {
	return s.classes.ListByYear(ctx, s.CurrentYear()-1, false)
}

func NextClassName(name string) string {
	digits := leadingDigits(name)
	if digits == "" {
		return name
	}

	number, err := strconv.Atoi(digits)
	if err != nil {
		return name
	}

	return strconv.Itoa(number+1) + strings.TrimPrefix(name, digits)
}

func leadingNumber(name string) int {
	number, err := strconv.Atoi(leadingDigits(name))
	if err != nil {
		return 0
	}

	return number
}

func leadingDigits(name string) string {
	end := 0
	for end < len(name) && name[end] >= '0' && name[end] <= '9' {
		end++
	}

	return name[:end]
}
