package controllers

import (
	"fmt"
	"log/slog"
	"os"
)

func NewLogger(levelString string) (slog.Handler, error) {
	var (
		lvlvar slog.LevelVar
	)
	err := lvlvar.UnmarshalText([]byte(levelString))
	if err != nil {
		return nil, fmt.Errorf("can't initialize logger: %w", err)
	}
	level := lvlvar.Level()
	return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}), nil
}
