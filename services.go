package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/duke605/tv-bot/moviedb"
	"github.com/duke605/tv-bot/utils"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/spf13/viper"
)

type SeriesService struct {
	notiRepo      *NotificationsRepo
	subsSrv       *SubscriptionsService
	discord       *discordgo.Session
	movieDBClient moviedb.Client

	seriesCacheByID *expirable.LRU[uint64, *moviedb.SeriesDetails]
	searchCache     *expirable.LRU[string, []utils.Tuple[string, uint64]]
}

func NewSeriesService(nr *NotificationsRepo, ss *SubscriptionsService, d *discordgo.Session, mdbc moviedb.Client) *SeriesService {
	return &SeriesService{
		notiRepo:        nr,
		subsSrv:         ss,
		discord:         d,
		movieDBClient:   mdbc,
		seriesCacheByID: expirable.NewLRU[uint64, *moviedb.SeriesDetails](100, nil, time.Minute*10),
		searchCache:     expirable.NewLRU[string, []utils.Tuple[string, uint64]](100, nil, time.Minute*10),
	}
}

// SearchSeries searches for series that partially or fully match the provided name and returns an array of tuples containing
// the name of the series and its id
func (srv *SeriesService) SearchSeries(ctx context.Context, name string) ([]utils.Tuple[string, uint64], error) {
	// Checking if IDs are in cache
	if ids, ok := srv.searchCache.Get(name); ok {
		return ids, nil
	}

	results := new(moviedb.SearchResults[*moviedb.SearchSeriesDetails])
	_, err := srv.movieDBClient.SearchTVSeriesDetails(name, results,
		moviedb.RequestOptionWithContext(ctx),
		moviedb.RequestOptionWithQueryParams("language", "en-US"),
	)
	if err != nil {
		return nil, err
	}

	if results.TotalResults == 0 {
		return []utils.Tuple[string, uint64]{}, nil
	}

	ret := make([]utils.Tuple[string, uint64], 0, len(results.Results))
	for _, s := range results.Results {
		date, err := time.Parse(time.DateOnly, s.FirstAirDate)
		if err != nil {
			continue
		}

		name := fmt.Sprintf("%s (%s)", s.Name, date.Format("2006"))
		ret = append(ret, utils.Tuple[string, uint64]{T: name, V: s.ID})
	}

	// Adding IDs to cache
	srv.searchCache.Add(name, ret)

	return ret, nil
}

