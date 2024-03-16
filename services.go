package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/duke605/tv-bot/moviedb"
	"github.com/duke605/tv-bot/utils"
	"github.com/spf13/viper"
)

type SeriesService struct {
	seriesRepo    *SeriesRepo
	notiRepo      *NotificationsRepo
	subsRepo      *SubscriptionsRepo
	discord       *discordgo.Session
	movieDBClient moviedb.Client
}

func NewSeriesService(sr *SeriesRepo, nr *NotificationsRepo, subr *SubscriptionsRepo, d *discordgo.Session, mdbc moviedb.Client) *SeriesService {
	return &SeriesService{
		seriesRepo:    sr,
		notiRepo:      nr,
		subsRepo:      subr,
		discord:       d,
		movieDBClient: mdbc,
	}
}

// FindNewEpisodes finds new episodes for all the series subscribed to in the database
// and notifies all subscribers about them in the discord
func (srv *SeriesService) FindNewEpisodes(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	seriesPager := srv.seriesRepo.List(ctx)
	now := time.Now()
	var subscribersIDs []uint64
	outstanding := utils.NewDualBatcher(10, func(embeds []*discordgo.MessageEmbed, notifications []*Notification) error {
		return srv.sendEmbedsAndMakeNotifications(ctx, notifications, embeds, subscribersIDs)
	})

	for {
		seriesModel, err := seriesPager.Next()
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		logger := slog.With("series_id", seriesModel.ID, "series_name", seriesModel.Name)

		// Getting subscribers to the series and skipping if none
		subscribersIDs, err = srv.subsRepo.GetAllSubscribedToSeries(ctx, seriesModel.ID)
		if err != nil {
			return err
		}
		if len(subscribersIDs) == 0 {
			logger.InfoContext(ctx, "No subscribers for series", "series_id", seriesModel.ID, "series_name", seriesModel.Name)
			continue
		}

		series := moviedb.SeriesDetails{}
		_, err = srv.movieDBClient.GetTVSeriesDetails(seriesModel.ID, &series,
			moviedb.RequestOptionWithQueryParams("language", "en-US"),
			moviedb.RequestOptionWithContext(ctx),
		)
		if err != nil {
			return err
		}

		season := moviedb.SeasonDetails{}
		_, err = srv.movieDBClient.GetTVSeasonDetails(seriesModel.ID, series.NumberOfSeasons, &season,
			moviedb.RequestOptionWithQueryParams("language", "en-US"),
			moviedb.RequestOptionWithContext(ctx),
		)
		if err != nil {
			return err
		}
		logger = logger.With("season_number", season.SeasonNumber)

		// Making a list of episodes that we haven't notified discord about
		for _, episode := range season.Episodes {
			// Checking if the episode has aired yet or the episode aired before we started listening
			if episode.AirDate == "" {
				continue
			}
			releaseDate, err := time.ParseInLocation(time.DateOnly, episode.AirDate, now.Location())
			if err != nil || releaseDate.After(now) || releaseDate.Before(seriesModel.CreatedAt) {
				continue
			}

			// Checking if we've already notified discord about this episode
			exists, err := srv.notiRepo.ExistsForEpisodeSeasonAndSeries(ctx, episode.EpisodeNumber, season.SeasonNumber, series.ID)
			if err != nil {
				return err
			} else if exists {
				continue
			}

			embed := srv.makeEmbedForEpisode(&series, &season, &episode, subscribersIDs)
			noti := &Notification{
				Episode:  episode.EpisodeNumber,
				Season:   episode.SeasonNumber,
				SeriesID: series.ID,
			}
			logger.InfoContext(ctx, "New episode found",
				"episode_id", episode.EpisodeNumber,
				"episode_type", episode.EpisodeType,
			)
			if err := outstanding.Add(embed, noti); err != nil {
				return err
			}
		}

		if err = outstanding.Flush(); err != nil {
			return err
		}
	}
}

func (SeriesService) makeEmbedForEpisode(
	series *moviedb.SeriesDetails,
	season *moviedb.SeasonDetails,
	episode *moviedb.EpisodeDetails,
	subscriberIDs []uint64,
) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Season",
				Value:  strconv.FormatInt(int64(season.SeasonNumber), 10),
				Inline: true,
			},
			{
				Name:   "Episode",
				Value:  strconv.FormatInt(int64(episode.EpisodeNumber), 10),
				Inline: true,
			},
			{
				Name:   "Runtime",
				Value:  HumanDuration(time.Minute * time.Duration(episode.Runtime)),
				Inline: true,
			},
			{
				Name:   "Wathers",
				Inline: true,
				Value: strings.Join(utils.Map(subscriberIDs, func(sID uint64, _ int) string {
					return fmt.Sprintf("<@%d>", sID)
				}), " "),
			},
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name: series.Name,
		},
		Title:       episode.Name,
		Description: episode.Overview,
	}

	if series.PosterPath != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			Width: 300,
			URL:   fmt.Sprintf("https://image.tmdb.org/t/p/w300/%s", series.PosterPath),
		}
	}

	if episode.StillPath != "" {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: fmt.Sprintf("https://image.tmdb.org/t/p/w780/%s", episode.StillPath),
		}
	}

	if series.Homepage != "" {
		embed.Author.URL = series.Homepage
	}

	if len(series.Networks) > 0 {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: series.Networks[0].Name,
		}

		if series.Networks[0].LogoPath != "" {
			embed.Footer.IconURL = fmt.Sprintf("https://image.tmdb.org/t/p/w45/%s", series.Networks[0].LogoPath)
		}
	}

	return embed
}

func (srv *SeriesService) sendEmbedsAndMakeNotifications(
	ctx context.Context,
	notifications []*Notification,
	embeds []*discordgo.MessageEmbed,
	subscriberIDs []uint64,
) error {
	channelID := viper.GetString("discord.notifications_channel_id")
	data := &discordgo.MessageSend{
		Embeds: embeds,
		Content: strings.Join(utils.Map(subscriberIDs, func(sID uint64, _ int) string {
			return fmt.Sprintf("<@%d>", sID)
		}), " "),
	}

	m, err := srv.discord.ChannelMessageSendComplex(channelID, data)
	if err != nil {
		return err
	}

	for _, n := range notifications {
		id, err := strconv.ParseUint(m.ID, 10, 64)
		if err != nil {
			return err
		}

		n.DiscordMessageID = id
	}

	return srv.notiRepo.InsertMany(ctx, notifications)
}

// AddToWatchlistByID adds a series by ID to watch for new episodes
func (srv *SeriesService) AddToWatchlistByID(ctx context.Context, seriesID uint64) (*Series, error) {
	details := moviedb.SeriesDetails{}
	_, err := srv.movieDBClient.GetTVSeriesDetails(seriesID, &details,
		moviedb.RequestOptionWithQueryParams("language", "en-US"),
		moviedb.RequestOptionWithContext(ctx),
	)
	if err != nil {
		return nil, err
	}

	seriesModel := Series{
		ID:         seriesID,
		Name:       details.Name,
		PosterPath: NewNull(details.PosterPath, details.PosterPath != ""),
		CreatedAt:  time.Now(),
	}

	err = srv.seriesRepo.Upsert(ctx, &seriesModel, "poster_path")
	if err != nil {
		return nil, err
	}

	return &seriesModel, nil
}

func (srv *SeriesService) IsOnWatchlist(ctx context.Context, seriesID uint64) (bool, error) {
	err := srv.seriesRepo.Find(ctx, seriesID, &Series{})
	if errors.Is(sql.ErrNoRows, err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}
