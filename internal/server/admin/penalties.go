package admin

import (
	"net/http"
	"strconv"

	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	penaltySettingsPath  = "/admin/penalties"
	penaltySettingsTitle = "Штрафы"
)

func (h *Handler) penaltySettingsShow(w http.ResponseWriter, r *http.Request) {
	settings := h.school.PenaltySettings()

	h.renderPenaltySettings(w, r, view.PenaltySettingsPage{
		Enabled: settings.Enabled,
		Name:    settings.Name,
		Unit:    settings.Unit,
		Full:    strconv.Itoa(settings.Full),
		Done:    r.URL.Query().Get("done") == "1",
	})
}

func (h *Handler) penaltySettingsSave(w http.ResponseWriter, r *http.Request) {
	input := school.PenaltySettingsInput{
		Enabled: r.PostFormValue("enabled") == "1",
		Name:    web.FormValue(r, "name"),
		Unit:    web.FormValue(r, "unit"),
		Full:    web.FormValue(r, "full"),
	}

	err := h.school.SavePenaltySettings(r.Context(), input)
	if errs, ok := web.FormErrors(err); ok {
		h.renderPenaltySettings(w, r, view.PenaltySettingsPage{
			Enabled: input.Enabled,
			Name:    input.Name,
			Unit:    input.Unit,
			Full:    input.Full,
			Errors:  errs,
		})

		return
	}
	if err != nil {
		h.base.ServerError(w, r, "save penalty settings", err)
		return
	}

	web.Redirect(w, r, penaltySettingsPath+"?done=1")
}

func (h *Handler) renderPenaltySettings(w http.ResponseWriter, r *http.Request, page view.PenaltySettingsPage) {
	page.Shell = h.base.Shell(r, penaltySettingsTitle, penaltySettingsPath)
	page.Action = penaltySettingsPath

	h.base.Render(w, r, pages.PenaltySettings(page))
}
