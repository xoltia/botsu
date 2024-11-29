package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/bot"
	"github.com/xoltia/botsu/internal/bot/commands"
	"github.com/xoltia/botsu/internal/goals"
	"github.com/xoltia/botsu/internal/guilds"
	"github.com/xoltia/botsu/internal/mediadata"
	"github.com/xoltia/botsu/internal/users"
	"github.com/xoltia/botsu/internal/videos"
	"github.com/xoltia/botsu/migrations"
)

var (
	configPath      = flag.String("config", "config.toml", "Path to config file")
	enableProfiling = flag.Bool("profiling", false, "Enable profiling")
	skipMigration   = flag.Bool("skip-migration", false, "Skip automatic migration")
	skipDataUpdate  = flag.Bool("skip-data-update", false, "Skip automatic data update")
)

func main() {
	flag.Parse()
	config := NewConfig()

	logLevel := new(slog.LevelVar)
	logLevel.Set(slog.LevelWarn)

	logHandler := tint.NewHandler(os.Stdout, &tint.Options{
		Level: logLevel,
	})

	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	err := config.Load(*configPath)
	if err != nil && !os.IsNotExist(err) {
		logger.Error("Unable to load config", slog.String("err", err.Error()), slog.String("path", *configPath))
		os.Exit(1)
	}

	err = config.LoadEnv()
	if err != nil {
		logger.Error("Unable to load config from env", slog.String("err", err.Error()))
		os.Exit(1)
	}

	config.LoadDefaults()

	// Config is loaded, now we can set the log level
	logLevel.Set(config.LogLevel)
	logger.Info("Log level set", slog.String("level", config.LogLevel.String()))

	if config.GoogleAPIKey != "" {
		err = videos.EnableYouTubeAPI(config.GoogleAPIKey)
		if err != nil {
			logger.Error("Unable to setup YouTube API", slog.String("err", err.Error()))
			os.Exit(1)
		} else {
			logger.Info("YouTube API enabled")
		}
	}

	discordgo.Logger = func(msgL, _caller int, format string, a ...interface{}) {
		msg := fmt.Sprintf("[DGO] "+format, a...)

		switch msgL {
		case discordgo.LogError:
			logger.Error(msg)
		case discordgo.LogWarning:
			logger.Warn(msg)
		case discordgo.LogInformational:
			logger.Info(msg)
		case discordgo.LogDebug:
			logger.Debug(msg)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mediaSearcher := mediadata.NewMediaSearcher("data")
	mediaSearcher.Logger = logger.WithGroup("searcher")

	if !*skipDataUpdate {
		if err = mediaSearcher.UpdateData(ctx); err != nil {
			logger.Error("Unable to update searcher data", slog.String("err", err.Error()))
			os.Exit(1)
		}
	}

	if err = mediaSearcher.Open(); err != nil {
		logger.Error("Unable to open searcher", slog.String("err", err.Error()))
		os.Exit(1)
	}

	defer func() {
		if err = mediaSearcher.Close(); err != nil {
			logger.Error("Unable to close searcher", slog.String("err", err.Error()))
		}
	}()

	logger.Debug("Starting data update ticker", slog.Duration("interval", config.DataUpdateInterval))
	updateTicker := time.NewTicker(config.DataUpdateInterval)
	defer updateTicker.Stop()

	go func() {
		for range updateTicker.C {
			if err = mediaSearcher.UpdateData(context.Background()); err != nil {
				logger.Error("Unable to update searcher data", slog.String("err", err.Error()))
			}
		}
	}()

	logger.Info("Connecting to database")

	if !*skipMigration {
		migrationURL := config.Database.ConnectionURL()
		//q := migrationURL.Query()
		//q.Add("sslmode", "disable")
		//migrationURL.RawQuery = q.Encode()

		// for debug purposes
		files, err := migrations.MigrationFS.ReadDir(".")
		if err == nil {
			logger.Debug("Loading embedded migrations")
			for _, f := range files {
				logger.Debug(fmt.Sprintf("%s", f))
			}
		}

		migrationSource, err := iofs.New(migrations.MigrationFS, ".")
		if err != nil {
			logger.Error("Unable to create migration source", slog.String("err", err.Error()))
			os.Exit(1)
		}

		m, err := migrate.NewWithSourceInstance("migrations.MigrationFS", migrationSource, migrationURL.String())
		if err != nil {
			logger.Error("Unable to create migration", slog.String("err", err.Error()))
			os.Exit(1)
		}

		err = m.Up()
		noChange := errors.Is(err, migrate.ErrNoChange)
		if err != nil && !noChange {
			logger.Error("Unable to run migrations", slog.String("err", err.Error()))
			os.Exit(1)
		}

		ver, dirty, err := m.Version()
		if err != nil {
			logger.Warn("Failed to get database version")
		}

		if dirty {
			logger.Warn("Database is dirty")
		}

		if noChange {
			logger.Info("Database is up to date", slog.Uint64("version", uint64(ver)))
		} else {
			logger.Info("Database updated", slog.Uint64("version", uint64(ver)))
		}
	} else {
		logger.Info("Skipping migration check")
	}

	pool, err := pgxpool.New(ctx, config.Database.ConnectionString())
	if err != nil {
		logger.Error("Unable to connect to database", slog.String("err", err.Error()))
		os.Exit(1)
	}

	defer pool.Close()

	activityRepo := activities.NewActivityRepository(pool)
	userRepo := users.NewUserRepository(pool)
	guildRepo := guilds.NewGuildRepository(pool)
	timeService := users.NewUserTimeService(userRepo, guildRepo)
	goalRepo := goals.NewGoalRepository(pool)
	goalService := goals.NewGoalService(goalRepo, timeService)

	bot := bot.NewBot(ctx, bot.Options{
		Logger:        logger.WithGroup("bot"),
		NoPanic:       config.NoPanic,
		MemberTracker: guildRepo,
	})
	// bot.SetNoPanic(config.NoPanic)

	bot.AddCommand(commands.LogCommandData, commands.NewLogCommand(activityRepo, userRepo, guildRepo, mediaSearcher, goalService, timeService))
	bot.AddCommand(commands.ConfigCommandData, commands.NewConfigCommand(userRepo, activityRepo))
	bot.AddCommand(commands.HistoryCommandData, commands.NewHistoryCommand(activityRepo))
	bot.AddCommand(commands.LeaderboardCommandData, commands.NewLeaderboardCommand(activityRepo, userRepo, guildRepo))
	bot.AddCommand(commands.UndoCommandData, commands.NewUndoCommand(activityRepo))
	bot.AddCommand(commands.ChartCommandData, commands.NewChartCommand(activityRepo, userRepo, guildRepo))
	bot.AddCommand(commands.GuildConfigCommandData, commands.NewGuildConfigCommand(guildRepo))
	bot.AddCommand(commands.ExportCommandData, commands.NewExportCommand(activityRepo))
	bot.AddCommand(commands.ImportCommandData, commands.NewImportCommand(activityRepo))
	bot.AddCommand(commands.GoalCommandData, commands.NewGoalCommand(goalService))
	logger.Info("Starting bot")

	intents := discordgo.IntentsNone

	if config.UseMembersIntent {
		intents = discordgo.IntentsGuildMembers
	}

	err = bot.Login(config.Token, intents)

	if err != nil {
		logger.Error("Unable to login", slog.String("err", err.Error()))
		os.Exit(1)
	}

	defer bot.Close()

	// Wait here until CTRL-C or other term signal is received.
	logger.Info("Setup completed, press CTRL-C to exit")

	if *enableProfiling {
		logger.Warn("Profiling server enabled")

		go func() {
			err := http.ListenAndServe("localhost:6060", nil)
			logger.Warn("Profiling server exited", slog.String("err", err.Error()))
		}()
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-c
}
