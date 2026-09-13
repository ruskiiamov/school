package server

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	allClassesOption = "Все классы"
	noClassOption    = "Без класса"
	nothingFound     = "Ничего не найдено"
	changedTitle     = "Пароль изменён"
)

type userSection struct {
	role      auth.Role
	path      string
	title     string
	created   string
	recipient string
	empty     string
	more      string
}

var userSections = []userSection{
	{
		role: auth.RoleTeacher, path: "/admin/teachers", title: "Учителя",
		created: "Учитель создан", recipient: "учителю", empty: "Пока нет учителей", more: "Ещё учителя",
	},
	{
		role: auth.RoleStudent, path: "/admin/students", title: "Ученики",
		created: "Ученик создан", recipient: "ученику", empty: "Пока нет учеников", more: "Ещё ученика",
	},
	{
		role: auth.RoleParent, path: "/admin/parents", title: "Родители",
		created: "Родитель создан", recipient: "родителю", empty: "Пока нет родителей", more: "Ещё родителя",
	},
}

func (sec userSection) hasClass() bool {
	return sec.role == auth.RoleStudent
}

func (sec userSection) userPath(id int64, suffix string) string {
	return sec.path + "/" + strconv.FormatInt(id, 10) + suffix
}

type userForm struct {
	fullName string
	login    string
	classID  int64
}

type userEdit struct {
	id      int64
	form    userForm
	entered bool
	errs    validation.Errors
}

func readUserForm(r *http.Request) userForm {
	return userForm{
		fullName: formValue(r, "full_name"),
		login:    formValue(r, "login"),
		classID:  formInt64(r, "class"),
	}
}

func (s *Server) usersList(sec userSection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.renderUsers(w, r, sec, userForm{}, nil, userEdit{})
	}
}

func (s *Server) userCreate(sec userSection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		form := readUserForm(r)

		if sec.hasClass() {
			err := s.school.CheckStudentClass(r.Context(), form.classID)
			if errs, ok := formErrors(err); ok {
				s.renderUsers(w, r, sec, form, errs, userEdit{})
				return
			}
			if err != nil {
				s.serverError(w, r, "check student class", err)
				return
			}
		}

		credentials, err := s.auth.CreateUser(r.Context(), auth.NewUser{Role: sec.role, FullName: form.fullName})
		if errs, ok := formErrors(err); ok {
			s.renderUsers(w, r, sec, form, errs, userEdit{})
			return
		}
		if err != nil {
			s.serverError(w, r, "create user", err)
			return
		}

		if sec.hasClass() && form.classID != 0 {
			if err := s.school.SetStudentClass(r.Context(), credentials.User.ID, form.classID); err != nil {
				s.serverError(w, r, "set student class", err)
				return
			}
		}

		s.created.put(s.sessionID(r), credentialsEntry{
			userID:   credentials.User.ID,
			kind:     credentialsCreated,
			login:    credentials.User.Login,
			password: credentials.Password,
		})

		s.redirect(w, r, sec.userPath(credentials.User.ID, "/created"))
	}
}

func (s *Server) userCreated(sec userSection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.sectionUser(w, r, sec)
		if !ok {
			return
		}

		entry, ok := s.created.take(s.sessionID(r), user.ID)
		if !ok {
			s.redirect(w, r, sec.path)
			return
		}

		page := view.UserCreatedPage{
			Title:    sec.created,
			FullName: user.FullName,
			Login:    entry.login,
			Password: entry.password,
			Note:     "Пароль показан один раз. Передайте данные " + sec.recipient + ".",
			ListHref: sec.path,
			NewHref:  sec.path,
			NewTitle: sec.more,
		}

		if entry.kind == credentialsPasswordChanged {
			page.Title = changedTitle
			page.Note = "Сессии пользователя сброшены. Новый пароль показан один раз, передайте его " + sec.recipient + "."
			page.NewHref = ""
		}

		page.Shell = s.shell(r, page.Title, sec.path)

		s.render(w, r, pages.UserCreated(page))
	}
}

func (s *Server) userUpdate(sec userSection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.sectionUser(w, r, sec)
		if !ok {
			return
		}

		form := readUserForm(r)
		edit := userEdit{id: user.ID, form: form, entered: true}

		if sec.hasClass() {
			err := s.school.CheckStudentClass(r.Context(), form.classID)
			if errs, ok := formErrors(err); ok {
				edit.errs = errs
				s.renderUsers(w, r, sec, userForm{}, nil, edit)

				return
			}
			if err != nil {
				s.serverError(w, r, "check student class", err)
				return
			}
		}

		err := s.auth.UpdateUser(r.Context(), user.ID, auth.UserInput{FullName: form.fullName, Login: form.login})
		if errs, ok := formErrors(err); ok {
			edit.errs = errs
			s.renderUsers(w, r, sec, userForm{}, nil, edit)

			return
		}
		if err != nil {
			s.handleServiceError(w, r, "update user", err)
			return
		}

		if sec.hasClass() {
			if err := s.school.SetStudentClass(r.Context(), user.ID, form.classID); err != nil {
				s.serverError(w, r, "set student class", err)
				return
			}
		}

		s.usersDone(w, r, sec)
	}
}

