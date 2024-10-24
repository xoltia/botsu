package goals

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GoalRepository struct {
	pool *pgxpool.Pool
}

func NewGoalRepository(pool *pgxpool.Pool) *GoalRepository {
	return &GoalRepository{pool}
}

func (r *GoalRepository) Create(ctx context.Context, g *Goal) (err error) {
	err = r.pool.QueryRow(
		ctx,
		`INSERT INTO goals (user_id, name, activity_type, media_type, youtube_channels, target, current, cron, due_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id`,
		g.UserID,
		g.Name,
		g.ActivityType,
		g.MediaType,
		g.YoutubeChannels,
		g.Target,
		g.Current,
		g.Cron,
		g.DueAt,
	).Scan(&g.ID)

	return
}

func (r *GoalRepository) FindByID(ctx context.Context, id int64) (goal *Goal, err error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, name, activity_type, media_type, youtube_channels, target, current, cron, due_at, created_at
		FROM goals		
		WHERE deleted_at IS NULL
		AND id = $1
	`, id)

	goal = &Goal{}

	err = row.Scan(
		&goal.ID,
		&goal.UserID,
		&goal.Name,
		&goal.ActivityType,
		&goal.MediaType,
		&goal.YoutubeChannels,
		&goal.Target,
		&goal.Current,
		&goal.Cron,
		&goal.DueAt,
		&goal.CreatedAt,
	)

	return
}

func (r *GoalRepository) FindByUserID(ctx context.Context, userID string) (goals []*Goal, err error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT id, user_id, name, activity_type, media_type, youtube_channels, target, current, cron, due_at, created_at
		FROM goals
		WHERE deleted_at IS NULL
		AND user_id = $1`,
		userID,
	)

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		g := &Goal{}
		err = rows.Scan(
			&g.ID,
			&g.UserID,
			&g.Name,
			&g.ActivityType,
			&g.MediaType,
			&g.YoutubeChannels,
			&g.Target,
			&g.Current,
			&g.Cron,
			&g.DueAt,
			&g.CreatedAt,
		)

		if err != nil {
			return
		}

		goals = append(goals, g)
	}

	return
}

func (r *GoalRepository) BeginUpdateTxByUserID(ctx context.Context, userID string) (goals []*Goal, tx pgx.Tx, err error) {
	tx, err = r.pool.Begin(ctx)

	if err != nil {
		return
	}

	rows, err := tx.Query(
		ctx,
		`SELECT id, user_id, name, activity_type, media_type, youtube_channels, target, current, cron, due_at, created_at
		FROM goals
		WHERE user_id = $1
		AND DELETED_AT IS NULL
		FOR UPDATE`,
		userID,
	)

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		g := &Goal{}
		err = rows.Scan(
			&g.ID,
			&g.UserID,
			&g.Name,
			&g.ActivityType,
			&g.MediaType,
			&g.YoutubeChannels,
			&g.Target,
			&g.Current,
			&g.Cron,
			&g.DueAt,
			&g.CreatedAt,
		)

		if err != nil {
			return
		}

		goals = append(goals, g)
	}

	return
}

func (r *GoalRepository) UpdateTx(ctx context.Context, tx pgx.Tx, g *Goal) (err error) {
	_, err = tx.Exec(
		ctx,
		`UPDATE goals
		SET name = $1, activity_type = $2, media_type = $3, youtube_channels = $4, target = $5, current = $6, cron = $7, due_at = $8
		WHERE id = $9`,
		g.Name,
		g.ActivityType,
		g.MediaType,
		g.YoutubeChannels,
		g.Target,
		g.Current,
		g.Cron,
		g.DueAt,
		g.ID,
	)

	return
}

func (r *GoalRepository) DeleteByID(ctx context.Context, id int64) (err error) {
	_, err = r.pool.Exec(ctx, "UPDATE GOALS SET deleted_at = (NOW() AT TIME ZONE 'UTC') WHERE id = $1", id)
	return
}
