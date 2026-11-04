package journal

import (
	"context"
	"strings"

	"github.com/ruskiiamov/school/internal/storage"
)

const (
	msgValueRange         = "Оценка: цифра от 1 до 5, можно с + или −, или 0"
	msgMarkCommentTooLong = "Комментарий к оценке не длиннее 200 символов"
	maxMarkCommentLength  = 200
)

type Mark struct {
	ID           int64
	StudentID    int64
	WorkTypeID   int64
	WorkTypeName string
	Value        string
	Comment      string
}

func (s *Service) Marks(ctx context.Context, lessonID int64) ([]Mark, error) {
	stored, err := s.marks.ListByLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	names, _, err := s.workTypeNames(ctx, markTypeIDs(stored))
	if err != nil {
		return nil, err
	}

	marks := make([]Mark, 0, len(stored))
	for _, mark := range stored {
		marks = append(marks, Mark{
			ID:           mark.ID,
			StudentID:    mark.StudentID,
			WorkTypeID:   mark.WorkTypeID,
			WorkTypeName: names[mark.WorkTypeID],
			Value:        mark.Value,
			Comment:      mark.Comment,
		})
	}

	return marks, nil
}

func markTypeIDs(marks []storage.Mark) []int64 {
	ids := make([]int64, 0, len(marks))
	for _, mark := range marks {
		ids = append(ids, mark.WorkTypeID)
	}

	return ids
}

func parseMarkValue(raw string) (string, bool) {
	value := strings.ReplaceAll(strings.TrimSpace(raw), "−", "-")

	switch {
	case value == "" || value == "0":
		return value, true
	case len(value) > 2 || value[0] < '1' || value[0] > '5':
		return "", false
	case len(value) == 2 && value[1] != '+' && value[1] != '-':
		return "", false
	default:
		return value, true
	}
}

// Graded — входит ли оценка в среднее и число оценок. Пустое значение —
// комментарий без оценки (D-116): строка есть, баллов нет.
func Graded(value string) bool {
	return value != ""
}

func MarkPoints(value string) float64 {
	if value == "" {
		return 0
	}

	points := float64(value[0] - '0')

	switch {
	case strings.HasSuffix(value, "+"):
		points += 0.5
	case strings.HasSuffix(value, "-"):
		points -= 0.5
	}

	return points
}
