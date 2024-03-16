package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/robfig/cron"
	"github.com/spf13/cobra"
)

var rootCommand = &cobra.Command{
	Use:   filepath.Base(os.Args[0]),
	Short: "Starts the discord bot",
	Args:  cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return TryAll(loadDiscord, loadSeriesService)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())

		defer cancel()

		if err := discord.Open(); err != nil {
			return err
		}
		defer discord.Close()

		c := cron.New()
		c.AddFunc("0 30 * * * *", func() {
			start := time.Now()
			slog.InfoContext(ctx, "Finding new episodes for series on watchlist")
			if err := seriesService.FindNewEpisodes(ctx); err != nil {
				slog.ErrorContext(ctx, "Error occurred while finding new episodes", "error", err)
			}
			slog.InfoContext(ctx, "Finished looking for new episodes", "duration", time.Since(start).String())
		})

		c.Run()
		defer c.Stop()

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs

		return nil
	},
}

//go:embed migrations/*.sql
var migrationFS embed.FS

var migrateCommand = &cobra.Command{
	Use:   "migrate",
	Short: "Applies all available migrations.",
	Args:  cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return TryAll(loadDatabase, setGooseDialect)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		goose.SetBaseFS(migrationFS)

		return goose.Up(database.DB, "migrations")
	},
}

var makeMigrationCommand = &cobra.Command{
	Use:   "make:migration",
	Short: "Create writes a new blank migration file.",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return TryAll(loadDatabase, setGooseDialect)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		migrationName := args[0]

		return goose.Create(nil, "migrations", migrationName, "sql")
	},
}

var rollbackMigrationCommand = &cobra.Command{
	Use:   "migrate:rollback",
	Short: "Rolls back a single migration from the current version.",
	Args:  cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return TryAll(loadDatabase, setGooseDialect)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		goose.SetBaseFS(migrationFS)

		return goose.Down(database.DB, "migrations")
	},
}

var watchlistAddCommand = &cobra.Command{
	Use:   "watchlist:add",
	Short: "Adds a series to the watchlist",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return TryAll(loadSeriesService, loadSeriesRepo)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return errors.New("expected int")
		}

		// Checking if the series is already on the watch list
		s := &Series{}
		err = seriesRepo.Find(cmd.Context(), id, s)
		if err != nil && !errors.Is(sql.ErrNoRows, err) {
			return err
		}
		if s.ID == id {
			fmt.Printf("'%s' is already on the watchlist\n", s.Name)
			return nil
		}

		s, err = seriesService.AddToWatchlistByID(cmd.Context(), id)
		if err != nil {
			return err
		}

		fmt.Printf("'%s' has been added to the watchlist\n", s.Name)
		return nil
	},
}

var watchlistRemoveCommand = &cobra.Command{
	Use:   "watchlist:remove",
	Short: "Removes a series from the watchlist",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return TryAll(loadSeriesRepo, loadSeriesService)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return errors.New("expected int")
		}

		// Checking if the series is already on the watch list
		s := &Series{}
		err = seriesRepo.Find(cmd.Context(), id, s)
		if err != nil && !errors.Is(sql.ErrNoRows, err) {
			return err
		}
		if yes, err := seriesService.IsOnWatchlist(cmd.Context(), id); err != nil {
			return err
		} else if !yes {
			fmt.Printf("No series with id '%d' found on the watchlist\n", id)
			return nil
		}

		err = seriesRepo.Delete(cmd.Context(), id)
		if err != nil {
			return err
		}

		fmt.Printf("'%s' has been removed from the watchlist\n", s.Name)
		return nil
	},
}

func init() {
	rootCommand.AddCommand(
		migrateCommand,
		makeMigrationCommand,
		rollbackMigrationCommand,
		watchlistAddCommand,
		watchlistRemoveCommand,
	)
}
