package penalties

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	basePath     = view.PenaltiesPath
	studentsPath = basePath + "/students"
	workoffsPath = basePath + "/workoffs"
	classOption  = "Все классы"
)

type Handler struct {
	base    *web.Base
	auth    *auth.Service
	school  *school.Service
	journal *journal.Service
}

func New(base *web.Base, authService *auth.Service, schoolService *school.Service, journalService *journal.Service) *Handler {
	return &Handler{base: base, auth: authService, school: schoolService, journal: journalService}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET "+basePath, h.staff(h.list))
	mux.Handle("GET "+studentsPath+"/{sid}", h.staff(h.student))
	mux.Handle("POST "+studentsPath+"/{sid}/workoffs", h.staff(h.workoffCreate))
	mux.Handle("POST "+workoffsPath+"/{id}/delete", h.staff(h.workoffDelete))
}

func (h *Handler) staff(fn http.HandlerFunc) http.Handler {
	enabled := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.school.PenaltySettings().Enabled {
			http.NotFound(w, r)
			return
		}

		fn(w, r)
	})

	return h.base.RequireAuth(web.RequireRole(auth.RoleAdmin, auth.RoleTeacher)(enabled))
}

type filter struct {
	query   string
	year    int
	years   []view.Option
	classID int64
}

func (h *Handler) filter(r *http.Request) (filter, error) {
	year, years, err := web.YearSelection(r, h.school)
	if err != nil {
		return filter{}, err
	}

	classID, err := strconv.ParseInt(r.URL.Query().Get("class"), 10, 64)
	if err != nil || classID < 0 {
		classID = 0
	}

	return filter{query: strings.TrimSpace(r.URL.Query().Get("q")), year: year, years: years, classID: classID}, nil
}

func (f filter) suffix() string {
	values := url.Values{"year": {strconv.Itoa(f.year)}}

	if f.query != "" {
		values.Set("q", f.query)
	}

	if f.classID != 0 {
		values.Set("class", strconv.FormatInt(f.classID, 10))
	}

	return "?" + values.Encode()
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	settings := h.school.PenaltySettings()

	f, err := h.filter(r)
	if err != nil {
		h.base.ServerError(w, r, "list class years", err)
		return
	}

	classes, err := h.school.Classes(ctx, f.year, true)
	if err != nil {
		h.base.ServerError(w, r, "list classes", err)
		return
	}

	page := view.PenaltiesPage{
		Shell:   h.base.Shell(r, settings.Name, basePath),
		Title:   settings.Name,
		Path:    basePath,
		Query:   f.query,
		Years:   f.years,
		Classes: []view.Option{{Name: classOption, Selected: f.classID == 0}},
		Empty:   "Долгов нет",
	}

	order := make(map[int64]int, len(classes))

	for i, class := range classes {
		order[class.ID] = i
		page.Classes = append(page.Classes, view.Option{ID: class.ID, Name: view.ClassTitle(class.Name), Selected: class.ID == f.classID})
	}

	if _, known := order[f.classID]; !known {
		f.classID = 0
	}

	if f.query != "" || f.classID != 0 {
		page.Empty = "Никого не нашли"
	}

	if page.Debtors, err = h.debtors(r, f, order); err != nil {
		h.base.ServerError(w, r, "list debtors", err)
		return
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.PenaltiesContent(page))
		return
	}

	h.base.Render(w, r, pages.Penalties(page))
}

func (h *Handler) debtors(r *http.Request, f filter, order map[int64]int) ([]view.DebtorRow, error) {
	ctx := r.Context()
	settings := h.school.PenaltySettings()

	debts, err := h.journal.Debts(ctx)
	if err != nil {
		return nil, err
	}

	students, err := h.auth.Users(ctx, auth.UserFilter{Role: auth.RoleStudent, Query: f.query})
	if err != nil {
		return nil, err
	}

	studentClasses, err := h.school.StudentClasses(ctx, f.year)
	if err != nil {
		return nil, err
	}

	type debtor struct {
		row   view.DebtorRow
		class int
		name  string
	}

	var found []debtor

	for _, student := range students {
		class, inClass := studentClasses[student.ID]
		debt := debts[student.ID]

		if debt == 0 || !inClass || (f.classID != 0 && class.ID != f.classID) {
			continue
		}

		found = append(found, debtor{
			row: view.DebtorRow{
				FullName:  student.FullName,
				ClassName: view.ClassTitle(class.Name),
				Debt:      view.FormatDebt(settings.Unit, debt),
				Href:      studentsPath + "/" + strconv.FormatInt(student.ID, 10) + f.suffix(),
			},
			class: order[class.ID],
			name:  student.FullName,
		})
	}

	slices.SortStableFunc(found, func(a, b debtor) int {
		if a.class != b.class {
			return a.class - b.class
		}

		return strings.Compare(a.name, b.name)
	})

	rows := make([]view.DebtorRow, 0, len(found))
	for _, item := range found {
		rows = append(rows, item.row)
	}

	return rows, nil
}

func (h *Handler) student(w http.ResponseWriter, r *http.Request) {
	h.renderStudent(w, r, nil)
}

