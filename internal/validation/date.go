package validation

import "time"

const DateLayout = "2006-01-02"

func ParseDate(value string) (time.Time, bool) {
	t, err := time.ParseInLocation(DateLayout, value, time.UTC)
	if err != nil {
		return time.Time{}, false
	}

	return t, true
}
