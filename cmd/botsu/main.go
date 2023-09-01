package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/UTD-JLA/botsu/internal/activities"
	"github.com/UTD-JLA/botsu/internal/bot"
	"github.com/UTD-JLA/botsu/internal/commands"
	"github.com/UTD-JLA/botsu/internal/users"
	"github.com/UTD-JLA/botsu/pkg/aodb"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	config := NewConfig()

	err := config.Load("config.toml")

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Reading anime database file")

	err = aodb.ReadDatabaseFile("anime-offline-database.json")

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Creating index")

	err = aodb.CreateIndex()

	if err != nil {
		log.Fatal(err)
	}

	bot := bot.NewBot()

	log.Println("Connecting to database")

	// conn, err := pgx.Connect(context.Background(), config.Database.ConnectionString())

	pool, err := pgxpool.New(context.Background(), config.Database.ConnectionString())

	if err != nil {
		log.Fatal(err)
	}

	defer pool.Close()

	if err != nil {
		log.Fatal(err)
	}

	activityRepo := activities.NewActivityRepository(pool)
	userRepo := users.NewUserRepository(pool)

	bot.AddCommand(commands.PingCommandData, commands.NewPingCommand())
	bot.AddCommand(commands.LogCommandData, commands.NewLogCommand(activityRepo, userRepo))
	bot.AddCommand(commands.ConfigCommandData, commands.NewConfigCommand(userRepo))
	bot.AddCommand(commands.HistoryCommandData, commands.NewHistoryCommand(activityRepo))
	bot.AddCommand(commands.LeaderboardCommandData, commands.NewLeaderboardCommand(activityRepo, userRepo))

	log.Println("Logging in")

	err = bot.Login(config.Token)

	if err != nil {
		log.Fatal(err)
	}

	defer bot.Close()

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running. Press CTRL-C to exit")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-c
}
