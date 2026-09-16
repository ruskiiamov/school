package diary

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	user, _ := web.UserFromContext(r.Context())
	child := queryID(r, "child")

	if user.Role == auth.RoleParent {
		h.parentSummary(w, r, user.ID, child)
		return
	}

	if child != 0 && child != user.ID {
		http.NotFound(w, r)
		return
	}

	h.renderMarks(w, r, diaryView{title: "Оценки", active: summaryPath, path: summaryPath, student: user.ID, hidden: map[string]string{}})
}

func (h *Handler) parentSummary(w http.ResponseWriter, r *http.Request, parentID, child int64) {
	selected, children, ok := h.selectChild(w, r, parentID, child)
	if !ok {
		return
	}

	if len(children) == 0 {
		h.renderMarksPage(w, r, view.MarksPage{Shell: h.base.Shell(r, "Оценки", summaryPath), Title: "Оценки", NoChildren: true})
		return
	}

	dv := diaryView{
		title:   "Оценки",
		active:  summaryPath,
		path:    summaryPath,
		student: selected.ID,
		hidden:  childHidden(selected.ID),
	}

	period := h.period(r)

	for _, candidate := range children {
		query := period.Query(url.Values{"child": {strconv.FormatInt(candidate.ID, 10)}})
		dv.children = append(dv.children, view.DiaryChild{
			Name:   candidate.FullName,
			Href:   summaryPath + "?" + query.Encode(),
			Active: candidate.ID == selected.ID,
		})
	}

	h.renderMarks(w, r, dv)
}

func (h *Handler) adminSummary(w http.ResponseWriter, r *http.Request) {
	student, title, ok := h.adminStudent(w, r)
	if !ok {
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	diary := adminDiaryPath + "/" + strconv.FormatInt(student.ID, 10)
	backHref := diary
	hidden := map[string]string{}

	if query != "" {
		backHref += "?" + url.Values{"q": {query}}.Encode()
		hidden["q"] = query
	}

	h.renderMarks(w, r, diaryView{
		title:    "Оценки · " + title,
		active:   adminDiaryPath,
		path:     diary + "/summary",
		student:  student.ID,
		hidden:   hidden,
		backHref: backHref,
	})
}

func (h *Handler) renderMarks(w http.ResponseWriter, r *http.Request, dv diaryView) {
	period := h.period(r)

	summary, err := h.journal.StudentSummary(r.Context(), dv.student, period.From, period.To)
	if err != nil {
		h.base.ServerError(w, r, "load student summary", err)
		return
	}

	page := view.MarksPage{
		Shell:    h.base.Shell(r, dv.title, dv.active),
		Title:    dv.title,
		Path:     dv.path,
		BackHref: dv.backHref,
		Children: dv.children,
		Hidden:   dv.hidden,
		Period:   web.PeriodForm(period, h.school.Today(), dv.path, dv.hidden),
	}

	diary := diaryView{path: diaryPath, hidden: dv.hidden}
	if dv.backHref != "" {
		diary.path = dv.backHref
		if i := strings.Index(diary.path, "?"); i >= 0 {
			diary.path = diary.path[:i]
		}
	}

	for _, subject := range summary.Subjects {
		row := view.SubjectMarksRow{
			Name:     subject.SubjectName,
			Absences: strconv.Itoa(subject.Absences),
			Average:  view.FormatAverage(subject.Average, len(subject.Marks)),
		}

		for _, mark := range subject.Marks {
			row.Marks = append(row.Marks, view.MarkChip{
				Value: strconv.Itoa(mark.Value),
				Title: markTitle(mark),
				Href:  diaryURL(diary, mark.Date, mark.LessonID),
			})
		}

		page.Subjects = append(page.Subjects, row)
	}

	h.renderMarksPage(w, r, page)
}

func (h *Handler) renderMarksPage(w http.ResponseWriter, r *http.Request, page view.MarksPage) {
	if web.IsHTMX(r) {
		h.base.Render(w, r, pages.MarksContent(page))
		return
	}

	h.base.Render(w, r, pages.Marks(page))
}

func (h *Handler) period(r *http.Request) web.Period {
	start, end := h.school.YearBounds(h.school.CurrentYear())

	return web.ParsePeriod(r, h.school.Today(), start, end)
}

func markTitle(mark journal.SummaryMark) string {
	title := view.FormatShortDate(mark.Date) + " · " + mark.WorkTypeName
	if mark.Label != "" {
		title += " · " + mark.Label
	}

	return title
}
