package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/duke605/tv-bot/utils"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/robfig/cron"
	"github.com/spf13/cobra"
)

var rootCommand = &cobra.Command{
	Use:   filepath.Base(os.Args[0]),
	Short: "Starts the discord bot",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		defer utils.ReturnPanic(&err)

		discordCommandService := srvCtn.Get(SrvCtnKeyDiscordCommandSrv).(*DiscordCommandService)
		discord := srvCtn.Get(SrvCtnKeyDiscord).(*discordgo.Session)
		seriesService := srvCtn.Get(SrvCtnKeySeriesSrv).(*SeriesService)

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		discordCommandService.RegisterHandlers(ctx)
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

		c.Start()
		defer c.Stop()

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		fmt.Println("Exiting!")

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
		return goose.SetDialect("sqlite3")
	},
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		defer utils.ReturnPanic(&err)

		database := srvCtn.Get(SrvCtnKeyDatabase).(*sqlx.DB)

		goose.SetBaseFS(migrationFS)

		return goose.Up(database.DB, "migrations")
	},
}

var makeMigrationCommand = &cobra.Command{
	Use:   "make:migration",
	Short: "Create writes a new blank migration file.",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return goose.SetDialect("sqlite3")
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
		return goose.SetDialect("sqlite3")
	},
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		defer utils.ReturnPanic(&err)

		database := srvCtn.Get(SrvCtnKeyDatabase).(*sqlx.DB)

		goose.SetBaseFS(migrationFS)

		return goose.Down(database.DB, "migrations")
	},
}

var registerDiscordCommandsCommand = &cobra.Command{
	Use:   "discord:commands:register",
	Short: "Registers discord commands",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		defer utils.ReturnPanic(&err)

		discordCommandService := srvCtn.Get(SrvCtnKeyDiscordCommandSrv).(*DiscordCommandService)

		if err := discordCommandService.RegisterCommands(cmd.Context()); err != nil {
			return err
		}

		fmt.Println("Registered commands successfully")
		return nil
	},
}

func init() {
	rootCommand.AddCommand(
		migrateCommand,
		makeMigrationCommand,
		rollbackMigrationCommand,
		registerDiscordCommandsCommand,
	)
}
