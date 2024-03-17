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
