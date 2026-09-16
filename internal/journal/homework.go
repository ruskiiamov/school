package journal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ruskiiamov/school/internal/files"
	"github.com/ruskiiamov/school/internal/storage"
	"github.com/ruskiiamov/school/internal/validation"
)

const (
	msgHomeworkTooLong = "Задание не длиннее 2000 символов"
	msgDueInvalid      = "Неверная дата"
	msgFileRequired    = "Выберите файл"
	msgFileNameTooLong = "Имя файла не длиннее 200 символов"
	msgFileTooLargeFmt = "Файл больше %d МБ"
	msgTooManyFilesFmt = "Не больше %d файлов у одного урока"

	maxHomeworkLength = 2000
	maxFileNameLength = 200
)

type FileLimits struct {
	MaxFileSize  int64
	MaxPerLesson int
}

type Homework struct {
	LessonID int64
	Text     string
	Due      time.Time
	Files    []HomeworkFile
}

func (h Homework) Empty() bool {
	return h.Text == "" && h.Due.IsZero() && len(h.Files) == 0
}

type HomeworkFile struct {
	ID          string
	LessonID    int64
	Name        string
	Size        int64
	ContentType string
}

type HomeworkInput struct {
	Text string
	Due  string
}

func (s *Service) Homework(ctx context.Context, lessonID int64) (Homework, error) {
	homework := Homework{LessonID: lessonID}

	stored, err := s.homework.ByLesson(ctx, lessonID)
	if err == nil {
		homework.Text = stored.Text
		homework.Due = stored.DueDate
	} else if !errors.Is(err, storage.ErrNotFound) {
		return Homework{}, err
	}

	storedFiles, err := s.homeworkFiles.ListByLesson(ctx, lessonID)
	if err != nil {
		return Homework{}, err
	}

	for _, file := range storedFiles {
		homework.Files = append(homework.Files, toHomeworkFile(file))
	}

	return homework, nil
}

func (s *Service) SaveHomework(ctx context.Context, teacherID, lessonID int64, input HomeworkInput) error {
	if _, err := s.LessonForTeacher(ctx, teacherID, lessonID); err != nil {
		return err
	}

	errs := validation.Errors{}

	text := validation.NormalizeLines(input.Text)
	if utf8.RuneCountInString(text) > maxHomeworkLength {
		errs.Add("text", msgHomeworkTooLong)
	}

	var due time.Time

	if input.Due != "" {
		parsed, ok := validation.ParseDate(input.Due)
		if !ok {
			errs.Add("due", msgDueInvalid)
		}

		due = parsed
	}

	if len(errs) > 0 {
		return errs
	}

	if text == "" && due.IsZero() {
		if err := s.homework.Delete(ctx, lessonID); err != nil {
			return err
		}

		s.log.InfoContext(ctx, "homework cleared", slog.Int64("lesson_id", lessonID), slog.Int64("teacher_id", teacherID))

		return nil
	}

	if err := s.homework.Upsert(ctx, storage.Homework{LessonID: lessonID, Text: text, DueDate: due}); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "homework saved", slog.Int64("lesson_id", lessonID), slog.Int64("teacher_id", teacherID))

	return nil
}

func (s *Service) AddHomeworkFile(ctx context.Context, teacherID, lessonID int64, name string, content io.Reader) error {
	if _, err := s.LessonForTeacher(ctx, teacherID, lessonID); err != nil {
		return err
	}

	name = fileName(name)
	if name == "" {
		return validation.Errors{"files": msgFileRequired}
	}

	if utf8.RuneCountInString(name) > maxFileNameLength {
		return validation.Errors{"files": msgFileNameTooLong}
	}

	count, err := s.homeworkFiles.CountByLesson(ctx, lessonID)
	if err != nil {
		return err
	}

	if count >= s.limits.MaxPerLesson {
		return validation.Errors{"files": fmt.Sprintf(msgTooManyFilesFmt, s.limits.MaxPerLesson)}
	}

	saved, err := s.store.Save(content, s.limits.MaxFileSize)
	if errors.Is(err, files.ErrTooLarge) {
		return validation.Errors{"files": fmt.Sprintf(msgFileTooLargeFmt, s.limits.MaxFileSize>>20)}
	}
	if err != nil {
		return err
	}

	err = s.homeworkFiles.Create(ctx, storage.HomeworkFile{
		ID:          saved.ID,
		LessonID:    lessonID,
		Name:        name,
		Size:        saved.Size,
		ContentType: saved.ContentType,
	})
	if err != nil {
		if removeErr := s.store.Delete(saved.ID); removeErr != nil {
			s.log.ErrorContext(ctx, "remove unsaved homework file", slog.String("file_id", saved.ID), slog.Any("error", removeErr))
		}

		return err
	}

	s.log.InfoContext(ctx, "homework file added", slog.Int64("lesson_id", lessonID), slog.Int64("teacher_id", teacherID),
		slog.String("file_id", saved.ID), slog.Int64("size", saved.Size))

	return nil
}

func (s *Service) DeleteHomeworkFile(ctx context.Context, teacherID, lessonID int64, fileID string) error {
	if _, err := s.LessonForTeacher(ctx, teacherID, lessonID); err != nil {
		return err
	}

	file, err := s.homeworkFiles.ByID(ctx, fileID)
	if errors.Is(err, storage.ErrNotFound) || (err == nil && file.LessonID != lessonID) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if err := s.homeworkFiles.Delete(ctx, fileID); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	if err := s.store.Delete(fileID); err != nil && !errors.Is(err, files.ErrNotFound) {
		s.log.ErrorContext(ctx, "remove homework file", slog.String("file_id", fileID), slog.Any("error", err))
	}

	s.log.InfoContext(ctx, "homework file deleted", slog.Int64("lesson_id", lessonID), slog.Int64("teacher_id", teacherID),
		slog.String("file_id", fileID))

	return nil
}

func (s *Service) HomeworkFile(ctx context.Context, fileID string) (HomeworkFile, error) {
	file, err := s.homeworkFiles.ByID(ctx, fileID)
	if errors.Is(err, storage.ErrNotFound) {
		return HomeworkFile{}, ErrNotFound
	}
	if err != nil {
		return HomeworkFile{}, err
	}

	return toHomeworkFile(file), nil
}

func (s *Service) OpenHomeworkFile(ctx context.Context, fileID string) (*os.File, error) {
	file, err := s.store.Open(fileID)
	if errors.Is(err, files.ErrNotFound) {
		s.log.ErrorContext(ctx, "homework file missing on disk", slog.String("file_id", fileID))

		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (s *Service) LessonVisibleToStudent(ctx context.Context, studentID, lessonID int64) (bool, error) {
	return s.lessons.VisibleToStudent(ctx, studentID, lessonID)
}

func fileName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}

	return validation.NormalizeSpaces(name)
}

func toHomeworkFile(file storage.HomeworkFile) HomeworkFile {
	return HomeworkFile{
		ID:          file.ID,
		LessonID:    file.LessonID,
		Name:        file.Name,
		Size:        file.Size,
		ContentType: file.ContentType,
	}
}
