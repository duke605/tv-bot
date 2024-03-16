package main

import (
	"database/sql"
	"time"
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

type Series struct {
	ID         uint64       `db:"id"`
	Name       string       `db:"name"`
	PosterPath Null[string] `db:"poster_path"`
	CreatedAt  time.Time    `db:"created_at"`
}

func (s *Series) ToMap() map[string]any {
	return map[string]any{
		"id":          s.ID,
		"name":        s.Name,
		"poster_path": s.PosterPath.NullOrValue(),
		"created_at":  s.CreatedAt,
	}
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

func (s *Notification) ToColumns(cols []string) []interface{} {
	values := make([]interface{}, len(cols))
	for i, col := range cols {
		switch col {
		case "episode":
			values[i] = s.Episode
		case "season":
			values[i] = s.Season
		case "series_id":
			values[i] = s.SeriesID
		case "discord_message_id":
			values[i] = s.DiscordMessageID
		}
	}

	return values
}

func (s *Notification) ToMap() map[string]any {
	return map[string]any{
		"episode":            s.Episode,
		"season":             s.Season,
		"series_id":          s.SeriesID,
		"discord_message_id": s.DiscordMessageID,
	}
}