// FindNewEpisodes finds new episodes for all the series subscribed to in the database
// and notifies all subscribers about them in the discord
func (srv *SeriesService) FindNewEpisodes(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	seriesPager := srv.subsSrv.GetDistinctSeriesIDsWithEpoch(ctx)
	now := time.Now()
	var subscribersIDs []uint64
	outstanding := utils.NewDualBatcher(10, func(embeds []*discordgo.MessageEmbed, notifications []*Notification) error {
		return srv.sendEmbedsAndMakeNotifications(ctx, notifications, embeds, subscribersIDs)
	})
	finishedSeries := []*moviedb.SeriesDetails{}

	for {
		row, more, err := seriesPager.Next()
		if err != nil {
			return err
		} else if !more {
			break
		}
		seriesID, epoch := row.T, row.V
		logger := slog.With("series_id", seriesID)
		logger.DebugContext(ctx, "Looking for new episodes for series")

		// Getting subscribers to the series and skipping if none
		subscribersIDs, err = srv.subsSrv.GetAllSubscribedToSeries(ctx, seriesID)
		if err != nil {
			return err
		}
		if len(subscribersIDs) == 0 {
			logger.WarnContext(ctx, "No subscribers for series")
			continue
		}

		series := &moviedb.SeriesDetails{}
		_, err = srv.movieDBClient.GetTVSeriesDetails(seriesID, series,
			moviedb.RequestOptionWithQueryParams("language", "en-US"),
			moviedb.RequestOptionWithContext(ctx),
		)
		if err != nil {
			return err
		}
		srv.seriesCacheByID.Add(series.ID, series)

		// Adding the series to the finished slice to inform subscribers that a series
		// they subscribe to has ended or been cancelled
		if series.Status == "Ended" || series.Status == "Canceled" || series.Status == "Cancelled" {
			finishedSeries = append(finishedSeries, series)
		}

		season := moviedb.SeasonDetails{}
		_, err = srv.movieDBClient.GetTVSeasonDetails(series.ID, series.NumberOfSeasons, &season,
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
			if err != nil || releaseDate.After(now) || releaseDate.Before(epoch) {
				continue
			}

			// Not showing notification if it's missing a description, runtime, or still path unless the time is past 8:00PM
			if (episode.Overview == "" || episode.StillPath == "" || episode.Runtime == 0) && time.Now().Hour() < 20 {
				slog.DebugContext(ctx, "Found new episode but has missing information. Delaying notification...",
					slog.String("series", series.Name),
					slog.Uint64("series_id", series.ID),
					slog.Int("episode", episode.EpisodeNumber),
					slog.Int("season", episode.SeasonNumber),
					slog.Bool("missing_overview", episode.Overview == ""),
					slog.Bool("missing_runtime", episode.Runtime == 0),
					slog.Bool("missing_still_path", episode.StillPath == ""),
				)
				continue
			}

			// Checking if we've already notified discord about this episode
			exists, err := srv.notiRepo.ExistsForEpisodeSeasonAndSeries(ctx, episode.EpisodeNumber, season.SeasonNumber, series.ID)
			if err != nil {
				return err
			} else if exists {
				continue
			}

			embed := srv.makeEmbedForEpisode(series, &season, &episode, subscribersIDs)
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

	return srv.sendFinishedSeriesNotificationsAndUnsubscribeSubscribers(ctx, finishedSeries)
}

// GetSeriesDetails gets details about a series. Function will attempt to look for the details in the cache
// before going out to the internet.
func (srv *SeriesService) GetSeriesDetails(ctx context.Context, seriesID uint64) (*moviedb.SeriesDetails, error) {
	series, ok := srv.seriesCacheByID.Get(seriesID)
	if ok {
		return series, nil
	}

	series = new(moviedb.SeriesDetails)
	_, err := srv.movieDBClient.GetTVSeriesDetails(seriesID, series,
		moviedb.RequestOptionWithContext(ctx),
		moviedb.RequestOptionWithQueryParams("language", "en-US"),
	)
	if err != nil {
		return nil, err
	}
	srv.seriesCacheByID.Add(seriesID, series)

	return series, nil
}

func (srv *SeriesService) sendFinishedSeriesNotificationsAndUnsubscribeSubscribers(
	ctx context.Context,
	series []*moviedb.SeriesDetails,
) error {
	if len(series) == 0 {
		return nil
	}

	seriesIDs := make([]uint64, 0, len(series))
	cancelled := ""
	ended := ""

	for _, s := range series {
		seriesIDs = append(seriesIDs, s.ID)
		if s.Status == "Ended" {
			ended += "\n- " + s.Name
		} else {
			cancelled += "\n- " + s.Name
		}
	}
	cancelled = strings.Trim(cancelled, "\n")
	ended = strings.Trim(ended, "\n")

	fields := make([]*discordgo.MessageEmbedField, 0, 2)
	if cancelled != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Cancelled",
			Inline: true,
			Value:  strings.Trim(cancelled, "\n"),
		})
	}
	if ended != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Ended",
			Inline: true,
			Value:  strings.Trim(ended, "\n"),
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Series cancelled or ended",
		Description: "Unfortunately the following series have either ended or been cancelled :pensive:",
		Fields:      fields,
	}

	subscriberIDs, err := srv.subsSrv.GetAllSubscribedToSeries(ctx, seriesIDs...)
	if err != nil {
		slog.ErrorContext(ctx, "Failed getting all subscribers for series", "series", seriesIDs)
		return err
	}

	channelID := viper.GetString("discord.notifications_channel_id")
	srv.discord.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embed: embed,
		Content: utils.Reduce(subscriberIDs, func(a string, e uint64) string {
			return a + "\n" + fmt.Sprintf("<@%d>", e)
		}, ""),
	})

	if err = srv.subsSrv.DeleteSubscriptionsForSeries(ctx, seriesIDs...); err != nil {
		slog.ErrorContext(ctx, "Failed unsubscribing subscribers from series", "series", seriesIDs)
		return err
	}

	return nil
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
				Name:   "Watchers",
				Inline: true,
				Value: strings.Join(utils.MapSlice(subscriberIDs, func(sID uint64, _ int) string {
					return fmt.Sprintf("<@%d>", sID)
				}), " "),
			},
			{
				Name:   "Episode type",
				Inline: true,
				Value:  episode.EpisodeType,
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

	if episode.StillPath != "" || series.BackdropPath != "" {
		path := episode.StillPath
		if path == "" {
			path = series.BackdropPath
		}

		embed.Image = &discordgo.MessageEmbedImage{
			URL: fmt.Sprintf("https://image.tmdb.org/t/p/w780/%s", path),
		}
	}

	if series.Homepage != "" {
		embed.Author.URL = series.Homepage
	}

	if len(series.Networks) > 0 {
		networks := ""
		for _, n := range series.Networks {
			networks += " | " + n.Name
		}

		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: strings.Trim(networks, "| "),
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
		Content: strings.Join(utils.MapSlice(subscriberIDs, func(sID uint64, _ int) string {
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

type commandHandler = func(context.Context, *discordgo.Session, *discordgo.InteractionCreate)
type autocompleteHandler = func(context.Context, *discordgo.Session, *discordgo.InteractionCreate, *discordgo.ApplicationCommandInteractionDataOption)
type discordCommand struct {
	discordgo.ApplicationCommand
	Handle       commandHandler
	Autocomplete map[string]autocompleteHandler
}

func (dc *discordCommand) addToHandlersMap(m map[string]*discordCommand) {
	m[dc.Name] = dc
}

type DiscordCommandService struct {
	sess      *discordgo.Session
	commands  map[string]*discordCommand
	seriesSrv *SeriesService
	subsSrv   *SubscriptionsService
}

func NewDiscordCommandService(s *discordgo.Session, ss *SeriesService, sus *SubscriptionsService) *DiscordCommandService {
	srv := &DiscordCommandService{
		seriesSrv: ss,
		sess:      s,
		subsSrv:   sus,
		commands:  map[string]*discordCommand{},
	}

	(&discordCommand{
		ApplicationCommand: discordgo.ApplicationCommand{
			Name:         "subscribe",
			Description:  "Subscribes you to a series so you will be notified when a new episode of releases",
			DMPermission: PP(false),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "series",
					Autocomplete: true,
					Required:     true,
					Description:  "The series to unsubscribe from",
				},
			},
		},
		Handle: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})

			resp := utils.NewDiscordResponse(s, i)
			seriesOpt := i.ApplicationCommandData().Options[0]
			userID, _ := strconv.ParseUint(i.Member.User.ID, 10, 64)
			seriesID, err := strconv.ParseUint(seriesOpt.StringValue(), 10, 64)
			if err != nil {
				resp.SetWarning("").SetTitle("Series must be selected from the list").Edit()
				return
			}

			series, err := srv.seriesSrv.GetSeriesDetails(ctx, seriesID)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to get series information", "error", err)
				resp.SetError(err).SetTitle("Failed to look up information about series").Edit()
				return
			}

			// Checking if series has ended and removing all subscriptions for it if it has
			status := strings.ToLower(series.Status)
			if status == "canceled" || status == "ended" {
				srv.subsSrv.DeleteSubscriptionsForSeries(ctx, seriesID)
				slog.ErrorContext(ctx, "User tried to subscribe to a canceled/finished series", "series", series.Name, "user", i.Member.User.Username)
				resp.SetWarning("You cannot subscribe to a series that has ended or been canceled").SetTitlef("Series %s", status).Edit()
				return
			}

			isSubbed, err := srv.subsSrv.UserIsSubscribed(ctx, seriesID, userID)
			if err != nil {
				slog.ErrorContext(ctx, "Failed checking if user is subscribed", "user_id", userID, "series_id", seriesID)
				resp.SetError(err).SetTitle("Failed checking subscription status").Edit()
				return
			} else if isSubbed {
				resp.SetWarning("").SetTitle("You are already subscribed").Edit()
				return
			}

			if err = srv.subsSrv.SubscribeUserToSeries(ctx, seriesID, userID); err != nil {
				slog.ErrorContext(ctx, "Failed to subscribe user to series", "user_id", userID, "series_id", seriesID, "error", err)
				resp.SetError(err).SetTitle("Failed to subscribe you to series").Edit()
				return
			}

			imagePath := ""
			thumbnailPath := ""
			if series.BackdropPath != "" {
				imagePath, _ = url.JoinPath("https://image.tmdb.org/t/p/w780", series.BackdropPath)
			}
			if series.PosterPath != "" {
				thumbnailPath, _ = url.JoinPath("https://image.tmdb.org/t/p/w780", series.PosterPath)
			}
			resp.SetSuccess("You will now receive updates when new episodes release").
				SetImage(imagePath).
				SetThumbnail(thumbnailPath).
				SetTitlef("Successfully subscribed to '%s'", series.Name).
				Edit()
		},
		Autocomplete: map[string]autocompleteHandler{
			"series": srv.autocompleteForSeriesName,
		},
	}).addToHandlersMap(srv.commands)

	(&discordCommand{
		ApplicationCommand: discordgo.ApplicationCommand{
			Name:         "subscriptions",
			Description:  "Lists all series you are subscribed to",
			DMPermission: PP(false),
		},
		Handle: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})

			resp := utils.NewDiscordResponse(s, i)
			userID, _ := strconv.ParseUint(i.Member.User.ID, 10, 64)

			subs, err := srv.subsSrv.GetUserSubscriptions(ctx, userID)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to get user's subscriptions", "error", err)
				resp.SetError(err).SetTitle("Failed get your subscriptions").Edit()
				return
			}

			// Getting series details for all subscriptions
			series := map[string]string{}
			for _, sub := range subs {
				s, err := srv.seriesSrv.GetSeriesDetails(ctx, sub.SeriesID)
				if err != nil {
					slog.ErrorContext(ctx, "Failed to get series information for a user's subscription", "error", err, "subscription", sub.ToMap())
					resp.SetError(err).SetTitle("Failed get details on one of your subscriptions").Edit()
					return
				}

				date, _ := time.Parse(time.DateOnly, s.FirstAirDate)
				series[s.Status] += fmt.Sprintf("\n- %s (%s)", s.Name, date.Format("2006"))
			}

			for status, list := range series {
				status = strings.Trim(strings.ReplaceAll(status, "Series", ""), " ")
				resp.AddField(status+" series", list, false)
			}

			resp.SetInfo("").
				SetTitle("You subscriptions").
				Edit()
		},
	}).addToHandlersMap(srv.commands)

	(&discordCommand{
		ApplicationCommand: discordgo.ApplicationCommand{
			Name:         "unsubscribe",
			Description:  "Unsubscribes you from a series",
			DMPermission: PP(false),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "series",
					Autocomplete: true,
					Required:     true,
					Description:  "The series to unsubscribe from",
				},
			},
		},
		Handle: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {

		},
		Autocomplete: map[string]autocompleteHandler{
			"series": srv.autocompleteForSeriesName,
		},
	}).addToHandlersMap(srv.commands)

	return srv
}

