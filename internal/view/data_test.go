package view

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNavItemsMarkActive(t *testing.T) {
	t.Parallel()

	items := NavItems("/")

	assert.True(t, items[0].Active)
	assert.False(t, items[0].Disabled)

	for _, item := range items[1:] {
		assert.False(t, item.Active, item.Title)
		assert.True(t, item.Disabled, item.Title)
	}
}
