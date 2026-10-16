package logger

import (
	"io"
	"log/slog"

	"github.com/alice101-dev/ai-code-reviewer/internal/config"
)

func New(w io.Writer, cfg config.Log, app config.App) *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.Level))

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	return slog.New(handler).With(
		slog.String("service", app.Name),
		slog.String("env", app.Env),
	)
}