func (srv *DiscordCommandService) RegisterHandlers(ctx context.Context) {
	srv.sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		commandName := i.ApplicationCommandData().Name
		command := srv.commands[commandName]
		if command == nil {
			slog.WarnContext(ctx, "Unknown command", "command", commandName)
			utils.NewDiscordResponse(s, i).SetWarning("").SetTitle("Unknown command").Respond()
			return
		}

		slog.InfoContext(ctx, "Received command", "command", commandName, "type", i.ApplicationCommandData().Type().String())
		if i.Type == discordgo.InteractionApplicationCommand {
			command.Handle(ctx, s, i)
		} else if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
			ctx, cancel := context.WithTimeout(ctx, time.Second*3)
			defer cancel()

			for _, opt := range i.ApplicationCommandData().Options {
				if opt.Focused {
					command.Autocomplete[opt.Name](ctx, s, i, opt)
					break
				}
			}
		}
	})
}

func (srv *DiscordCommandService) RegisterCommands(ctx context.Context) error {
	appID := viper.GetString("discord.client_id")
	serverID := viper.GetString("discord.server_id")
	commands := utils.Map(srv.commands, func(cmd *discordCommand, _ string) *discordgo.ApplicationCommand {
		return &cmd.ApplicationCommand
	})

	_, err := srv.sess.ApplicationCommandBulkOverwrite(appID, serverID, commands, discordgo.WithContext(ctx))
	return err
}

