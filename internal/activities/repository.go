package activities

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xoltia/botsu/pkg/orderedmap"
)

type MemberStats struct {
	UserID        string
	TotalDuration time.Duration
}

type UserActivityPage struct {
	Activities []*Activity
	PageCount  int
	Page       int
}

type ImportInfo struct {
	Timestamp time.Time
	Count     int
}

type ChannelStats struct {
	ChannelName   string
	TotalDuration time.Duration
}

type ActivityRepository struct {
	pool *pgxpool.Pool
}

func NewActivityRepository(pool *pgxpool.Pool) *ActivityRepository {
	return &ActivityRepository{pool: pool}
}

func (r *ActivityRepository) Create(ctx context.Context, activity *Activity) error {
	err := r.pool.QueryRow(
		ctx,
		`INSERT INTO activities (user_id, guild_id, name, primary_type, media_type, duration, date, meta)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id;`,
		activity.UserID,
		activity.GuildID,
		activity.Name,
		activity.PrimaryType,
		activity.MediaType,
		activity.Duration,
		activity.Date,
		activity.Meta).
		Scan(&activity.ID)

	return err
}

func (r *ActivityRepository) ImportMany(ctx context.Context, as []*Activity) error {
	columnNames := []string{
		"user_id",
		"guild_id",
		"name",
		"primary_type",
		"media_type",
		"duration",
		"date",
		"meta",
		"created_at",
		"deleted_at",
		"imported_at",
	}

	now := time.Now().UTC()
	rows := make([][]interface{}, len(as))

	for i, a := range as {
		rows[i] = []interface{}{
			a.UserID,
			a.GuildID,
			a.Name,
			a.PrimaryType,
			a.MediaType,
			a.Duration,
			a.Date,
			a.Meta,
			a.CreatedAt,
			a.DeletedAt,
			now,
		}
	}

	_, err := r.pool.CopyFrom(ctx, pgx.Identifier{"activities"}, columnNames, pgx.CopyFromRows(rows))
	return err
}

func (r *ActivityRepository) UndoImportByUserIDAndTimestamp(
	ctx context.Context,
	userID string,
	timestamp time.Time,
) (int64, error) {
	query := `
		UPDATE activities
		SET deleted_at = NOW() AT TIME ZONE 'UTC'
		WHERE user_id = $1
		AND imported_at = $2 AT TIME ZONE 'UTC'
		AND deleted_at IS NULL
	`

	tag, err := r.pool.Exec(ctx, query, userID, timestamp)
	return tag.RowsAffected(), err
}

