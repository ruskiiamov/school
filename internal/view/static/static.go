package static

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"sync"
)

//go:embed css/app.css js/htmx.min.js js/app.js favicon.svg
var FS embed.FS

const Prefix = "/static/"

var Version = sync.OnceValue(func() string {
	digest := sha256.New()

	err := fs.WalkDir(FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		content, err := FS.ReadFile(path)
		if err != nil {
			return err
		}

		digest.Write([]byte(path))
		digest.Write(content)

		return nil
	})
	if err != nil {
		panic("hash embedded static files: " + err.Error())
	}

	return hex.EncodeToString(digest.Sum(nil))[:8]
})

func URL(name string) string {
	return Prefix + Version() + "/" + name
}