func (srv *DiscordCommandService) autocompleteForSeriesName(
	ctx context.Context,
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	o *discordgo.ApplicationCommandInteractionDataOption,
) {
	partialName := o.StringValue()
	slog.DebugContext(ctx, "Autocomplete for series", "value", partialName)
	if partialName == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: []*discordgo.ApplicationCommandOptionChoice{},
			},
		})
		return
	}

	// Looking up partial name
	series, err := srv.seriesSrv.SearchSeries(ctx, partialName)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to search series", "partial_name", partialName, "error", err)
		utils.NewDiscordResponse(s, i).SetError(err).SetTitle("Could not search for series").Respond()
		return
	}
	series = utils.Clamp(series, 20)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: utils.MapSlice(series, func(series utils.Tuple[string, uint64], _ int) *discordgo.ApplicationCommandOptionChoice {
				return &discordgo.ApplicationCommandOptionChoice{
					Name:  series.T,
					Value: fmt.Sprint(series.V),
				}
			}),
		},
	})
}

type SubscriptionsService struct {
	*SubscriptionsRepo
}

func NewSubscriptionsService(sr *SubscriptionsRepo) *SubscriptionsService {
	return &SubscriptionsService{
		SubscriptionsRepo: sr,
	}
}

func (srv *SubscriptionsService) SubscribeUserToSeries(ctx context.Context, seriesID, userID uint64) error {
	return srv.SubscriptionsRepo.Insert(ctx, &Subscription{
		SeriesID:  seriesID,
		UserID:    userID,
		CreatedAt: time.Now(),
	})
}