func (r *ActivityRepository) GetRecentImportsByUserID(
	ctx context.Context,
	userID string,
	limit int,
) ([]ImportInfo, error) {
	query := `
		SELECT imported_at, COUNT(*) as count
		FROM activities
		WHERE user_id = $1
		AND imported_at IS NOT NULL
		AND deleted_at IS NULL
		GROUP BY imported_at
		ORDER BY imported_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}

	var importHistory []ImportInfo

	for rows.Next() {
		importInfo := ImportInfo{}
		if err = rows.Scan(&importInfo.Timestamp, &importInfo.Count); err != nil {
			return nil, err
		}

		importHistory = append(importHistory, importInfo)
	}

	return importHistory, nil
}

func (r *ActivityRepository) GetTotalYouTubeWatchTimeByUserID(
	ctx context.Context,
	userID string,
	start, end time.Time,
) (time.Duration, error) {
	query := `
		SELECT COALESCE(SUM(duration), 0)
		FROM activities
		WHERE user_id = $1
		AND media_type = 'video'
		AND meta->>'platform' = 'youtube'
		AND deleted_at IS NULL
		AND date >= $2
		AND date <= $3
	`

	row := r.pool.QueryRow(ctx, query, userID, start, end)
	var total time.Duration
	err := row.Scan(&total)
	return total, err
}

func (r *ActivityRepository) GetLatestYouTubeChannelNamesByUserIDAndChannelID(
	ctx context.Context,
	userID string,
	channelID ...string,
) ([]string, error) {
	query := `
		SELECT COALESCE(meta->>'channel_name', user_id::text)
		FROM activities
		WHERE user_id = $1
		AND media_type = 'video'
		AND meta->>'channel_name' IS NOT NULL
		AND meta->>'channel_id' = $2
		AND meta->>'platform' = 'youtube'
		AND deleted_at IS NULL
		ORDER BY date DESC
		LIMIT 1
	`

	batch := &pgx.Batch{}
	for _, id := range channelID {
		batch.Queue(query, userID, id)
	}

	result := r.pool.SendBatch(ctx, batch)
	defer result.Close()

	channelNames := make([]string, 0, len(channelID))
	for i := 0; i < len(channelID); i++ {
		row := result.QueryRow()
		var channelName string
		if err := row.Scan(&channelName); err != nil {
			return nil, err
		}
		channelNames = append(channelNames, channelName)
	}

	return channelNames, nil
}

func (r *ActivityRepository) GetTotalByUserIDGroupByVideoChannel(
	ctx context.Context,
	userID string,
	start, end time.Time,
	limit int,
) (orderedmap.Map[ChannelStats], error) {
	query := `
		SELECT
			COALESCE(SUM(duration), 0) AS total_duration,
			meta->>'channel_id' AS channel_id
		FROM activities
		WHERE user_id = $1
		AND media_type = 'video'
		AND meta->>'platform' = 'youtube'
		AND meta->>'channel_id' IS NOT NULL
		AND date >= $2
		AND date <= $3
		AND deleted_at IS NULL
		GROUP BY meta->>'channel_id'
		ORDER BY total_duration DESC
		LIMIT $4
	`
	rows, err := r.pool.Query(ctx, query, userID, start, end, limit)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	channels := orderedmap.New[ChannelStats]()
	for rows.Next() {
		var channelID string
		var duration time.Duration

		if err := rows.Scan(&duration, &channelID); err != nil {
			return nil, err
		}

		channels.Set(channelID, ChannelStats{
			TotalDuration: duration,
		})
	}

	channelNames, err := r.GetLatestYouTubeChannelNamesByUserIDAndChannelID(ctx, userID, channels.Keys()...)
	if err != nil {
		return nil, err
	}

	for i, channelID := range channels.Keys() {
		v, _ := channels.Get(channelID)
		channels.Set(channelID, ChannelStats{
			ChannelName:   channelNames[i],
			TotalDuration: v.TotalDuration,
		})
	}

	return channels, nil
}

func (r *ActivityRepository) GetTotalByUserIDGroupedByMonth(
	ctx context.Context,
	userID, guildID string,
	start, end time.Time,
) (orderedmap.Map[time.Duration], error) {
	query := `
		SELECT
			to_char(date_series.month, 'YYYY-MM') AS month,
			COALESCE(SUM(duration), 0) AS total_duration
		FROM (
			SELECT
				generate_series(
					$3::date,
					$4::date,
					interval '1 month'
				) AS month
		) AS date_series
		LEFT JOIN users u ON u.id = $1
		LEFT JOIN guilds g ON g.id = $2
		LEFT JOIN activities
			ON date_series.month = date_trunc(
				'month',
				activities.date at time zone COALESCE(u.timezone, g.timezone, 'UTC')
			)
			AND activities.user_id = $1
			AND activities.deleted_at IS NULL
		GROUP BY month
		ORDER BY month ASC
	`

	rows, err := r.pool.Query(ctx, query, userID, guildID, start, end)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	durations := orderedmap.NewWithCapacity[time.Duration](int(end.Sub(start).Hours()/24) + 1)

	for rows.Next() {
		var date string
		var duration time.Duration

		if err := rows.Scan(&date, &duration); err != nil {
			return nil, err
		}

		durations.Set(date, duration)
	}

	return durations, nil
}

// Returns map of day (YYYY-MM-DD) to total duration
// filling in missing days with 0 (string formatted according to user's timezone)
func (r *ActivityRepository) GetTotalByUserIDGroupedByDay(
	ctx context.Context,
	userID, guildID string,
	start, end time.Time,
) (orderedmap.Map[time.Duration], error) {
	// day should be truncated to a string `YYYY-MM-DD` in the user's timezone
	query := `
		SELECT
			to_char(date_series.day, 'YYYY-MM-DD') AS day,
			COALESCE(SUM(duration), 0) AS total_duration
		FROM (
			SELECT
				generate_series(
					$3::date,
					$4::date,
					interval '1 day'
				) AS day
		) AS date_series
		LEFT JOIN users u ON u.id = $1
		LEFT JOIN guilds g ON g.id = $2
		LEFT JOIN activities
			ON date_series.day = date_trunc(
				'day',
				activities.date at time zone COALESCE(u.timezone, g.timezone, 'UTC')
			)
			AND activities.user_id = $1
			AND activities.deleted_at IS NULL
		GROUP BY day
		ORDER BY day ASC
	`

	rows, err := r.pool.Query(ctx, query, userID, guildID, start, end)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	durations := orderedmap.NewWithCapacity[time.Duration](int(end.Sub(start).Hours()/24) + 1)

	for rows.Next() {
		var date string
		var duration time.Duration

		if err := rows.Scan(&date, &duration); err != nil {
			return nil, err
		}

		durations.Set(date, duration)
	}

	return durations, nil
}

func (r *ActivityRepository) GetLatestByUserID(ctx context.Context, userID, guildID string) (*Activity, error) {
	query := `
		SELECT activities.id,
			   user_id,
			   guild_id,
			   name,
			   primary_type,
			   media_type,
			   duration,
			   date at time zone COALESCE(u.timezone, g.timezone, 'UTC'),
			   created_at,
			   deleted_at,
			   imported_at,
			   meta
		FROM activities
		LEFT JOIN users u ON activities.user_id = u.id
		LEFT JOIN guilds g ON g.id = $2
		WHERE activities.user_id = $1
		AND deleted_at IS NULL
		ORDER BY date DESC
		LIMIT 1
	`

	var activity Activity
	err := r.pool.QueryRow(ctx, query, userID, guildID).Scan(
		&activity.ID,
		&activity.UserID,
		&activity.GuildID,
		&activity.Name,
		&activity.PrimaryType,
		&activity.MediaType,
		&activity.Duration,
		&activity.Date,
		&activity.CreatedAt,
		&activity.DeletedAt,
		&activity.ImportedAt,
		&activity.Meta,
	)
	if err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *ActivityRepository) GetByID(ctx context.Context, id uint64, guildID string) (*Activity, error) {
	query := `
		SELECT activities.id,
			   user_id,
			   guild_id,
			   name,
			   primary_type,
			   media_type,
			   duration,
			   date at time zone COALESCE(u.timezone, g.timezone, 'UTC'),
			   created_at,
			   deleted_at,
			   imported_at,
			   meta
		FROM activities
		LEFT JOIN users u ON activities.user_id = u.id
		LEFT JOIN guilds g ON g.id = $2
		WHERE activities.id = $1
		AND deleted_at IS NULL
	`

	var activity Activity
	err := r.pool.QueryRow(ctx, query, id, guildID).Scan(
		&activity.ID,
		&activity.UserID,
		&activity.GuildID,
		&activity.Name,
		&activity.PrimaryType,
		&activity.MediaType,
		&activity.Duration,
		&activity.Date,
		&activity.CreatedAt,
		&activity.DeletedAt,
		&activity.ImportedAt,
		&activity.Meta,
	)
	if err != nil {
		return nil, err
	}

	return &activity, nil
}

func (r *ActivityRepository) GetAllByUserID(ctx context.Context, userID, guildID string) ([]*Activity, error) {
	query := `
		SELECT activities.id,
			   user_id,
			   guild_id,
			   name,
			   primary_type,
			   media_type,
			   duration,
			   date at time zone COALESCE(u.timezone, g.timezone, 'UTC'),
			   created_at,
			   deleted_at,
			   imported_at,
			   meta
		FROM activities
		LEFT JOIN users u ON activities.user_id = u.id
		LEFT JOIN guilds g ON g.id = $2
		WHERE activities.user_id = $1
		AND deleted_at IS NULL
		ORDER BY date DESC
	`
	rows, err := r.pool.Query(ctx, query, userID, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activities := make([]*Activity, 0)
	for rows.Next() {
		var activity Activity
		if err := rows.Scan(
			&activity.ID,
			&activity.UserID,
			&activity.GuildID,
			&activity.Name,
			&activity.PrimaryType,
			&activity.MediaType,
			&activity.Duration,
			&activity.Date,
			&activity.CreatedAt,
			&activity.DeletedAt,
			&activity.ImportedAt,
			&activity.Meta,
		); err != nil {
			return nil, err
		}
		activities = append(activities, &activity)
	}

	return activities, nil
}

func (r *ActivityRepository) PageByUserID(
	ctx context.Context,
	userID, guildID string,
	limit, offset int,
) (*UserActivityPage, error) {
	query := `
		SELECT activities.id,
			   user_id,
			   guild_id,
			   name,
			   primary_type,
			   media_type,
			   duration,
			   date at time zone COALESCE(u.timezone, g.timezone, 'UTC'),
			   created_at,
			   deleted_at,
			   imported_at,
			   meta,
			   CEIL(COUNT(*) OVER() / $3::float) AS page_count,
			   CEIL($4::float / $3::float) + 1 AS page
		FROM activities
		LEFT JOIN users u ON activities.user_id = u.id
		LEFT JOIN guilds g ON g.id = $2
		WHERE activities.user_id = $1
		AND deleted_at IS NULL
		ORDER BY date DESC
		LIMIT $3
		OFFSET $4
	`

	rows, err := r.pool.Query(ctx, query, userID, guildID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	page := &UserActivityPage{
		Activities: make([]*Activity, 0),
	}

	for rows.Next() {
		var activity Activity
		if err := rows.Scan(
			&activity.ID,
			&activity.UserID,
			&activity.GuildID,
			&activity.Name,
			&activity.PrimaryType,
			&activity.MediaType,
			&activity.Duration,
			&activity.Date,
			&activity.CreatedAt,
			&activity.DeletedAt,
			&activity.ImportedAt,
			&activity.Meta,
			&page.PageCount,
			&page.Page,
		); err != nil {
			return nil, err
		}
		page.Activities = append(page.Activities, &activity)
	}
	return page, nil
}

func (r *ActivityRepository) DeleteByID(ctx context.Context, id uint64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE activities
		SET deleted_at = NOW() AT TIME ZONE 'UTC'
		WHERE id = $1
	`, id)
	return err
}

