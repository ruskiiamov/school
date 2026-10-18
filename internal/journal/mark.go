package journal

import "context"

const msgValueRange = "Оценка от 1 до 5"

type Mark struct {
	ID           int64
	StudentID    int64
	WorkTypeID   int64
	WorkTypeName string
	Value        int
}

func (s *Service) Marks(ctx context.Context, lessonID int64) ([]Mark, error) {
	stored, err := s.marks.ListByLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	workTypes, err := s.workTypes.List(ctx, true)
	if err != nil {
		return nil, err
	}

	names := make(map[int64]string, len(workTypes))
	for _, workType := range workTypes {
		names[workType.ID] = workType.Name
	}

	marks := make([]Mark, 0, len(stored))
	for _, mark := range stored {
		marks = append(marks, Mark{
			ID:           mark.ID,
			StudentID:    mark.StudentID,
			WorkTypeID:   mark.WorkTypeID,
			WorkTypeName: names[mark.WorkTypeID],
			Value:        mark.Value,
		})
	}

	return marks, nil
}
