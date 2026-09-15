package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
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

func (h *Handler) classShow(w http.ResponseWriter, r *http.Request) {
	class, ok := h.pathClass(w, r)
	if !ok {
		return
	}

	h.renderClassCard(w, r, class, classCardErrors{})
}

func (h *Handler) classStudentAdd(w http.ResponseWriter, r *http.Request) {
	class, ok := h.pathClass(w, r)
	if !ok {
		return
	}

	err := h.addClassStudent(r, class.ID, web.FormInt64(r, "student_id"))
	if errs, ok := web.FormErrors(err); ok {
		h.renderClassCard(w, r, class, classCardErrors{students: errs})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "add class student", err)
		return
	}

	h.classStudentsDone(w, r, class)
}

func (h *Handler) addClassStudent(r *http.Request, classID, studentID int64) error {
	_, err := h.activeUser(r.Context(), studentID, auth.RoleStudent)
	if errors.Is(err, auth.ErrNotFound) {
		return validation.Errors{"student": msgStudentUnknown}
	}
	if err != nil {
		return err
	}

	return h.school.AddClassStudent(r.Context(), classID, studentID)
}

func (h *Handler) classStudentRemove(w http.ResponseWriter, r *http.Request) {
	class, ok := h.pathClass(w, r)
	if !ok {
		return
	}

	studentID, ok := web.PathValue(r, "sid")
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := h.school.RemoveClassStudent(r.Context(), class.ID, studentID)
	if errs, ok := web.FormErrors(err); ok {
		h.renderClassCard(w, r, class, classCardErrors{students: errs})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "remove class student", err)
		return
	}

	h.classStudentsDone(w, r, class)
}

func (h *Handler) classAssign(w http.ResponseWriter, r *http.Request) {
	class, ok := h.pathClass(w, r)
	if !ok {
		return
	}

	err := h.assignTeacher(r, class.ID, web.FormInt64(r, "subject_id"), web.FormInt64(r, "teacher_id"))
	if errs, ok := web.FormErrors(err); ok {
		h.renderClassCard(w, r, class, classCardErrors{assignments: errs})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "assign teacher", err)
		return
	}

	h.classAssignmentsDone(w, r, class)
}

func (h *Handler) assignTeacher(r *http.Request, classID, subjectID, teacherID int64) error {
	if teacherID != 0 {
		_, err := h.activeUser(r.Context(), teacherID, auth.RoleTeacher)
		if errors.Is(err, auth.ErrNotFound) {
			return validation.Errors{"teacher": msgTeacherUnknown}
		}
		if err != nil {
			return err
		}
	}

	return h.school.AssignTeacher(r.Context(), classID, subjectID, teacherID)
}

func (h *Handler) classAssignmentRemove(w http.ResponseWriter, r *http.Request) {
	class, ok := h.pathClass(w, r)
	if !ok {
		return
	}

	assignmentID, ok := web.PathValue(r, "aid")
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := h.school.RemoveAssignment(r.Context(), class.ID, assignmentID)
	if errs, ok := web.FormErrors(err); ok {
		h.renderClassCard(w, r, class, classCardErrors{assignments: errs})
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "remove assignment", err)
		return
	}

	h.classAssignmentsDone(w, r, class)
}

func (h *Handler) pathClass(w http.ResponseWriter, r *http.Request) (school.Class, bool) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return school.Class{}, false
	}

	class, err := h.school.ClassByID(r.Context(), id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load class", err)
		return school.Class{}, false
	}

	return class, true
}

func (h *Handler) classStudentsDone(w http.ResponseWriter, r *http.Request, class school.Class) {
	if !web.IsHTMX(r) {
		web.Redirect(w, r, classPath(class.ID, ""))
		return
	}

	block, err := h.classStudentsBlock(r, class, nil)
	if err != nil {
		h.base.ServerError(w, r, "load class students", err)
		return
	}

	h.base.Render(w, r, pages.ClassStudents(block))
}

func (h *Handler) classAssignmentsDone(w http.ResponseWriter, r *http.Request, class school.Class) {
	if !web.IsHTMX(r) {
		web.Redirect(w, r, classPath(class.ID, ""))
		return
	}

	block, err := h.classAssignmentsBlock(r, class, nil)
	if err != nil {
		h.base.ServerError(w, r, "load class assignments", err)
		return
	}

	h.base.Render(w, r, pages.ClassAssignments(block))
}

func (h *Handler) renderClassCard(w http.ResponseWriter, r *http.Request, class school.Class, errs classCardErrors) {
	students, err := h.classStudentsBlock(r, class, errs.students)
	if err != nil {
		h.base.ServerError(w, r, "load class students", err)
		return
	}

	if web.IsHTMX(r) && errs.students != nil {
		h.base.Render(w, r, pages.ClassStudents(students))
		return
	}

	assignments, err := h.classAssignmentsBlock(r, class, errs.assignments)
	if err != nil {
		h.base.ServerError(w, r, "load class assignments", err)
		return
	}

	if web.IsHTMX(r) && errs.assignments != nil {
		h.base.Render(w, r, pages.ClassAssignments(assignments))
		return
	}

	page := view.ClassPage{
		Shell:       h.base.Shell(r, class.Name, classesPath),
		ID:          class.ID,
		Name:        class.Name,
		YearName:    school.YearName(class.Year),
		Active:      class.Active,
		Students:    students,
		Assignments: assignments,
	}

	h.base.Render(w, r, pages.Class(page))
}

func (h *Handler) classEditable(class school.Class) bool {
	return class.Active && class.Year == h.school.CurrentYear()
}

func (h *Handler) classStudentsBlock(r *http.Request, class school.Class, errs validation.Errors) (view.ClassStudentsBlock, error) {
	ctx := r.Context()

	students, err := h.auth.Users(ctx, auth.UserFilter{Role: auth.RoleStudent, IncludeInactive: true})
	if err != nil {
		return view.ClassStudentsBlock{}, err
	}

	classes, err := h.school.StudentClasses(ctx, class.Year)
	if err != nil {
		return view.ClassStudentsBlock{}, err
	}

	block := view.ClassStudentsBlock{
		CanEdit:   h.classEditable(class),
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

func (h *Handler) classAssignmentsBlock(r *http.Request, class school.Class, errs validation.Errors) (view.ClassAssignmentsBlock, error) {
	ctx := r.Context()

	assignments, err := h.school.Assignments(ctx, class.ID)
	if err != nil {
		return view.ClassAssignmentsBlock{}, err
	}

	subjects, err := h.school.Subjects(ctx, true)
	if err != nil {
		return view.ClassAssignmentsBlock{}, err
	}

	teachers, err := h.auth.Users(ctx, auth.UserFilter{Role: auth.RoleTeacher, IncludeInactive: true})
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
		CanEdit:   h.classEditable(class),
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
