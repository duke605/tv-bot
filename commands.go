package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
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
	"github.com/spf13/viper"
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
		viper := srvCtn.Get(SrvCtnKeyViper).(*viper.Viper)

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		discordCommandService.RegisterHandlers(ctx)
		if err := discord.Open(); err != nil {
			return err
		}
		defer discord.Close()

		c := cron.New()
		c.AddFunc("0,30 * * * *", func() {
			start := time.Now()
			slog.InfoContext(ctx, "Finding new episodes for series on watchlist")
			if err := seriesService.FindNewEpisodes(ctx); err != nil {
				slog.ErrorContext(ctx, "Error occurred while finding new episodes", "error", err)
				return
			}
			slog.InfoContext(ctx, "Finished looking for new episodes", "duration", time.Since(start).String())
		})

		c.Start()
		defer c.Stop()

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		http.HandleFunc("GET /shutdown", func(w http.ResponseWriter, r *http.Request) {
			slog.InfoContext(ctx, "Received shutdown command from HTTP server")
			fmt.Fprintln(w, "Shutting down...")
			signal.Stop(sigs)
			sigs <- syscall.SIGTERM
		})

		server := http.Server{Addr: viper.GetString("addr")}
		go server.ListenAndServe()
		httpCtx, _ := context.WithTimeout(context.Background(), time.Second*5)
		defer server.Shutdown(httpCtx)

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

var findNewEpisodesCommand = &cobra.Command{
	Use:   "episodes:find_new",
	Short: "Finds new episodes for all subscribed series",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		defer utils.ReturnPanic(&err)
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		seriesSrv := srvCtn.Get(SrvCtnKeySeriesSrv).(*SeriesService)

		start := time.Now()
		if err := seriesSrv.FindNewEpisodes(ctx); err != nil {
			return err
		}

		fmt.Println("Finished looking for new episodes. Took", time.Since(start).String())
		return nil
	},
}

var deleteEpisodeNotificationsCommand = &cobra.Command{
	Use:   "episodes:delete_notifications",
	Short: "Deletes notifications for episode releases from the database so notifications can be sent again",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		defer utils.ReturnPanic(&err)
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		notiRepo := srvCtn.Get(SrvCtnKeyNotificationsRepo).(*NotificationsRepo)

		start := time.Now()
		n, err := notiRepo.DeleteAllNotifications(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("Finished deleting %d notification(s). Took %s\n", n, time.Since(start).String())
		return nil
	},
}

func init() {
	rootCommand.AddCommand(
		migrateCommand,
		makeMigrationCommand,
		rollbackMigrationCommand,
		registerDiscordCommandsCommand,
		findNewEpisodesCommand,
		deleteEpisodeNotificationsCommand,
	)
}
