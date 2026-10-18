package validation

import "time"

const (
	DateLayout      = "2006-01-02"
	InputDateLayout = "02.01.2006"
)

var dateLayouts = []string{"2.1.2006", DateLayout}

func ParseDate(value string) (time.Time, bool) {
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}
