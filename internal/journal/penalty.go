package journal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgDonePercentRange   = "Выполнено: целое число от 0 до 100"
	msgPenaltyLocked      = "Штраф уже погашен, изменить нельзя"
	msgWorkoffFuture      = "Дата не может быть позже сегодняшней"
	msgWorkoffPoints      = "Отработано: целое число больше 0"
	msgWorkoffOverDebt    = "Больше долга: долг %d"
	msgWorkoffCommentLong = "Комментарий: не больше 200 символов"

	fullDonePercent         = 100
	maxWorkoffCommentLength = 200
)

type PenaltyStatus int

const (
	PenaltyNone PenaltyStatus = iota
	PenaltyOpen
	PenaltyPartial
	PenaltyPaid
)

type Penalty struct {
	DonePercent int
	Points      int
	Status      PenaltyStatus
}

func (p Penalty) Locked() bool {
	return p.Status == PenaltyPartial || p.Status == PenaltyPaid
}

func PenaltyPoints(full, donePercent int) int {
	return (full*(fullDonePercent-donePercent) + fullDonePercent/2) / fullDonePercent
}

type HistoryPenalty struct {
	LessonID     int64
	Date         time.Time
	SubjectName  string
	WorkTypeName string
	Penalty
}

type Workoff struct {
	ID         int64
	StudentID  int64
	Date       time.Time
	Points     int
	Comment    string
	RecordedBy int64
}

type PenaltyHistory struct {
	Debt      int
	Penalties []HistoryPenalty
	Workoffs  []Workoff
}

type WorkoffInput struct {
	Date    string
	Points  string
	Comment string
}

type ledger struct {
	statuses map[int64]PenaltyStatus
	debts    map[int64]int
}

func (s *Service) ledger(ctx context.Context, studentIDs []int64) (ledger, []storage.StudentPenalty, error) {
	penalties, err := s.penalties.ListByStudents(ctx, studentIDs)
	if err != nil {
		return ledger{}, nil, err
	}

	worked, err := s.workoffs.Totals(ctx)
	if err != nil {
		return ledger{}, nil, err
	}

	return buildLedger(penalties, worked), penalties, nil
}

func buildLedger(penalties []storage.StudentPenalty, worked map[int64]int) ledger {
	result := ledger{statuses: make(map[int64]PenaltyStatus, len(penalties)), debts: map[int64]int{}}
	charged := map[int64]int{}

	for _, penalty := range penalties {
		before := charged[penalty.StudentID]
		charged[penalty.StudentID] = before + penalty.Points
		covered := min(max(worked[penalty.StudentID]-before, 0), penalty.Points)
		result.statuses[penalty.ID] = penaltyStatus(penalty.Points, covered)
	}

	for studentID, total := range charged {
		result.debts[studentID] = max(total-worked[studentID], 0)
	}

	return result
}

func penaltyStatus(points, covered int) PenaltyStatus {
	switch {
	case points == 0:
		return PenaltyNone
	case covered == points:
		return PenaltyPaid
	case covered > 0:
		return PenaltyPartial
	default:
		return PenaltyOpen
	}
}

func (s *Service) StudentDebt(ctx context.Context, studentID int64) (int, error) {
	ledger, _, err := s.ledger(ctx, []int64{studentID})
	if err != nil {
		return 0, err
	}

	return ledger.debts[studentID], nil
}

func (s *Service) Debts(ctx context.Context) (map[int64]int, error) {
	charged, err := s.penalties.Totals(ctx)
	if err != nil {
		return nil, err
	}

	worked, err := s.workoffs.Totals(ctx)
	if err != nil {
		return nil, err
	}

	debts := map[int64]int{}

	for studentID, total := range charged {
		if debt := total - worked[studentID]; debt > 0 {
			debts[studentID] = debt
		}
	}

	return debts, nil
}

