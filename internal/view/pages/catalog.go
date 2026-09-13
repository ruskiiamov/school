package pages

import "strconv"

func catalogURL(base string, id int64, suffix string, showInactive bool) string {
	url := base
	if id != 0 {
		url += "/" + strconv.FormatInt(id, 10) + suffix
	}

	if showInactive {
		url += "?inactive=1"
	}

	return url
}

func catalogEditURL(base string, id int64, showInactive bool) string {
	url := base + "?edit=" + strconv.FormatInt(id, 10)
	if showInactive {
		url += "&inactive=1"
	}

	return url
}
