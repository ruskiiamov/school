package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	allClassesOption = "Все классы"
	noClassOption    = "Без класса"
	nothingFound     = "Ничего не найдено"
)

type userSection struct {
	role      auth.Role
	path      string
	title     string
	created   string
	recipient string
	empty     string
	hint      string
}

var userSections = []userSection{
	{
		role: auth.RoleTeacher, path: "/admin/teachers", title: "Учителя",
		created: "Учитель создан", recipient: "учителю", empty: "Пока нет учителей",
		hint: "Логин и одноразовый пароль приложение придумывает само и показывает один раз после создания. Какие классы и предметы ведёт учитель, задаётся в карточке класса",
	},
	{
		role: auth.RoleStudent, path: "/admin/students", title: "Ученики",
		created: "Ученик создан", recipient: "ученику", empty: "Пока нет учеников",
		hint: "Логин и одноразовый пароль приложение придумывает само и показывает один раз после создания. Класс ученика можно указать сразу или позже, в строке правки",
	},
	parentsSection,
}

var parentsSection = userSection{
	role: auth.RoleParent, path: "/admin/parents", title: "Родители",
	created: "Родитель создан", recipient: "родителю", empty: "Пока нет родителей",
	hint: "Родитель видит дневники и оценки своих детей. Детей привязывают в строке правки: нажмите на строку родителя и найдите ученика по ФИО",
}

func (sec userSection) hasClass() bool {
	return sec.role == auth.RoleStudent
}

func (sec userSection) userPath(id int64, suffix string) string {
	return sec.path + "/" + strconv.FormatInt(id, 10) + suffix
}

type userForm struct {
	name    auth.Name
	login   string
	classID int64
}

type userEdit struct {
	id      int64
	form    userForm
	entered bool
	errs    validation.Errors
}

func readUserForm(r *http.Request) userForm {
	return userForm{
		name: auth.Name{
			Last:   web.FormValue(r, "last_name"),
			First:  web.FormValue(r, "first_name"),
			Middle: web.FormValue(r, "middle_name"),
		},
		login:   web.FormValue(r, "login"),
		classID: web.FormInt64(r, "class"),
	}
}

func (h *Handler) usersList(sec userSection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.renderUsers(w, r, sec, userForm{}, nil, userEdit{})
	}
}

func (h *Handler) userCreate(sec userSection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		form := readUserForm(r)

		if sec.hasClass() {
			err := h.school.CheckStudentClass(r.Context(), form.classID)
			if errs, ok := web.FormErrors(err); ok {
				h.renderUsers(w, r, sec, form, errs, userEdit{})
				return
			}
			if err != nil {
				h.base.ServerError(w, r, "check student class", err)
				return
			}
		}

		credentials, err := h.auth.CreateUser(r.Context(), auth.NewUser{Role: sec.role, Name: form.name})
		if errs, ok := web.FormErrors(err); ok {
			h.renderUsers(w, r, sec, form, errs, userEdit{})
			return
		}
		if err != nil {
			h.base.ServerError(w, r, "create user", err)
			return
		}

		if sec.hasClass() && form.classID != 0 {
			if err := h.school.SetStudentClass(r.Context(), credentials.User.ID, form.classID); err != nil {
				h.base.ServerError(w, r, "set student class", err)
				return
			}
		}

		h.created.put(h.base.SessionID(r), credentialsEntry{
			userID:   credentials.User.ID,
			kind:     credentialsCreated,
			login:    credentials.User.Login,
			password: credentials.Password,
		})

		web.Redirect(w, r, sec.userPath(credentials.User.ID, "/created"))
	}
}

func (h *Handler) userCreated(sec userSection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := h.sectionUser(w, r, sec)
		if !ok {
			return
		}

		entry, ok := h.created.take(h.base.SessionID(r), user.ID, credentialsCreated)
		if !ok {
			web.Redirect(w, r, sec.path)
			return
		}

		page := view.UserCreatedPage{
			Shell:     h.base.Shell(r, sec.created, sec.path),
			Title:     sec.created,
			FullName:  user.FullName,
			Login:     entry.login,
			Password:  entry.password,
			Note:      "Пароль показан один раз. Передайте данные " + sec.recipient,
			ListHref:  sec.path,
			ListTitle: "К списку",
		}

		h.base.Render(w, r, pages.UserCreated(page))
	}
}

func (h *Handler) userUpdate(sec userSection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := h.sectionUser(w, r, sec)
		if !ok {
			return
		}

		form := readUserForm(r)
		edit := userEdit{id: user.ID, form: form, entered: true}

		if sec.hasClass() {
			err := h.school.CheckStudentClass(r.Context(), form.classID)
			if errs, ok := web.FormErrors(err); ok {
				edit.errs = errs
				h.renderUsers(w, r, sec, userForm{}, nil, edit)

				return
			}
			if err != nil {
				h.base.ServerError(w, r, "check student class", err)
				return
			}
		}

		err := h.auth.UpdateUser(r.Context(), user.ID, auth.UserInput{Name: form.name, Login: form.login})
		if errs, ok := web.FormErrors(err); ok {
			edit.errs = errs
			h.renderUsers(w, r, sec, userForm{}, nil, edit)

			return
		}
		if err != nil {
			h.base.HandleServiceError(w, r, "update user", err)
			return
		}

		if sec.hasClass() {
			if err := h.school.SetStudentClass(r.Context(), user.ID, form.classID); err != nil {
				h.base.ServerError(w, r, "set student class", err)
				return
			}
		}

		h.usersDone(w, r, sec)
	}
}

