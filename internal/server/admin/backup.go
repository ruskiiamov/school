package admin

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/ruskiiamov/school/internal/view"
	"github.com/ruskiiamov/school/internal/view/pages"
)

const (
	backupPath         = "/admin/backup"
	backupDownloadPath = backupPath + "/download"
	backupTitle        = "Резервная копия"
	backupContentType  = "application/gzip"
)

func (h *Handler) backupShow(w http.ResponseWriter, r *http.Request) {
	usage, err := h.backup.Usage()
	if err != nil {
		h.base.ServerError(w, r, "measure homework files", err)
		return
	}

	page := view.BackupPage{
		Shell:        h.base.Shell(r, backupTitle, backupPath),
		DownloadPath: backupDownloadPath,
		Files:        view.Plural(usage.FileCount, "файл", "файла", "файлов") + ", " + view.FormatFileSize(usage.FileBytes),
	}

	h.base.Render(w, r, pages.Backup(page))
}

func (h *Handler) backupDownload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	archive, err := h.backup.Prepare(ctx)
	if err != nil {
		h.base.ServerError(w, r, "prepare backup", err)
		return
	}
	defer func() {
		if err := archive.Close(); err != nil {
			h.log.ErrorContext(ctx, "clean up backup", slog.Any("error", err))
		}
	}()

	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(h.transferTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.base.ServerError(w, r, "extend write deadline", err)
		return
	}

	name := "school-backup-" + h.school.Today().Format("2006-01-02") + ".tar.gz"

	w.Header().Set("Content-Type", backupContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))

	written, err := archive.WriteTo(w)
	if err != nil {
		h.log.ErrorContext(ctx, "write backup", slog.Any("error", err), slog.Int64("bytes", written))
		return
	}

	h.log.InfoContext(ctx, "backup downloaded", slog.Int64("bytes", written))
}
