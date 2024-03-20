package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/bwmarrin/snowflake"
	"github.com/duke605/tv-bot/moviedb"
	"github.com/duke605/tv-bot/utils"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sarulabs/di"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"go.uber.org/ratelimit"
	"golang.org/x/oauth2"
)

var srvCtn di.Container

const (
	SrvCtnKeyDatabase          string = "database"
	SrvCtnKeyViper             string = "viper"
	SrvCtnKeyMovieDBClient     string = "moviedbClient"
	SrvCtnKeyDiscord           string = "discord"
	SrvCtnKeySubsRepo          string = "subsRepo"
	SrvCtnKeySnowflakeGen      string = "snowflakeGenerator"
	SrvCtnKeyNotificationsRepo string = "notificationsRepo"
	SrvCtnKeySeriesSrv         string = "seriesService"
	SrvCtnKeySubsSrv           string = "subsService"
	SrvCtnKeyDiscordCommandSrv string = "discordCommandService"
	SrvCtnKeySeriesRepo        string = "seriesRepo"
)

func init() {
	builder := utils.Must(di.NewBuilder())
	defer func() {
		srvCtn = builder.Build()
	}()

	if err := builder.Add(di.Def{
		Name: SrvCtnKeyDatabase,
		Close: func(obj interface{}) error {
			return (obj.(*sqlx.DB)).Close()
		},
		Build: func(ctn di.Container) (interface{}, error) {
			viper := ctn.Get(SrvCtnKeyViper).(*viper.Viper)
			connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", viper.GetString("db.file"))

			return sqlx.Connect("sqlite3", connStr)
		},
	}, di.Def{
		Name: SrvCtnKeyViper,
		Build: func(ctn di.Container) (interface{}, error) {
			v := viper.New()
			v.SetConfigFile(".env.yaml")
			v.AutomaticEnv()
			if err := v.ReadInConfig(); err != nil {
				return nil, err
			}

			if err := viper.MergeConfigMap(v.AllSettings()); err != nil {
				return nil, err
			}

			return v, nil
		},
	}, di.Def{
		Name: SrvCtnKeyMovieDBClient,
		Build: func(ctn di.Container) (interface{}, error) {
			viper := ctn.Get(SrvCtnKeyViper).(*viper.Viper)
			t := oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: viper.GetString("moviedb.access_token"),
				TokenType:   "bearer",
			})
			httpClient := oauth2.NewClient(context.Background(), t)
			rl := ratelimit.New(2)

			return moviedb.NewClient(viper.GetString("moviedb.base_url"),
				moviedb.ClientOptionWithHTTPClient(httpClient),
				moviedb.ClientOptionGlobalRequestOption(func(r *http.Request) *http.Request {
					slog.Debug("Hitting TheMovieDB endpoint", slog.String("url", r.URL.String()))
					return r
				}),
				moviedb.ClientOptionGlobalRequestOption(func(r *http.Request) *http.Request {
					rl.Take()
					return r
				}),
			)
		},
	}, di.Def{
		Name: SrvCtnKeyDiscord,
		Build: func(ctn di.Container) (interface{}, error) {
			viper := ctn.Get(SrvCtnKeyViper).(*viper.Viper)
			d, err := discordgo.New("Bot " + viper.GetString("discord.access_token"))
			if err != nil {
				return nil, err
			}

			d.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
				fmt.Println("Discord bot ready!")
			})

			return d, nil
		},
	}, di.Def{
		Name: "subsRepo",
		Build: func(ctn di.Container) (interface{}, error) {
			db := ctn.Get(SrvCtnKeyDatabase).(*sqlx.DB)

			return NewSubscriptionsRepo(db), nil
		},
	}, di.Def{
		Name: SrvCtnKeySnowflakeGen,
		Build: func(ctn di.Container) (interface{}, error) {
			return snowflake.NewNode(1)
		},
	}, di.Def{
		Name: SrvCtnKeyNotificationsRepo,
		Build: func(ctn di.Container) (interface{}, error) {
			db := ctn.Get(SrvCtnKeyDatabase).(*sqlx.DB)

			return NewNotificationsRepo(db), nil
		},
	}, di.Def{
		Name: SrvCtnKeySeriesSrv,
		Build: func(ctn di.Container) (interface{}, error) {
			notificationsRepo := ctn.Get(SrvCtnKeyNotificationsRepo).(*NotificationsRepo)
			subSrv := ctn.Get(SrvCtnKeySubsSrv).(*SubscriptionsService)
			discord := ctn.Get(SrvCtnKeyDiscord).(*discordgo.Session)
			moviedbClient := ctn.Get(SrvCtnKeyMovieDBClient).(moviedb.Client)
			seriesRepo := ctn.Get(SrvCtnKeySeriesRepo).(*SeriesRepo)

			return NewSeriesService(notificationsRepo, subSrv, discord, moviedbClient, seriesRepo), nil
		},
	}, di.Def{
		Name: SrvCtnKeySubsSrv,
		Build: func(ctn di.Container) (interface{}, error) {
			subsRepo := ctn.Get(SrvCtnKeySubsRepo).(*SubscriptionsRepo)

			return NewSubscriptionsService(subsRepo), nil
		},
	}, di.Def{
		Name: SrvCtnKeyDiscordCommandSrv,
		Build: func(ctn di.Container) (interface{}, error) {
			ctn.Get(SrvCtnKeyViper)
			discord := ctn.Get(SrvCtnKeyDiscord).(*discordgo.Session)
			seriesSrv := ctn.Get(SrvCtnKeySeriesSrv).(*SeriesService)
			subsService := ctn.Get(SrvCtnKeySubsSrv).(*SubscriptionsService)

			return NewDiscordCommandService(discord, seriesSrv, subsService), nil
		},
	}, di.Def{
		Name: SrvCtnKeySeriesRepo,
		Build: func(ctn di.Container) (interface{}, error) {
			db := ctn.Get(SrvCtnKeyDatabase).(*sqlx.DB)

			return NewSeriesRepo(db), nil
		},
	}); err != nil {
		panic(err)
	}
}

func init() {
	f := utils.NewDateFile(afero.NewOsFs(), "log.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	l := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(l))
}

func main() {
	defer srvCtn.DeleteWithSubContainers()
	rootCommand.Execute()
}
