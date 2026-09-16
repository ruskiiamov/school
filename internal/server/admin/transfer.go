package admin

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const transferPath = classesPath + "/transfer"

func (h *Handler) transferShow(w http.ResponseWriter, r *http.Request) {
	plan, err := h.school.TransferPlan(r.Context())
	if err != nil {
		h.base.ServerError(w, r, "plan class transfer", err)
		return
	}

	h.renderTransfer(w, r, plan, defaultTransferInput(plan), nil)
}

func (h *Handler) transferApply(w http.ResponseWriter, r *http.Request) {
	plan, err := h.school.TransferPlan(r.Context())
	if err != nil {
		h.base.ServerError(w, r, "plan class transfer", err)
		return
	}

	input := transferInputFromForm(r, plan)

	err = h.school.Transfer(r.Context(), input)
	if errs, ok := web.FormErrors(err); ok {
		h.renderTransfer(w, r, plan, input, errs)
		return
	}
	if err != nil {
		h.base.HandleServiceError(w, r, "transfer classes", err)
		return
	}

	web.Redirect(w, r, classesListURL(plan.ToYear, false))
}

func defaultTransferInput(plan school.TransferPlan) school.TransferInput {
	input := school.TransferInput{}

	for _, item := range plan.Classes {
		class := school.TransferClassInput{
			ClassID:  item.Class.ID,
			Transfer: !item.Exists && !item.Graduating,
			Name:     item.NewName,
		}

		for _, student := range item.Students {
			if item.Eligible(student.ID) {
				class.StudentIDs = append(class.StudentIDs, student.ID)
			}
		}

		input.Classes = append(input.Classes, class)
	}

	return input
}

func transferInputFromForm(r *http.Request, plan school.TransferPlan) school.TransferInput {
	input := school.TransferInput{}

	for _, item := range plan.Classes {
		key := strconv.FormatInt(item.Class.ID, 10)
		class := school.TransferClassInput{
			ClassID:  item.Class.ID,
			Transfer: r.PostFormValue("transfer-"+key) != "",
			Name:     web.FormValue(r, "name-"+key),
		}

		for _, value := range r.PostForm["students-"+key] {
			if id, err := strconv.ParseInt(value, 10, 64); err == nil {
				class.StudentIDs = append(class.StudentIDs, id)
			}
		}

		input.Classes = append(input.Classes, class)
	}

	return input
}

func (h *Handler) renderTransfer(w http.ResponseWriter, r *http.Request, plan school.TransferPlan, input school.TransferInput, errs map[string]string) {
	students, err := h.auth.Users(r.Context(), auth.UserFilter{Role: auth.RoleStudent, IncludeInactive: true})
	if err != nil {
		h.base.ServerError(w, r, "list students", err)
		return
	}

	names := make(map[int64]string, len(students))
	for _, student := range students {
		names[student.ID] = student.FullName
	}

	entered := make(map[int64]school.TransferClassInput, len(input.Classes))
	for _, class := range input.Classes {
		entered[class.ClassID] = class
	}

	page := view.TransferPage{
		Shell:        h.base.Shell(r, "Перевод классов", classesPath),
		FromYearName: school.YearName(plan.FromYear),
		ToYearName:   school.YearName(plan.ToYear),
		Action:       transferPath,
		BackHref:     classesListURL(plan.ToYear, false),
		Error:        errs["form"],
	}

	for _, item := range plan.Classes {
		key := strconv.FormatInt(item.Class.ID, 10)
		class := entered[item.Class.ID]

		row := view.TransferClassRow{
			ID:            item.Class.ID,
			Name:          item.Class.Name,
			NewName:       class.Name,
			Transfer:      class.Transfer,
			Graduating:    item.Graduating,
			Exists:        item.Exists,
			NameError:     errs["name-"+key],
			StudentsError: errs["students-"+key],
		}

		for _, student := range item.Students {
			row.Students = append(row.Students, view.TransferStudentRow{
				ID:           student.ID,
				FullName:     names[student.ID],
				Active:       student.Active,
				CurrentClass: student.CurrentClass,
				Checked:      slices.Contains(class.StudentIDs, student.ID),
			})
		}

		slices.SortFunc(row.Students, func(a, b view.TransferStudentRow) int {
			if a.FullName < b.FullName {
				return -1
			}
			if a.FullName > b.FullName {
				return 1
			}

			return 0
		})

		page.Classes = append(page.Classes, row)
	}

	h.base.Render(w, r, pages.Transfer(page))
}