func (s *Service) PenaltyHistory(ctx context.Context, studentID int64) (PenaltyHistory, error) {
	ledger, penalties, err := s.ledger(ctx, []int64{studentID})
	if err != nil {
		return PenaltyHistory{}, err
	}

	subjects, err := s.subjects.List(ctx, true)
	if err != nil {
		return PenaltyHistory{}, err
	}

	subjectNames := make(map[int64]string, len(subjects))
	for _, subject := range subjects {
		subjectNames[subject.ID] = subject.Name
	}

	typeIDs := make([]int64, 0, len(penalties))
	for _, penalty := range penalties {
		typeIDs = append(typeIDs, penalty.WorkTypeID)
	}

	typeNames, _, err := s.workTypeNames(ctx, typeIDs)
	if err != nil {
		return PenaltyHistory{}, err
	}

	history := PenaltyHistory{Debt: ledger.debts[studentID]}

	for _, penalty := range penalties {
		history.Penalties = append(history.Penalties, HistoryPenalty{
			LessonID:     penalty.LessonID,
			Date:         penalty.Date,
			SubjectName:  subjectNames[penalty.SubjectID],
			WorkTypeName: typeNames[penalty.WorkTypeID],
			Penalty:      Penalty{DonePercent: penalty.DonePercent, Points: penalty.Points, Status: ledger.statuses[penalty.ID]},
		})
	}

	// Новые сверху — ровно обратный порядок погашения, иначе внутри одного
	// дня погашенное оказывается над непогашенным (D-115).
	slices.Reverse(history.Penalties)

	workoffs, err := s.workoffs.ListByStudent(ctx, studentID)
	if err != nil {
		return PenaltyHistory{}, err
	}

	for _, workoff := range workoffs {
		history.Workoffs = append(history.Workoffs, toWorkoff(workoff))
	}

	return history, nil
}

func (s *Service) AddWorkoff(ctx context.Context, recordedBy, studentID int64, input WorkoffInput, today time.Time) (int64, error) {
	debt, err := s.StudentDebt(ctx, studentID)
	if err != nil {
		return 0, err
	}

	workoff, err := validateWorkoff(input, debt, today)
	if err != nil {
		return 0, err
	}

	workoff.StudentID = studentID
	workoff.RecordedBy = recordedBy

	id, err := s.workoffs.Create(ctx, workoff)
	if err != nil {
		return 0, err
	}

	s.log.InfoContext(ctx, "workoff recorded", slog.Int64("id", id), slog.Int64("student_id", studentID),
		slog.Int("points", workoff.Points), slog.Int64("recorded_by", recordedBy))

	return id, nil
}

func validateWorkoff(input WorkoffInput, debt int, today time.Time) (storage.Workoff, error) {
	errs := validation.Errors{}
	workoff := storage.Workoff{Comment: validation.NormalizeSpaces(input.Comment)}

	raw := strings.TrimSpace(input.Date)
	date, ok := validation.ParseDate(raw)

	switch {
	case raw == "":
		errs.Add("date", msgDateRequired)
	case !ok:
		errs.Add("date", msgDateInvalid)
	case date.After(today):
		errs.Add("date", msgWorkoffFuture)
	}

	workoff.Date = date

	points, err := strconv.Atoi(strings.TrimSpace(input.Points))

	switch {
	case err != nil || points <= 0:
		errs.Add("points", msgWorkoffPoints)
	case points > debt:
		errs.Add("points", fmt.Sprintf(msgWorkoffOverDebt, debt))
	}

	workoff.Points = points

	if utf8.RuneCountInString(workoff.Comment) > maxWorkoffCommentLength {
		errs.Add("comment", msgWorkoffCommentLong)
	}

	if len(errs) > 0 {
		return storage.Workoff{}, errs
	}

	return workoff, nil
}

func (s *Service) WorkoffByID(ctx context.Context, id int64) (Workoff, error) {
	workoff, err := s.workoffs.ByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return Workoff{}, ErrNotFound
	}
	if err != nil {
		return Workoff{}, err
	}

	return toWorkoff(workoff), nil
}

