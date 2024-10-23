package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"

	_ "github.com/glebarez/go-sqlite"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/guilds"
	"github.com/xoltia/botsu/internal/users"
)

var dbLocation = flag.String("db", "bot_data.db", "Database file location")
var maxConcurrency = flag.Int("max-concurrency", 3, "Maximum number of concurrent requests")
var populateVideoMeta = flag.Bool("vid-meta", true, "Populate video metadata")
var pgDBURL = flag.String("pgdb", "", "Postgres database URL")

type aibUser struct {
	ID                              string
	Timezone                        *string
	ReadingSpeedCharsPerMinute      *float32
	ReadingSpeedBookPagesPerMinute  *float32
	ReadingSpeedMangaPagesPerMinute *float32
	DailyGoal                       *uint64
}

type aibGuild struct {
	ID       string
	Timezone *string
}

func main() {
	flag.Parse()

	db, err := sql.Open("sqlite", *dbLocation)

	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	pgPool, err := pgxpool.New(ctx, *pgDBURL)

	if err != nil {
		log.Fatal(err)
	}

	activityRepo := activities.NewActivityRepository(pgPool)
	userRepo := users.NewUserRepository(pgPool)
	guildRepo := guilds.NewGuildRepository(pgPool)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	go func() {
		<-sigs
		fmt.Println("Interrupted")
		cancel()
	}()

	userRows, err := db.QueryContext(ctx, "SELECT user_id, time_zone, reading_speed, reading_speed_book_pages, reading_speed_pages, daily_goal FROM user_configs")

	if err != nil {
		log.Fatal(err)
	}

	for userRows.Next() {
		var user aibUser

		err = userRows.Scan(
			&user.ID,
			&user.Timezone,
			&user.ReadingSpeedCharsPerMinute,
			&user.ReadingSpeedBookPagesPerMinute,
			&user.ReadingSpeedMangaPagesPerMinute,
			&user.DailyGoal,
		)

		if err != nil {
			log.Fatal(err)
		}

		newUser := users.NewUser(user.ID)
		newUser.Timezone = user.Timezone

		if user.DailyGoal != nil {
			newUser.DailyGoal = int(*user.DailyGoal)
		}

		if user.ReadingSpeedCharsPerMinute != nil {
			newUser.VisualNovelReadingSpeed = *user.ReadingSpeedCharsPerMinute
		}

		if user.ReadingSpeedBookPagesPerMinute != nil {
			newUser.BookReadingSpeed = *user.ReadingSpeedBookPagesPerMinute
		}

		if user.ReadingSpeedMangaPagesPerMinute != nil {
			newUser.MangaReadingSpeed = *user.ReadingSpeedMangaPagesPerMinute
		}

		err = userRepo.Create(ctx, newUser)

		if err != nil {
			log.Println(err)
		}
	}

	guildRows, err := db.QueryContext(ctx, "SELECT guild_id, time_zone FROM guild_configs")

	if err != nil {
		log.Fatal(err)
	}

	for guildRows.Next() {
		var guild aibGuild

		err = guildRows.Scan(
			&guild.ID,
			&guild.Timezone,
		)

		if err != nil {
			log.Fatal(err)
		}

		newGuild := guilds.NewGuild(guild.ID)
		newGuild.Timezone = guild.Timezone

		err = guildRepo.Create(ctx, newGuild)

		if err != nil {
			log.Println(err)
		}
	}

	activityRows, err := db.QueryContext(ctx, "SELECT id, user_id, name, type, url, date, duration, raw_duration, raw_duration_unit, speed FROM activities")

	if err != nil {
		log.Fatal(err)
	}

	i := 0
	sem := make(chan struct{}, *maxConcurrency)
	wg := sync.WaitGroup{}

	for activityRows.Next() {
		fmt.Printf("Processing row %d\n", i)

		var activity aibActivity

		err = activityRows.Scan(
			&activity.ID,
			&activity.UserID,
			&activity.Name,
			&activity.Type,
			&activity.URL,
			&activity.Date,
			&activity.Duration,
			&activity.RawDuration,
			&activity.RawDurationUnit,
			&activity.Speed,
		)

		if err != nil {
			log.Fatal(err)
		}

		// get tags, relations are in tags_to_activities (tag_id, activity_id)
		// and tags (id, name)
		tagsRows, err := db.QueryContext(ctx, `
			SELECT tags.name
			FROM tags_to_activities
			INNER JOIN tags ON tags_to_activities.tag_id = tags.id
			WHERE tags_to_activities.activity_id = ?
		`, activity.ID)

		if err != nil {
			log.Fatal(err)
		}

		for tagsRows.Next() {
			var tag string

			err = tagsRows.Scan(&tag)

			if err != nil {
				log.Fatal(err)
			}

			activity.Tags = append(activity.Tags, tag)
		}

		newActivity := activity.asNewFormatActivity()

		if *populateVideoMeta && newActivity.MediaType != nil &&
			*newActivity.MediaType == activities.ActivityMediaTypeVideo && activity.URL != nil {
			fmt.Printf("Processing video %s\n", *activity.URL)
			wg.Add(1)
			sem <- struct{}{}

			go func() {
				defer wg.Done()
				defer func() { <-sem }()

				err = populateVideoMetadata(ctx, newActivity, *activity.URL)

				if err != nil {
					log.Println(err)
				}

				err = activityRepo.Create(ctx, newActivity)

				if err != nil {
					log.Println(err)
				}
			}()
		} else {
			err = activityRepo.Create(ctx, newActivity)

			if err != nil {
				log.Println(err)
			}
		}

		i++
	}

	wg.Wait()
}
