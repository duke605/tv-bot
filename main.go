package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/duke605/tv-bot/moviedb"
	"github.com/duke605/tv-bot/utils"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

var database *sqlx.DB
var moviedbClient moviedb.Client
var discord *discordgo.Session
var seriesService *SeriesService
var seriesRepo *SeriesRepo
var subsRepo *SubscriptionsRepo
var notificationsRepo *NotificationsRepo

// var snowflakes *snowflake.Node
var loaded = map[string]bool{}

func loadEnv() error {
	if ok, err := LoadOrNoop("config"); !ok || err != nil {
		return err
	}

	viper.SetConfigFile(".env.yaml")
	viper.AutomaticEnv()
	return viper.ReadInConfig()
}

func loadDatabase() error {
	if ok, err := LoadOrNoop("database", loadEnv); !ok || err != nil {
		return err
	}

	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", viper.GetString("db.file"))
	db, err := sqlx.Connect("sqlite3", connStr)
	if err != nil {
		return err
	}

	database = db
	return err
}

func loadMovieDBClient() error {
	if ok, err := LoadOrNoop("moviedbClient", loadEnv); !ok || err != nil {
		return err
	}

	t := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: viper.GetString("moviedb.access_token"),
		TokenType:   "bearer",
	})
	httpClient := oauth2.NewClient(context.Background(), t)
	c, err := moviedb.NewClient(viper.GetString("moviedb.base_url"), moviedb.ClientOptionWithHTTPClient(httpClient))
	if err != nil {
		return err
	}

	moviedbClient = c
	return nil
}

func loadDiscord() error {
	if ok, err := LoadOrNoop("discord", loadEnv); !ok || err != nil {
		return err
	}

	d, err := discordgo.New("Bot " + viper.GetString("discord.access_token"))
	if err != nil {
		return err
	}

	discord = d
	return nil
}

func loadSubsRepo() error {
	if ok, err := LoadOrNoop("subsRepo", loadDatabase); !ok || err != nil {
		return err
	}

	subsRepo = NewSubscriptionsRepo(database)
	return nil
}

func loadSeriesRepo() error {
	if ok, err := LoadOrNoop("seriesRepo", loadDatabase); !ok || err != nil {
		return err
	}

	seriesRepo = NewSeriesRepo(database)
	return nil
}

// func loadSnowflakes() error {
// 	if ok, err := LoadOrNoop("snowflakes"); !ok || err != nil {
// 		return err
// 	}

// 	s, err := snowflake.NewNode(1)
// 	if err != nil {
// 		return err
// 	}

// 	snowflakes = s
// 	return nil
// }

func loadNotificationsRepo() error {
	if ok, err := LoadOrNoop("notificationsRepo", loadDatabase); !ok || err != nil {
		return err
	}

	notificationsRepo = NewNotificationsRepo(database)
	return nil
}

func loadSeriesService() error {
	if ok, err := LoadOrNoop("seriesService", loadSeriesRepo, loadMovieDBClient, loadDiscord, loadNotificationsRepo, loadSubsRepo); !ok || err != nil {
		return err
	}

	seriesService = NewSeriesService(seriesRepo, notificationsRepo, subsRepo, discord, moviedbClient)
	return nil
}

func init() {
	f := utils.NewDateFile(afero.NewOsFs(), "log.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	l := slog.NewJSONHandler(f, nil)
	slog.SetDefault(slog.New(l))
}

func main() {
	defer func() {
		if database != nil {
			database.Close()
		}
	}()
	rootCommand.Execute()
}
