package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/duke605/tv-bot/utils"
	"github.com/jmoiron/sqlx"
)

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

	start := time.Now()
	defer func() {
		slog.DebugContext(ctx, "Inserting many notifications", "query", query, "duration", time.Since(start).String(), "error", err)
	}()
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

	start := time.Now()
	defer func() {
		slog.DebugContext(ctx, "Checking for existence of notification", "query", query, "duration", time.Since(start).String(), "error", err)
	}()
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

func (repo *SubscriptionsRepo) GetDistinctSeriesIDs(ctx context.Context) utils.Pager[uint64] {
	builder := sq.Select("DISTINCT series_id").From("subscriptions").Limit(10)

	return utils.NewPager(func(page int, buf []uint64) ([]uint64, error) {
		query, args, err := builder.Offset(uint64(page) * 10).ToSql()
		if err != nil {
			return nil, err
		}

		start := time.Now()
		defer func() {
			slog.DebugContext(ctx, "Getting distinct series IDs", "query", query, "duration", time.Since(start).String(), "error", err)
		}()
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

// GetAllSubscribedToSeries returns a slice of user IDs that are subscribed to the series ID provided
func (repo *SubscriptionsRepo) GetAllSubscribedToSeries(ctx context.Context, seriesID uint64) ([]uint64, error) {
	query, args, err := sq.Select("user_id").
		From("subscriptions").
		ToSql()
	if err != nil {
		return nil, err
	}

	start := time.Now()
	defer func() {
		slog.DebugContext(ctx, "Getting subscribers for series", "query", query, "duration", time.Since(start).String(), "error", err)
	}()
	userIDs := []uint64{}
	if err = repo.db.SelectContext(ctx, &userIDs, query, args...); err != nil {
		return nil, err
	}

	return userIDs, nil
}

func (repo *SubscriptionsRepo) Insert(ctx context.Context, sub *Subscription) error {
	query, args, err := sq.Insert("subscriptions").
		SetMap(sub.ToMap()).
		ToSql()
	if err != nil {
		return err
	}

	start := time.Now()
	defer func() {
		slog.DebugContext(ctx, "Inserting subscription", "query", query, "duration", time.Since(start).String(), "error", err)
	}()
	_, err = repo.db.ExecContext(ctx, query, args)
	return err
}

func (repo *SubscriptionsRepo) UserIsSubscribed(ctx context.Context, userID uint64) (bool, error) {
	query, args, err := sq.Select("true").
		From("subscriptions").
		Limit(1).
		Where("user_id", userID).
		ToSql()
	if err != nil {
		return false, err
	}

	start := time.Now()
	defer func() {
		slog.DebugContext(ctx, "Checking if user is subscribed", "query", query, "user_id", userID, "duration", time.Since(start).String(), "error", err)
	}()
	f := false
	err = repo.db.GetContext(ctx, &f, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return f, nil
}

func (repo *SubscriptionsRepo) GetEarliestSubscriptionForSeries(ctx context.Context, seriesID uint64) (time.Time, error) {
	query, args, err := sq.Select("MIN(created_at)").
		From("subscriptions").
		Where("series_id", seriesID).
		ToSql()
	if err != nil {
		return time.Time{}, err
	}

	start := time.Now()
	defer func() {
		slog.DebugContext(ctx, "Getting earliest subscription for series", "query", query, "series_id", seriesID, "duration", time.Since(start).String(), "error", err)
	}()
	t := time.Time{}
	err = repo.db.GetContext(ctx, &t, query, args...)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

func (repo *SubscriptionsRepo) DeleteSubscriptionsForSeries(ctx context.Context, seriesID uint64) error {
	query, args, err := sq.Delete("subscriptions").
		Where("series_id", seriesID).
		ToSql()
	if err != nil {
		return err
	}

	start := time.Now()
	defer func() {
		slog.DebugContext(ctx, "Deleting subscriptions for series", "query", query, "series_id", seriesID, "duration", time.Since(start).String(), "error", err)
	}()
	_, err = repo.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	return nil
}