func (h *Handler) workoffCreate(w http.ResponseWriter, r *http.Request) {
	student, ok := h.pathStudent(w, r)
	if !ok {
		return
	}

	user, _ := web.UserFromContext(r.Context())
	form := &view.WorkoffForm{
		Date:    web.FormValue(r, "date"),
		Points:  web.FormValue(r, "points"),
		Comment: web.FormValue(r, "comment"),
	}

	input := journal.WorkoffInput{Date: form.Date, Points: form.Points, Comment: form.Comment}

	_, err := h.journal.AddWorkoff(r.Context(), user.ID, student.ID, input, h.school.Today())
	if errs, invalid := web.FormErrors(err); invalid {
		form.Errors = errs
		h.renderStudent(w, r, form)

		return
	}
	if err != nil {
		h.base.ServerError(w, r, "add workoff", err)
		return
	}

	h.studentDone(w, r, student.ID)
}

func (h *Handler) workoffDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := web.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	workoff, err := h.journal.WorkoffByID(r.Context(), id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load workoff", err)
		return
	}

	user, _ := web.UserFromContext(r.Context())

	if user.Role == auth.RoleAdmin {
		err = h.journal.DeleteWorkoff(r.Context(), id)
	} else {
		err = h.journal.DeleteOwnWorkoff(r.Context(), user.ID, id)
	}

	if err != nil {
		h.base.HandleServiceError(w, r, "delete workoff", err)
		return
	}

	h.studentDone(w, r, workoff.StudentID)
}

func (h *Handler) studentDone(w http.ResponseWriter, r *http.Request, studentID int64) {
	if web.IsHTMX(r) {
		r.SetPathValue("sid", strconv.FormatInt(studentID, 10))
		h.renderStudent(w, r, nil)

		return
	}

	f, err := h.filter(r)
	if err != nil {
		h.base.ServerError(w, r, "list class years", err)
		return
	}

	web.Redirect(w, r, studentsPath+"/"+strconv.FormatInt(studentID, 10)+f.suffix())
}

func (h *Handler) pathStudent(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	id, ok := web.PathValue(r, "sid")
	if !ok {
		http.NotFound(w, r)
		return auth.User{}, false
	}

	student, err := h.auth.UserByID(r.Context(), id)
	if err != nil {
		h.base.HandleServiceError(w, r, "load student", err)
		return auth.User{}, false
	}

	if student.Role != auth.RoleStudent {
		http.NotFound(w, r)
		return auth.User{}, false
	}

	return student, true
}

func (h *Handler) renderStudent(w http.ResponseWriter, r *http.Request, form *view.WorkoffForm) {
	ctx := r.Context()
	settings := h.school.PenaltySettings()

	student, ok := h.pathStudent(w, r)
	if !ok {
		return
	}

	f, err := h.filter(r)
	if err != nil {
		h.base.ServerError(w, r, "list class years", err)
		return
	}

	history, err := h.journal.PenaltyHistory(ctx, student.ID)
	if err != nil {
		h.base.ServerError(w, r, "load penalty history", err)
		return
	}

	recorders, err := h.recorderNames(r)
	if err != nil {
		h.base.ServerError(w, r, "list staff", err)
		return
	}

	user, _ := web.UserFromContext(ctx)
	suffix := f.suffix()

	block := view.PenaltyStudentBlock{
		History: web.PenaltyHistory(settings, history, web.HistoryLinks{
			Recorder: recorders,
			Delete: func(workoff journal.Workoff) string {
				if user.Role != auth.RoleAdmin && workoff.RecordedBy != user.ID {
					return ""
				}

				return workoffsPath + "/" + strconv.FormatInt(workoff.ID, 10) + "/delete" + suffix
			},
		}),
	}

	if history.Debt > 0 || form != nil {
		if form == nil {
			form = &view.WorkoffForm{Date: h.school.Today().Format(validation.InputDateLayout)}
		}

		form.Action = studentsPath + "/" + strconv.FormatInt(student.ID, 10) + "/workoffs" + suffix
		form.Unit = settings.Unit
		block.Form = form
	}

	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.PenaltyStudentContent(block))
		return
	}

	title := student.FullName

	class, _, err := h.school.StudentClassIn(ctx, student.ID, f.year)
	if err != nil {
		h.base.ServerError(w, r, "load student class", err)
		return
	}

	if class.Name != "" {
		title += " · " + view.ClassTitle(class.Name)
	}

	h.base.Render(w, r, pages.PenaltyStudent(view.PenaltyStudentPage{
		Shell:    h.base.Shell(r, title, basePath),
		Title:    title,
		BackHref: basePath + suffix,
		Block:    block,
	}))
}

func (h *Handler) recorderNames(r *http.Request) (map[int64]string, error) {
	names := map[int64]string{}

	for _, role := range []auth.Role{auth.RoleTeacher, auth.RoleAdmin} {
		users, err := h.auth.Users(r.Context(), auth.UserFilter{Role: role, IncludeInactive: true})
		if err != nil {
			return nil, err
		}

		for _, user := range users {
			names[user.ID] = user.FullName
		}
	}

	return names, nil
}
