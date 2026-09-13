package components

import (
	"strconv"

	"github.com/ruskiiamov/school/internal/view"
)

func optionValue(option view.Option) string {
	return strconv.FormatInt(option.ID, 10)
}
