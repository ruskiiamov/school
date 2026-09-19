package web_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ruskiiamov/school/internal/config"
	"github.com/ruskiiamov/school/internal/server/web"
)

func TestClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		behindProxy bool
		forwarded   []string
		want        string
	}{
		{name: "direct connection ignores the header", forwarded: []string{"203.0.113.7"}, want: "192.0.2.1"},
		{name: "proxy without the header", behindProxy: true, want: "192.0.2.1"},
		{name: "proxy sets the address", behindProxy: true, forwarded: []string{"203.0.113.7"}, want: "203.0.113.7"},
		{name: "client-supplied prefix is ignored", behindProxy: true, forwarded: []string{"10.0.0.1, 203.0.113.7"}, want: "203.0.113.7"},
		{name: "last header line wins", behindProxy: true, forwarded: []string{"10.0.0.1", "2001:db8::7"}, want: "2001:db8::7"},
		{name: "garbage falls back to the connection", behindProxy: true, forwarded: []string{"not-an-ip"}, want: "192.0.2.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{HTTP: config.HTTP{BehindProxy: tt.behindProxy}}
			base := web.New(cfg, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

			req := httptest.NewRequest(http.MethodPost, "/login", nil)
			for _, value := range tt.forwarded {
				req.Header.Add("X-Forwarded-For", value)
			}

			assert.Equal(t, tt.want, base.ClientIP(req))
		})
	}
}
