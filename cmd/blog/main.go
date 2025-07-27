package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/nasermirzaei89/blog"
	"github.com/nasermirzaei89/env"
)

func main() {
	slog.SetLogLoggerLevel(getLogLevelFromEnv())

	slog.Info("starting app...")

	err := blog.Run(context.Background())
	if err != nil {
		slog.Error("failed to run application", "error", err)
		os.Exit(1)
	}

	slog.Info("app ran successfully")
}

func getLogLevelFromEnv() slog.Level {
	levelStr := env.GetString("LOG_LEVEL", "info")
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		slog.Warn("unknown log level, defaulting to info", "level", levelStr)
		return slog.LevelInfo
	}
}