func (h *Handler) userSetActive(sec userSection, active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := h.sectionUser(w, r, sec)
		if !ok {
			return
		}

		if err := h.auth.SetUserActive(r.Context(), user.ID, active); err != nil {
			h.base.HandleServiceError(w, r, "set user active", err)
			return
		}

		h.usersDone(w, r, sec)
	}
}

func (h *Handler) usersDone(w http.ResponseWriter, r *http.Request, sec userSection) {
	if web.IsHTMX(r) {
		h.renderUsers(w, r, sec, userForm{}, nil, userEdit{})
		return
	}

	web.Redirect(w, r, sec.path+rawListQuery(r))
}

func (h *Handler) sectionUser(w http.ResponseWriter, r *http.Request, sec userSection) (auth.User, bool) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return auth.User{}, false
	}

	user, err := h.auth.UserByID(r.Context(), id)
	if errors.Is(err, auth.ErrNotFound) || (err == nil && user.Role != sec.role) {
		http.NotFound(w, r)
		return auth.User{}, false
	}
	if err != nil {
		h.base.ServerError(w, r, "load user", err)
		return auth.User{}, false
	}

	return user, true
}

func (h *Handler) renderUsers(w http.ResponseWriter, r *http.Request, sec userSection, newForm userForm, newErrs validation.Errors, edit userEdit) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	classID := queryInt64(r, "class")
	inactive := showInactive(r)

	users, err := h.auth.Users(r.Context(), auth.UserFilter{Role: sec.role, Query: query, IncludeInactive: inactive})
	if err != nil {
		h.base.ServerError(w, r, "list users", err)
		return
	}

	page := view.UsersPage{
		Shell:        h.base.Shell(r, sec.title, sec.path),
		Title:        sec.title,
		Hint:         sec.hint,
		Path:         sec.path,
		ListQuery:    listQuery(query, classID, inactive),
		Query:        query,
		ShowClass:    sec.hasClass(),
		ShowInactive: inactive,
		ToggleHref:   sec.path + listQuery(query, classID, !inactive),
		New:          userFieldsView(newForm, newErrs),
		EmptyMessage: sec.empty,
	}

	if query != "" || classID != 0 {
		page.EmptyMessage = nothingFound
	}

	var (
		classes        []school.Class
		studentClasses map[int64]school.Class
		children       childrenData
	)

	if sec.role == auth.RoleParent {
		children, err = h.loadChildren(r)
		if err != nil {
			h.base.ServerError(w, r, "list children", err)
			return
		}

		page.ShowChildren = true
	}

	if sec.hasClass() {
		classes, err = h.school.Classes(r.Context(), h.school.CurrentYear(), false)
		if err != nil {
			h.base.ServerError(w, r, "list classes", err)
			return
		}

		studentClasses, err = h.school.StudentClasses(r.Context(), h.school.CurrentYear())
		if err != nil {
			h.base.ServerError(w, r, "list student classes", err)
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

		if page.ShowChildren {
			row.ChildNames = children.names(user.ID)
		}

		if row.Editing {
			form := userForm{name: user.Name, login: user.Login, classID: class.ID}
			if edit.entered {
				form = edit.form
			}

			row.Fields = userFieldsView(form, edit.errs)
			if sec.hasClass() {
				row.Fields.Classes = classOptions(classes, form.classID, noClassOption)
			}

			if page.ShowChildren {
				row.Children, err = h.childrenBlock(r, children, user.ID, edit.errs["child"])
				if err != nil {
					h.base.ServerError(w, r, "search children", err)
					return
				}
			}
		}

		page.Users = append(page.Users, row)
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.UsersPage(page))
		return
	}

	h.base.Render(w, r, pages.Users(page))
}

func userFieldsView(form userForm, errs validation.Errors) view.UserFields {
	return view.UserFields{
		LastName:   form.name.Last,
		FirstName:  form.name.First,
		MiddleName: form.name.Middle,
		Login:      form.login,
		Errors:     errs,
	}
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
	return encodeQuery(listValues(query, classID, inactive))
}

func listValues(query string, classID int64, inactive bool) url.Values {
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

	return values
}

func encodeQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	return "?" + values.Encode()
}

func rawListQuery(r *http.Request) string {
	return listQuery(strings.TrimSpace(r.URL.Query().Get("q")), queryInt64(r, "class"), showInactive(r))
}
