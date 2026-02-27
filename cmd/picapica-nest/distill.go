package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ameyamatmk/picapica-nest/internal/distill"
	"github.com/sipeed/picoclaw/pkg/config"
)

func cmdDistill(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: picapica-nest distill <daily|weekly|monthly>")
	}

	switch args[0] {
	case "daily":
		return cmdDistillDaily(args[1:])
	default:
		return fmt.Errorf("unknown distill subcommand: %s", args[0])
	}
}

func cmdDistillDaily(args []string) error {
	fs := flag.NewFlagSet("distill daily", flag.ExitOnError)
	dateStr := fs.String("date", "", "対象日 (YYYY-MM-DD)。未指定時は前日")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	var targetDate time.Time
	if *dateStr == "" {
		targetDate = time.Now().AddDate(0, 0, -1)
	} else {
		targetDate, err = time.Parse("2006-01-02", *dateStr)
		if err != nil {
			return fmt.Errorf("invalid date format: %w", err)
		}
	}

	workspace := cfg.WorkspacePath()
	params := distill.DailyParams{
		Date:       targetDate,
		LogsDir:    filepath.Join(workspace, "logs"),
		OutputDir:  filepath.Join(workspace, "memory", "daily"),
		PromptPath: filepath.Join(workspace, "prompts", "daily_distill.md"),
	}

	fmt.Printf("Running daily distillation for %s\n", targetDate.Format("2006-01-02"))
	return distill.RunDaily(context.Background(), params)
}