func (s *Service) DeleteWorkoff(ctx context.Context, id int64) error {
	if err := s.workoffs.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}

		return err
	}

	s.log.InfoContext(ctx, "workoff deleted", slog.Int64("id", id))

	return nil
}

func (s *Service) DeleteOwnWorkoff(ctx context.Context, teacherID, id int64) error {
	workoff, err := s.WorkoffByID(ctx, id)
	if err != nil {
		return err
	}

	if workoff.RecordedBy != teacherID {
		return ErrForbidden
	}

	return s.DeleteWorkoff(ctx, id)
}

func toWorkoff(stored storage.Workoff) Workoff {
	return Workoff{
		ID:         stored.ID,
		StudentID:  stored.StudentID,
		Date:       stored.Date,
		Points:     stored.Points,
		Comment:    stored.Comment,
		RecordedBy: stored.RecordedBy,
	}
}

func parseDonePercent(raw string) (int, bool) {
	value := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "%"))
	if value == "" {
		return fullDonePercent, true
	}

	percent, err := strconv.Atoi(value)
	if err != nil || percent < 0 || percent > fullDonePercent {
		return 0, false
	}

	return percent, true
}

func penaltyChange(studentID, workTypeID int64, percent, full int) storage.PenaltyChange {
	if percent == fullDonePercent {
		return storage.PenaltyChange{StudentID: studentID, WorkTypeID: workTypeID, Remove: true}
	}

	return storage.PenaltyChange{
		StudentID:   studentID,
		WorkTypeID:  workTypeID,
		DonePercent: percent,
		Points:      PenaltyPoints(full, percent),
	}
}

func keyedPenalties(stored []storage.Penalty, ledger ledger) map[MarkKey]Penalty {
	penalties := make(map[MarkKey]Penalty, len(stored))

	for _, penalty := range stored {
		key := MarkKey{WorkTypeID: penalty.WorkTypeID, StudentID: penalty.StudentID}
		penalties[key] = Penalty{DonePercent: penalty.DonePercent, Points: penalty.Points, Status: ledger.statuses[penalty.ID]}
	}

	return penalties
}

func (s *Service) lessonPenalties(ctx context.Context, lessonID int64) (map[MarkKey]Penalty, map[int64]int, error) {
	stored, err := s.penalties.ListByLesson(ctx, lessonID)
	if err != nil {
		return nil, nil, err
	}

	students := make([]int64, 0, len(stored))
	for _, penalty := range stored {
		students = append(students, penalty.StudentID)
	}

	ledger, _, err := s.ledger(ctx, uniqueIDs(students))
	if err != nil {
		return nil, nil, err
	}

	return keyedPenalties(stored, ledger), ledger.debts, nil
}

func donePenaltyChange(current *Penalty, allowed bool, studentID, workTypeID int64, raw string, full int) (storage.PenaltyChange, bool, string, error) {
	percent, ok := parseDonePercent(raw)
	if !ok {
		return storage.PenaltyChange{}, false, msgDonePercentRange, nil
	}

	currentPercent := fullDonePercent
	if current != nil {
		currentPercent = current.DonePercent
	}

	switch {
	case percent == currentPercent:
		return storage.PenaltyChange{}, false, "", nil
	case current != nil && current.Locked():
		return storage.PenaltyChange{}, false, msgPenaltyLocked, nil
	case current == nil && !allowed:
		return storage.PenaltyChange{}, false, "", ErrForbidden
	}

	return penaltyChange(studentID, workTypeID, percent, full), true, "", nil
}

func (s *Service) savePenalties(ctx context.Context, teacherID, lessonID int64, changes []storage.PenaltyChange) error {
	if len(changes) == 0 {
		return nil
	}

	if err := s.penalties.Set(ctx, lessonID, changes); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "penalties saved", slog.Int64("lesson_id", lessonID), slog.Int("changes", len(changes)),
		slog.Int64("teacher_id", teacherID))

	return nil
}
