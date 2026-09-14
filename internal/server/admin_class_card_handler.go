package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	msgStudentUnknown = "Такого ученика нет"
	msgTeacherUnknown = "Такого учителя нет"
	studentOption     = "Выберите ученика"
	subjectOption     = "Выберите предмет"
	teacherOption     = "Выберите учителя"
)

type classCardErrors struct {
	students    validation.Errors
	assignments validation.Errors
}

func (s *Server) classShow(w http.ResponseWriter, r *http.Request) {
	class, ok := s.pathClass(w, r)
	if !ok {
		return
	}

	s.renderClassCard(w, r, class, classCardErrors{})
}

func (s *Server) classStudentAdd(w http.ResponseWriter, r *http.Request) {
	class, ok := s.pathClass(w, r)
	if !ok {
		return
	}

	err := s.addClassStudent(r, class.ID, formInt64(r, "student_id"))
	if errs, ok := formErrors(err); ok {
		s.renderClassCard(w, r, class, classCardErrors{students: errs})
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "add class student", err)
		return
	}

	s.classStudentsDone(w, r, class)
}

func (s *Server) addClassStudent(r *http.Request, classID, studentID int64) error {
	_, err := s.activeUser(r.Context(), studentID, auth.RoleStudent)
	if errors.Is(err, auth.ErrNotFound) {
		return validation.Errors{"student": msgStudentUnknown}
	}
	if err != nil {
		return err
	}

	return s.school.AddClassStudent(r.Context(), classID, studentID)
}

func (s *Server) classStudentRemove(w http.ResponseWriter, r *http.Request) {
	class, ok := s.pathClass(w, r)
	if !ok {
		return
	}

	studentID, ok := pathValue(r, "sid")
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := s.school.RemoveClassStudent(r.Context(), class.ID, studentID)
	if errs, ok := formErrors(err); ok {
		s.renderClassCard(w, r, class, classCardErrors{students: errs})
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "remove class student", err)
		return
	}

	s.classStudentsDone(w, r, class)
}

func (s *Server) classAssign(w http.ResponseWriter, r *http.Request) {
	class, ok := s.pathClass(w, r)
	if !ok {
		return
	}

	err := s.assignTeacher(r, class.ID, formInt64(r, "subject_id"), formInt64(r, "teacher_id"))
	if errs, ok := formErrors(err); ok {
		s.renderClassCard(w, r, class, classCardErrors{assignments: errs})
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "assign teacher", err)
		return
	}

	s.classAssignmentsDone(w, r, class)
}

func (s *Server) assignTeacher(r *http.Request, classID, subjectID, teacherID int64) error {
	if teacherID != 0 {
		_, err := s.activeUser(r.Context(), teacherID, auth.RoleTeacher)
		if errors.Is(err, auth.ErrNotFound) {
			return validation.Errors{"teacher": msgTeacherUnknown}
		}
		if err != nil {
			return err
		}
	}

	return s.school.AssignTeacher(r.Context(), classID, subjectID, teacherID)
}

func (s *Server) classAssignmentRemove(w http.ResponseWriter, r *http.Request) {
	class, ok := s.pathClass(w, r)
	if !ok {
		return
	}

	assignmentID, ok := pathValue(r, "aid")
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := s.school.RemoveAssignment(r.Context(), class.ID, assignmentID)
	if errs, ok := formErrors(err); ok {
		s.renderClassCard(w, r, class, classCardErrors{assignments: errs})
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "remove assignment", err)
		return
	}

	s.classAssignmentsDone(w, r, class)
}

func (s *Server) pathClass(w http.ResponseWriter, r *http.Request) (school.Class, bool) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return school.Class{}, false
	}

	class, err := s.school.ClassByID(r.Context(), id)
	if err != nil {
		s.handleServiceError(w, r, "load class", err)
		return school.Class{}, false
	}

	return class, true
}

func (s *Server) classStudentsDone(w http.ResponseWriter, r *http.Request, class school.Class) {
	if !isHTMX(r) {
		s.redirect(w, r, classPath(class.ID, ""))
		return
	}

	block, err := s.classStudentsBlock(r, class, nil)
	if err != nil {
		s.serverError(w, r, "load class students", err)
		return
	}

	s.render(w, r, pages.ClassStudents(block))
}

func (s *Server) classAssignmentsDone(w http.ResponseWriter, r *http.Request, class school.Class) {
	if !isHTMX(r) {
		s.redirect(w, r, classPath(class.ID, ""))
		return
	}

	block, err := s.classAssignmentsBlock(r, class, nil)
	if err != nil {
		s.serverError(w, r, "load class assignments", err)
		return
	}

	s.render(w, r, pages.ClassAssignments(block))
}

