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
	case "weekly":
		return cmdDistillWeekly(args[1:])
	case "monthly":
		return cmdDistillMonthly(args[1:])
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

func cmdDistillWeekly(args []string) error {
	fs := flag.NewFlagSet("distill weekly", flag.ExitOnError)
	weekStr := fs.String("week", "", "対象週 (YYYY-WNN)。未指定時は前週（前の土曜〜金曜）")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	var year, week int
	if *weekStr == "" {
		// 前週: 直近の金曜日が属する ISO 週
		now := time.Now()
		// 直近の金曜日を見つける（土曜朝に実行される想定）
		daysBack := int(now.Weekday()-time.Friday+7) % 7
		if daysBack == 0 {
			daysBack = 7 // 金曜日当日なら前週の金曜
		}
		lastFriday := now.AddDate(0, 0, -daysBack)
		year, week = lastFriday.ISOWeek()
	} else {
		_, err := fmt.Sscanf(*weekStr, "%d-W%d", &year, &week)
		if err != nil {
			return fmt.Errorf("invalid week format (expected YYYY-WNN): %w", err)
		}
	}

	workspace := cfg.WorkspacePath()
	params := distill.WeeklyParams{
		Year:       year,
		Week:       week,
		DailyDir:   filepath.Join(workspace, "memory", "daily"),
		OutputDir:  filepath.Join(workspace, "memory", "weekly"),
		PromptPath: filepath.Join(workspace, "prompts", "weekly_distill.md"),
	}

	fmt.Printf("Running weekly distillation for %d-W%02d\n", year, week)
	return distill.RunWeekly(context.Background(), params)
}

func cmdDistillMonthly(args []string) error {
	fs := flag.NewFlagSet("distill monthly", flag.ExitOnError)
	monthStr := fs.String("month", "", "対象月 (YYYY-MM)。未指定時は前月")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	var year, month int
	if *monthStr == "" {
		// 前月
		now := time.Now()
		prevMonth := now.AddDate(0, -1, 0)
		year = prevMonth.Year()
		month = int(prevMonth.Month())
	} else {
		_, err := fmt.Sscanf(*monthStr, "%d-%d", &year, &month)
		if err != nil {
			return fmt.Errorf("invalid month format (expected YYYY-MM): %w", err)
		}
		if month < 1 || month > 12 {
			return fmt.Errorf("invalid month: %d (must be 1-12)", month)
		}
	}

	workspace := cfg.WorkspacePath()
	params := distill.MonthlyParams{
		Year:       year,
		Month:      month,
		WeeklyDir:  filepath.Join(workspace, "memory", "weekly"),
		DailyDir:   filepath.Join(workspace, "memory", "daily"),
		OutputDir:  filepath.Join(workspace, "memory", "monthly"),
		PromptPath: filepath.Join(workspace, "prompts", "monthly_distill.md"),
	}

	fmt.Printf("Running monthly distillation for %d-%02d\n", year, month)
	return distill.RunMonthly(context.Background(), params)
}
