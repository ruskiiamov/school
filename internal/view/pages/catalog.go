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

func inactiveToggleURL(base string, showInactive bool) string {
	if showInactive {
		return base
	}

	return base + "?inactive=1"
}

func classesQuery(year int, showInactive bool) string {
	query := "?year=" + strconv.Itoa(year)
	if showInactive {
		query += "&inactive=1"
	}

	return query
}

func classURL(id int64, suffix string, year int, showInactive bool) string {
	url := classesBase
	if id != 0 {
		url += "/" + strconv.FormatInt(id, 10) + suffix
	}

	return url + classesQuery(year, showInactive)
}

func classEditURL(id int64, year int, showInactive bool) string {
	return classesBase + classesQuery(year, showInactive) + "&edit=" + strconv.FormatInt(id, 10)
}
