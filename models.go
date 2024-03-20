package main

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/duke605/tv-bot/moviedb"
)

type Null[T any] struct {
	sql.Null[T]
}

func (n Null[T]) NullOrValue() interface{} {
	if n.Valid {
		return n.V
	}

	return nil
}

func NewNull[T any](v T, valid bool) Null[T] {
	return Null[T]{
		Null: sql.Null[T]{
			V:     v,
			Valid: valid,
		},
	}
}

type JSON[T any] struct {
	V T
}

func (j *JSON[T]) Scan(src interface{}) error {
	if src == nil {
		j.V = *new(T)
	}

	var b []byte
	switch src := src.(type) {
	case string:
		b = []byte(src)
	case []byte:
		b = src
	default:
		return fmt.Errorf("models: '%T' not supported", src)
	}

	return json.Unmarshal(b, &j.V)
}

func (j JSON[T]) Value() (driver.Value, error) {
	return json.Marshal(j.V)
}

type Notification struct {
	Episode          int    `db:"episode"`
	Season           int    `db:"name"`
	SeriesID         uint64 `db:"series_id"`
	DiscordMessageID uint64 `db:"discord_message_id"`
}

func (Notification) GetColumns() []string {
	return []string{
		"episode", "season", "series_id", "discord_message_id",
	}
}

func (n *Notification) ToColumns(cols []string) []interface{} {
	values := make([]interface{}, len(cols))
	for i, col := range cols {
		switch col {
		case "episode":
			values[i] = n.Episode
		case "season":
			values[i] = n.Season
		case "series_id":
			values[i] = n.SeriesID
		case "discord_message_id":
			values[i] = n.DiscordMessageID
		}
	}

	return values
}

func (n *Notification) ToMap() map[string]any {
	return map[string]any{
		"episode":            n.Episode,
		"season":             n.Season,
		"series_id":          n.SeriesID,
		"discord_message_id": n.DiscordMessageID,
	}
}

type Subscription struct {
	SeriesID  uint64    `db:"series_id"`
	UserID    uint64    `db:"user_id"`
	CreatedAt time.Time `db:"created_at"`
}

func (s *Subscription) ToMap() map[string]any {
	return map[string]any{
		"series_id":  s.SeriesID,
		"user_id":    s.UserID,
		"created_at": s.CreatedAt,
	}
}

type Series struct {
	ID                 uint64                       `db:"id"`
	NextEpisodeAirDate Null[time.Time]              `db:"next_episode_air_date"`
	Data               JSON[*moviedb.SeriesDetails] `db:"data"`
	LastFetchedAt      time.Time                    `db:"last_fetched_at"`
}

func (s *Series) ToMap() map[string]any {
	return map[string]any{
		"id":                    s.ID,
		"next_episode_air_date": s.NextEpisodeAirDate,
		"data":                  s.Data,
		"last_fetched_at":       s.LastFetchedAt,
	}
}