func (s *Server) userSetPassword(sec userSection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.sectionUser(w, r, sec)
		if !ok {
			return
		}

		password, err := s.auth.ResetPassword(r.Context(), user.ID)
		if err != nil {
			s.handleServiceError(w, r, "reset user password", err)
			return
		}

		s.created.put(s.sessionID(r), credentialsEntry{
			userID:   user.ID,
			kind:     credentialsPasswordChanged,
			login:    user.Login,
			password: password,
		})

		s.redirect(w, r, sec.userPath(user.ID, "/created"))
	}
}

func (s *Server) userSetActive(sec userSection, active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.sectionUser(w, r, sec)
		if !ok {
			return
		}

		if err := s.auth.SetUserActive(r.Context(), user.ID, active); err != nil {
			s.handleServiceError(w, r, "set user active", err)
			return
		}

		s.usersDone(w, r, sec)
	}
}

func (s *Server) usersDone(w http.ResponseWriter, r *http.Request, sec userSection) {
	if isHTMX(r) {
		s.renderUsers(w, r, sec, userForm{}, nil, userEdit{})
		return
	}

	s.redirect(w, r, sec.path+rawListQuery(r))
}

func (s *Server) sectionUser(w http.ResponseWriter, r *http.Request, sec userSection) (auth.User, bool) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return auth.User{}, false
	}

	user, err := s.auth.UserByID(r.Context(), id)
	if errors.Is(err, auth.ErrNotFound) || (err == nil && user.Role != sec.role) {
		http.NotFound(w, r)
		return auth.User{}, false
	}
	if err != nil {
		s.serverError(w, r, "load user", err)
		return auth.User{}, false
	}

	return user, true
}

func (s *Server) renderUsers(w http.ResponseWriter, r *http.Request, sec userSection, newForm userForm, newErrs validation.Errors, edit userEdit) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	classID := queryInt64(r, "class")
	inactive := showInactive(r)

	users, err := s.auth.Users(r.Context(), auth.UserFilter{Role: sec.role, Query: query, IncludeInactive: inactive})
	if err != nil {
		s.serverError(w, r, "list users", err)
		return
	}

	page := view.UsersPage{
		Shell:        s.shell(r, sec.title, sec.path),
		Title:        sec.title,
		Path:         sec.path,
		ListQuery:    listQuery(query, classID, inactive),
		Query:        query,
		ShowClass:    sec.hasClass(),
		ShowInactive: inactive,
		ToggleHref:   sec.path + listQuery(query, classID, !inactive),
		New:          view.UserFields{FullName: newForm.fullName, Errors: newErrs},
		EmptyMessage: sec.empty,
	}

	if query != "" || classID != 0 {
		page.EmptyMessage = nothingFound
	}

	var (
		classes        []school.Class
		studentClasses map[int64]school.Class
	)

	if sec.hasClass() {
		classes, err = s.school.Classes(r.Context(), s.school.CurrentYear(), false)
		if err != nil {
			s.serverError(w, r, "list classes", err)
			return
		}

		studentClasses, err = s.school.StudentClasses(r.Context(), s.school.CurrentYear())
		if err != nil {
			s.serverError(w, r, "list student classes", err)
			return
		}

		page.ClassFilter = classOptions(classes, classID, allClassesOption)
		page.New.Classes = classOptions(classes, newForm.classID, noClassOption)
	}

	editing := editingID(r, edit.entered, edit.id)

	for _, user := range users {
		class := studentClasses[user.ID]
		if classID != 0 && class.ID != classID {
			continue
		}

		row := view.UserRow{
			ID:        user.ID,
			FullName:  user.FullName,
			Login:     user.Login,
			ClassName: class.Name,
			Active:    user.Active,
			Editing:   user.ID == editing,
		}

		if row.Editing {
			form := userForm{fullName: user.FullName, login: user.Login, classID: class.ID}
			if edit.entered {
				form = edit.form
			}

			row.Fields = view.UserFields{FullName: form.fullName, Login: form.login, Errors: edit.errs}
			if sec.hasClass() {
				row.Fields.Classes = classOptions(classes, form.classID, noClassOption)
			}
		}

		page.Users = append(page.Users, row)
	}

	if isHTMX(r) {
		s.render(w, r, pages.UsersPage(page))
		return
	}

	s.render(w, r, pages.Users(page))
}

func classOptions(classes []school.Class, selected int64, first string) []view.Option {
	if len(classes) == 0 {
		return nil
	}

	options := make([]view.Option, 0, len(classes)+1)
	options = append(options, view.Option{Name: first, Selected: selected == 0})

	for _, class := range classes {
		options = append(options, view.Option{ID: class.ID, Name: class.Name, Selected: class.ID == selected})
	}

	return options
}

func queryInt64(r *http.Request, name string) int64 {
	value, err := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	if err != nil || value < 0 {
		return 0
	}

	return value
}

func listQuery(query string, classID int64, inactive bool) string {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}

	if classID != 0 {
		values.Set("class", strconv.FormatInt(classID, 10))
	}

	if inactive {
		values.Set("inactive", "1")
	}

	if len(values) == 0 {
		return ""
	}

	return "?" + values.Encode()
}

func rawListQuery(r *http.Request) string {
	return listQuery(strings.TrimSpace(r.URL.Query().Get("q")), queryInt64(r, "class"), showInactive(r))
}
