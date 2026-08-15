package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/ehssanehs/arp-responder/internal/config"
)

func New(c config.Config) (*slog.Logger, io.Closer, error) {
	var w io.Writer = os.Stdout
	var closer io.Closer
	if c.Logging.Output == "file" {
		f, err := os.OpenFile(c.Logging.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
		if err != nil { return nil, nil, fmt.Errorf("open log file: %w", err) }
		w, closer = f, f
	}
	level := slog.LevelInfo
	switch strings.ToLower(c.LogLevel) { case "warning", "warn": level = slog.LevelWarn; case "error": level = slog.LevelError }
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})), closer, nil
}
