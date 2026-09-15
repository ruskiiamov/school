package storage

import (
	"database/sql"
	"time"
)

func toMillis(t time.Time) int64 {
	return t.UnixMilli()
}

func fromMillis(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}

const dateLayout = "2006-01-02"

func toDate(t time.Time) string {
	return t.Format(dateLayout)
}

func fromDate(value string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, value, time.UTC)
}

func toNullDate(t time.Time) sql.NullString {
	if t.IsZero() {
		return sql.NullString{}
	}

	return sql.NullString{String: toDate(t), Valid: true}
}

func fromNullDate(value sql.NullString) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, nil
	}

	return fromDate(value.String)
}
