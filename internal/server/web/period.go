package web

import (
	"net/http"
	"net/url"
	"time"

	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
)

type Period struct {
	From time.Time
	To   time.Time
	min  time.Time
	max  time.Time
}

func ParsePeriod(r *http.Request, today, min, max time.Time) Period {
	p := Period{min: min, max: max}
	query := r.URL.Query()

	from, fromOK := validation.ParseDate(query.Get("from"))
	to, toOK := validation.ParseDate(query.Get("to"))

	if !fromOK || !toOK {
		return p.month(today)
	}

	if to.Before(from) {
		from, to = to, from
	}

	p.From, p.To = from, to

	return p.clamp()
}

func (p Period) Month(delta int) Period {
	return p.month(time.Date(p.From.Year(), p.From.Month()+time.Month(delta), 1, 0, 0, 0, 0, time.UTC))
}

func (p Period) month(anchor time.Time) Period {
	p.From = time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	p.To = p.From.AddDate(0, 1, -1)

	return p.clamp()
}

func (p Period) clamp() Period {
	if p.From.Before(p.min) {
		p.From = p.min
	}

	if p.To.After(p.max) {
		p.To = p.max
	}

	if p.To.Before(p.From) {
		p.To = p.From
	}

	return p
}

func (p Period) Query(query url.Values) url.Values {
	result := url.Values{}
	for name, values := range query {
		result[name] = values
	}

	result.Set("from", p.From.Format(validation.DateLayout))
	result.Set("to", p.To.Format(validation.DateLayout))

	return result
}

func PeriodForm(p Period, today time.Time, path string, hidden map[string]string) view.PeriodForm {
	base := url.Values{}
	for name, value := range hidden {
		base.Set(name, value)
	}

	href := func(target Period) string {
		return path + "?" + target.Query(base).Encode()
	}

	return view.PeriodForm{
		From:     p.From.Format(validation.DateLayout),
		To:       p.To.Format(validation.DateLayout),
		Label:    view.FormatPeriod(p.From, p.To),
		PrevHref: href(p.Month(-1)),
		ThisHref: href(Period{min: p.min, max: p.max}.month(today)),
		NextHref: href(p.Month(1)),
	}
}
