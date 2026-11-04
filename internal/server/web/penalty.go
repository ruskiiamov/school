package web

import (
	"github.com/ruskiiamov/school/internal/journal"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/view"
)

const MsgPenaltyLocked = "Штраф погашен, изменить нельзя"

var penaltyStatuses = map[journal.PenaltyStatus]string{
	journal.PenaltyOpen:    "не погашено",
	journal.PenaltyPartial: "частично",
	journal.PenaltyPaid:    "погашено",
}

func PenaltyStatus(status journal.PenaltyStatus) string {
	return penaltyStatuses[status]
}

func PenaltySummary(settings school.PenaltySettings, penalty journal.Penalty) string {
	summary := "Выполнено " + view.FormatPercent(penalty.DonePercent) + " · " + view.FormatPoints(settings.Unit, penalty.Points)
	if status := PenaltyStatus(penalty.Status); status != "" {
		summary += " · " + status
	}

	return summary
}

type HistoryLinks struct {
	Lesson   func(journal.HistoryPenalty) string
	Delete   func(journal.Workoff) string
	Recorder map[int64]string
}

func PenaltyHistory(settings school.PenaltySettings, history journal.PenaltyHistory, links HistoryLinks) view.PenaltyHistory {
	result := view.PenaltyHistory{Debt: view.FormatDebt(settings.Unit, history.Debt), Unit: settings.Unit}

	for _, penalty := range history.Penalties {
		row := view.PenaltyHistoryRow{
			Date:     view.FormatShortDate(penalty.Date),
			Subject:  penalty.SubjectName,
			WorkType: penalty.WorkTypeName,
			Done:     view.FormatPercent(penalty.DonePercent),
			Points:   view.FormatPoints(settings.Unit, penalty.Points),
			Status:   PenaltyStatus(penalty.Status),
		}

		if links.Lesson != nil {
			row.Href = links.Lesson(penalty)
		}

		result.Penalties = append(result.Penalties, row)
	}

	for _, workoff := range history.Workoffs {
		date := view.FormatShortDate(workoff.Date)
		row := view.WorkoffRow{
			Date:       date,
			Points:     view.FormatPoints(settings.Unit, workoff.Points),
			RecordedBy: view.ShortName(links.Recorder[workoff.RecordedBy]),
			Comment:    workoff.Comment,
		}

		if links.Delete != nil {
			row.DeleteAction = links.Delete(workoff)
			row.Confirm = "Удалить отработку от " + date + "?"
		}

		result.Workoffs = append(result.Workoffs, row)
	}

	return result
}