func (r *ActivityRepository) GetTopMembers(ctx context.Context, guildID string, limit int, start, end time.Time) ([]*MemberStats, error) {
	members := make([]*MemberStats, 0)
	rows, err := r.pool.Query(ctx, `
		SELECT m.user_id, COALESCE(SUM(a.duration), 0) AS total_duration
		FROM guild_members m
		LEFT JOIN activities a ON m.user_id = a.user_id
		WHERE m.guild_id = $1
		AND a.date >= $2
		AND a.date <= $3
		AND a.deleted_at IS NULL
		GROUP BY m.user_id
		ORDER BY total_duration DESC
		LIMIT $4
	`, guildID, start, end, limit)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var member MemberStats
		if err := rows.Scan(
			&member.UserID,
			&member.TotalDuration,
		); err != nil {
			return nil, err
		}
		members = append(members, &member)
	}

	return members, nil
}

func (r *ActivityRepository) GetAvgSpeedByMediaTypeAndUserID(ctx context.Context, mediaType, userID string, start, end time.Time) (float32, error) {
	query := `
		SELECT COALESCE(AVG((meta->'speed')::numeric), 0)
		FROM activities
		WHERE user_id = $1
		AND media_type = $2
		AND deleted_at IS NULL
		AND date >= $3
		AND date <= $4
		AND meta->'speed' IS NOT NULL
		AND jsonb_typeof(meta->'speed') = 'number'
	`

	row := r.pool.QueryRow(ctx, query, userID, mediaType, start, end)

	var avg float32
	err := row.Scan(&avg)
	return avg, err
}

func (r *ActivityRepository) GetTotalWatchTimeOfVideoByUserID(ctx context.Context, userID, videoPlatform, videoID string) (total time.Duration, err error) {
	query := `
		SELECT COALESCE(SUM(duration), 0)
		FROM activities
		WHERE user_id = $1
		AND media_type = 'video'
		AND meta->>'platform' = $2
		AND meta->>'video_id' = $3
		AND deleted_at IS NULL
	`

	row := r.pool.QueryRow(ctx, query, userID, videoPlatform, videoID)
	err = row.Scan(&total)
	return
}