func (s *Server) renderClassCard(w http.ResponseWriter, r *http.Request, class school.Class, errs classCardErrors) {
	students, err := s.classStudentsBlock(r, class, errs.students)
	if err != nil {
		s.serverError(w, r, "load class students", err)
		return
	}

	if isHTMX(r) && errs.students != nil {
		s.render(w, r, pages.ClassStudents(students))
		return
	}

	assignments, err := s.classAssignmentsBlock(r, class, errs.assignments)
	if err != nil {
		s.serverError(w, r, "load class assignments", err)
		return
	}

	if isHTMX(r) && errs.assignments != nil {
		s.render(w, r, pages.ClassAssignments(assignments))
		return
	}

	page := view.ClassPage{
		Shell:       s.shell(r, class.Name, classesPath),
		ID:          class.ID,
		Name:        class.Name,
		YearName:    school.YearName(class.Year),
		Active:      class.Active,
		Students:    students,
		Assignments: assignments,
	}

	s.render(w, r, pages.Class(page))
}

func (s *Server) classEditable(class school.Class) bool {
	return class.Active && class.Year == s.school.CurrentYear()
}

func (s *Server) classStudentsBlock(r *http.Request, class school.Class, errs validation.Errors) (view.ClassStudentsBlock, error) {
	ctx := r.Context()

	students, err := s.auth.Users(ctx, auth.UserFilter{Role: auth.RoleStudent, IncludeInactive: true})
	if err != nil {
		return view.ClassStudentsBlock{}, err
	}

	classes, err := s.school.StudentClasses(ctx, class.Year)
	if err != nil {
		return view.ClassStudentsBlock{}, err
	}

	block := view.ClassStudentsBlock{
		CanEdit:   s.classEditable(class),
		AddAction: classPath(class.ID, "/students"),
		Error:     firstError(errs, "student", "class"),
	}

	var candidates []view.Option

	for _, student := range students {
		current, member := classes[student.ID]
		if member && current.ID == class.ID {
			block.Students = append(block.Students, view.MemberRow{
				ID:           student.ID,
				FullName:     student.FullName,
				Active:       student.Active,
				RemoveAction: classPath(class.ID, "/students/"+strconv.FormatInt(student.ID, 10)+"/remove"),
			})
		}

		if !member && student.Active {
			candidates = append(candidates, view.Option{ID: student.ID, Name: student.FullName})
		}
	}

	if block.CanEdit && len(candidates) > 0 {
		block.Candidates = append([]view.Option{{Name: studentOption, Selected: true}}, candidates...)
	}

	return block, nil
}

func (s *Server) classAssignmentsBlock(r *http.Request, class school.Class, errs validation.Errors) (view.ClassAssignmentsBlock, error) {
	ctx := r.Context()

	assignments, err := s.school.Assignments(ctx, class.ID)
	if err != nil {
		return view.ClassAssignmentsBlock{}, err
	}

	subjects, err := s.school.Subjects(ctx, true)
	if err != nil {
		return view.ClassAssignmentsBlock{}, err
	}

	teachers, err := s.auth.Users(ctx, auth.UserFilter{Role: auth.RoleTeacher, IncludeInactive: true})
	if err != nil {
		return view.ClassAssignmentsBlock{}, err
	}

	subjectByID := make(map[int64]school.Subject, len(subjects))
	for _, subject := range subjects {
		subjectByID[subject.ID] = subject
	}

	teacherByID := make(map[int64]auth.User, len(teachers))
	for _, teacher := range teachers {
		teacherByID[teacher.ID] = teacher
	}

	block := view.ClassAssignmentsBlock{
		CanEdit:   s.classEditable(class),
		AddAction: classPath(class.ID, "/assignments"),
		Errors:    errs,
	}

	for _, assignment := range assignments {
		subject := subjectByID[assignment.SubjectID]
		teacher := teacherByID[assignment.TeacherID]

		block.Assignments = append(block.Assignments, view.AssignmentRow{
			ID:            assignment.ID,
			Subject:       subject.Name,
			Teacher:       teacher.FullName,
			SubjectActive: subject.Active,
			TeacherActive: teacher.Active,
			RemoveAction:  classPath(class.ID, "/assignments/"+strconv.FormatInt(assignment.ID, 10)+"/remove"),
		})
	}

	if !block.CanEdit {
		return block, nil
	}

	block.Subjects = []view.Option{{Name: subjectOption, Selected: true}}
	for _, subject := range subjects {
		if subject.Active {
			block.Subjects = append(block.Subjects, view.Option{ID: subject.ID, Name: subject.Name})
		}
	}

	block.Teachers = []view.Option{{Name: teacherOption, Selected: true}}
	for _, teacher := range teachers {
		if teacher.Active {
			block.Teachers = append(block.Teachers, view.Option{ID: teacher.ID, Name: teacher.FullName})
		}
	}

	if len(block.Subjects) == 1 || len(block.Teachers) == 1 {
		block.Subjects, block.Teachers = nil, nil
	}

	return block, nil
}

func firstError(errs validation.Errors, keys ...string) string {
	for _, key := range keys {
		if message := errs[key]; message != "" {
			return message
		}
	}

	return ""
}

func classPath(id int64, suffix string) string {
	return classesPath + "/" + strconv.FormatInt(id, 10) + suffix
}
