package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	sq "github.com/Masterminds/squirrel"
	"github.com/duke605/tv-bot/utils"
	"github.com/jmoiron/sqlx"
)

type SeriesRepo struct {
	db *sqlx.DB
}

func NewSeriesRepo(db *sqlx.DB) *SeriesRepo {
	return &SeriesRepo{
		db: db,
	}
}

func (repo *SeriesRepo) Upsert(ctx context.Context, s *Series, fieldsToUpdate ...string) error {
	builder := sq.Insert("series").SetMap(s.ToMap())
	if len(fieldsToUpdate) > 0 {
		suffix := "ON CONFLICT(`id`) DO UPDATE SET"
		for _, field := range fieldsToUpdate {
			suffix += fmt.Sprintf("\n`%[1]s` = excluded.`%[1]s`", field)
		}
		builder = builder.Suffix(suffix)
	} else {
		builder = builder.Suffix("ON CONFLICT(`id`) DO NOTHING")
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "Upserting series", "query", query)
	if _, err = repo.db.ExecContext(ctx, query, args...); err != nil {
		return err
	}

	return nil
}

func (repo *SeriesRepo) Delete(ctx context.Context, id uint64) error {
	query, args, err := sq.Delete("series").
		Where("id", id).
		ToSql()
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "Deleting series", "query", query)
	if _, err := repo.db.ExecContext(ctx, query, args...); err != nil {
		return err
	}

	return nil
}

func (repo *SeriesRepo) Find(ctx context.Context, id uint64, s *Series) error {
	query, args, err := sq.Select("*").
		From("series").
		Where("id", id).
		ToSql()
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "Finding series", "query", query)
	return repo.db.GetContext(ctx, s, query, args...)
}

func (repo *SeriesRepo) List(ctx context.Context, filter ...interface{}) utils.Pager[Series] {
	builder := sq.Select("*").From("series").Limit(10)

	if len(filter) > 0 {
		builder = builder.Where(filter[0])
	}

	return utils.NewPager(func(_ int, buf []Series) ([]Series, error) {
		if len(buf) > 0 {
			builder = builder.Where(sq.Gt{
				"id": buf[len(buf)-1].ID,
			})
		}

		query, args, err := builder.ToSql()
		if err != nil {
			return nil, err
		}

		slog.DebugContext(ctx, "Listing series", "query", query)
		buf = buf[:0]
		err = repo.db.SelectContext(ctx, &buf, query, args...)
		if err != nil {
			return nil, err
		} else if len(buf) == 0 {
			return nil, sql.ErrNoRows
		}

		return buf, err
	})
}

type NotificationsRepo struct {
	db *sqlx.DB
}

func NewNotificationsRepo(db *sqlx.DB) *NotificationsRepo {
	return &NotificationsRepo{
		db: db,
	}
}

func (repo *NotificationsRepo) InsertMany(ctx context.Context, notis []*Notification) error {
	if len(notis) == 0 {
		return nil
	}

	cols := notis[0].GetColumns()
	builder := sq.Insert("notifications").Columns(cols...)
	for _, noti := range notis {
		builder = builder.Values(noti.ToColumns(cols)...)
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "Inserting many notifications", "query", query)
	_, err = repo.db.ExecContext(ctx, query, args...)
	return err
}

func (repo *NotificationsRepo) ExistsForEpisodeSeasonAndSeries(ctx context.Context, episode, season int, seriesID uint64) (bool, error) {
	query, args, err := sq.Select("series_id").
		From("notifications").
		Where(sq.Eq{
			"series_id": seriesID,
			"season":    season,
			"episode":   episode,
		}).
		Limit(1).
		ToSql()
	if err != nil {
		return false, err
	}

	slog.DebugContext(ctx, "Checking for existence of notification", "query", query)
	err = repo.db.GetContext(ctx, &seriesID, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

type SubscriptionsRepo struct {
	db *sqlx.DB
}

func NewSubscriptionsRepo(db *sqlx.DB) *SubscriptionsRepo {
	return &SubscriptionsRepo{
		db: db,
	}
}

// GetAllSubscribedToSeries returns a slice of user IDs that are subscribed to the series ID provided
func (repo *SubscriptionsRepo) GetAllSubscribedToSeries(ctx context.Context, seriesID uint64) ([]uint64, error) {
	query, args, err := sq.Select("user_id").
		From("subscriptions").
		ToSql()
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "Getting subscribers for series", "query", query)
	userIDs := []uint64{}
	if err = repo.db.SelectContext(ctx, &userIDs, query, args...); err != nil {
		return nil, err
	}

	return userIDs, nil
}
