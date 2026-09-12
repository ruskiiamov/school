package static

import "embed"

//go:embed css/app.css js/htmx.min.js js/app.js favicon.svg
var FS embed.FS
