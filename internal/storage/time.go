package storage

import "time"

func toMillis(t time.Time) int64 {
	return t.UnixMilli()
}

func fromMillis(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}
