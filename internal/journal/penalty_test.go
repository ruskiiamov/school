package journal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ruskiiamov/school/internal/storage"
)

func TestPenaltyPoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		full, done, points int
	}{
		{20, 30, 14},
		{20, 33, 13},
		{15, 50, 8},
		{20, 0, 20},
		{20, 98, 0},
		{20, 97, 1},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.points, PenaltyPoints(tt.full, tt.done), "%d × (100 − %d)", tt.full, tt.done)
	}
}

func TestParseDonePercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw     string
		percent int
		ok      bool
	}{
		{"", 100, true},
		{"30", 30, true},
		{" 30 % ", 30, true},
		{"0", 0, true},
		{"100", 100, true},
		{"101", 0, false},
		{"-1", 0, false},
		{"30,5", 0, false},
		{"abc", 0, false},
	}

	for _, tt := range tests {
		percent, ok := parseDonePercent(tt.raw)
		assert.Equal(t, tt.ok, ok, tt.raw)
		assert.Equal(t, tt.percent, percent, tt.raw)
	}
}

func TestLedgerCoversOldestFirst(t *testing.T) {
	t.Parallel()

	const student = 7

	day := func(d int) time.Time { return time.Date(2026, time.October, d, 0, 0, 0, 0, time.UTC) }
	penalty := func(id int64, points int, lessonDate time.Time) storage.StudentPenalty {
		return storage.StudentPenalty{Penalty: storage.Penalty{ID: id, StudentID: student, Points: points}, Date: lessonDate}
	}

	algebra := penalty(1, 20, day(1))
	history := penalty(2, 14, day(10))
	earlier := penalty(3, 10, day(20).AddDate(0, -1, 0))
	empty := penalty(4, 0, day(21))

	tests := []struct {
		name      string
		penalties []storage.StudentPenalty
		worked    int
		statuses  map[int64]PenaltyStatus
		debt      int
	}{
		{"nothing worked", []storage.StudentPenalty{algebra, history}, 0,
			map[int64]PenaltyStatus{1: PenaltyOpen, 2: PenaltyOpen}, 34},
		{"oldest first", []storage.StudentPenalty{algebra, history}, 25,
			map[int64]PenaltyStatus{1: PenaltyPaid, 2: PenaltyPartial}, 9},
		{"older lesson first", []storage.StudentPenalty{earlier, algebra, history}, 25,
			map[int64]PenaltyStatus{3: PenaltyPaid, 1: PenaltyPartial, 2: PenaltyOpen}, 19},
		{"all paid", []storage.StudentPenalty{algebra, history}, 34,
			map[int64]PenaltyStatus{1: PenaltyPaid, 2: PenaltyPaid}, 0},
		{"zero points have no status", []storage.StudentPenalty{algebra, empty}, 20,
			map[int64]PenaltyStatus{1: PenaltyPaid, 4: PenaltyNone}, 0},
	}

	for _, tt := range tests {
		result := buildLedger(tt.penalties, map[int64]int{student: tt.worked})
		assert.Equal(t, tt.statuses, result.statuses, tt.name)
		assert.Equal(t, tt.debt, result.debts[student], tt.name)
	}
}
