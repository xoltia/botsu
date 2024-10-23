package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/golang-module/carbon/v2"
	"github.com/jackc/pgx/v5"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/bot"
	"github.com/xoltia/botsu/internal/guilds"
	"github.com/xoltia/botsu/internal/users"
	"github.com/xoltia/botsu/pkg/discordutil"
	"github.com/xoltia/botsu/pkg/ref"
)

var LeaderboardCommandData = &discordgo.ApplicationCommand{
	Name:         "leaderboard",
	Description:  "View the leaderboard",
	DMPermission: ref.New(false),
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "day",
			Description: "View the leaderboard for the current day",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "week",
			Description: "View the leaderboard for the current week",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "month",
			Description: "View the leaderboard for the current month",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "year",
			Description: "View the leaderboard for the current year",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "all",
			Description: "View the leaderboard for all time",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "custom",
			Description: "View the leaderboard over a custom time period",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "start",
					Description: "The start date",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "end",
					Description: "The end date",
					Required:    true,
				},
			},
		},
	},
}

type LeaderboardCommand struct {
	r *activities.ActivityRepository
	u *users.UserRepository
	g *guilds.GuildRepository
}

func NewLeaderboardCommand(r *activities.ActivityRepository, u *users.UserRepository, g *guilds.GuildRepository) *LeaderboardCommand {
	return &LeaderboardCommand{r: r, u: u, g: g}
}

func (c *LeaderboardCommand) Handle(ctx *bot.InteractionContext) error {
	if err := ctx.DeferResponse(); err != nil {
		return err
	}

	i := ctx.Interaction()
	s := ctx.Session()

	if i.GuildID == "" {
		return errors.New("this command can only be used in a guild")
	}

	var start, end time.Time
	now := carbon.Now(carbon.UTC)

	if len(ctx.Options()) == 0 {
		return bot.ErrInvalidOptions
	}

	subcommand := ctx.Options()[0]

	switch subcommand.Name {
	case "day":
		start = now.StartOfDay().ToStdTime()
		end = now.EndOfDay().ToStdTime()
	case "week":
		start = now.StartOfWeek().ToStdTime()
		end = now.EndOfWeek().ToStdTime()
	case "month":
		start = now.StartOfMonth().ToStdTime()
		end = now.EndOfMonth().ToStdTime()
	case "year":
		start = now.StartOfYear().ToStdTime()
		end = now.EndOfYear().ToStdTime()
	case "all":
		start = time.Unix(0, 0)
		end = time.Now()
	case "custom":
		options := subcommand.Options
		user, err := c.u.FindByID(ctx.Context(), i.Member.User.ID)
		guildID := i.GuildID

		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		timezone := carbon.UTC

		if user != nil && user.Timezone != nil {
			timezone = *user.Timezone
		} else if guildID != "" {
			guild, err := c.g.FindByID(ctx.ResponseContext(), guildID)

			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}

			if guild != nil && guild.Timezone != nil {
				timezone = *guild.Timezone
			}
		}

		startString, err := discordutil.GetRequiredStringOption(options, "start")
		if err != nil {
			return err
		}
		endString, err := discordutil.GetRequiredStringOption(options, "end")
		if err != nil {
			return err
		}
		carbonStart := carbon.SetTimezone(timezone).Parse(startString)
		carbonEnd := carbon.SetTimezone(timezone).Parse(endString)

		validStart := carbonStart.IsValid()
		validEnd := carbonEnd.IsValid()
		errorMsg := ""

		if !validStart && !validEnd {
			errorMsg = "Invalid start and end date."
		} else if !validStart {
			errorMsg = "Invalid start date."
		} else if !validEnd {
			errorMsg = "Invalid end date."
		}

		if errorMsg != "" {
			_, err := ctx.Followup(&discordgo.WebhookParams{
				Content: errorMsg,
			}, false)

			return err
		}

		if carbonEnd.Lt(carbonStart) {
			start = carbonEnd.ToStdTime()
			end = carbonStart.ToStdTime()
		} else {
			start = carbonStart.ToStdTime()
			end = carbonEnd.ToStdTime()
		}
	}

	// Note: Do not go over 100 members as Discord will not allow fetching 100+ in a single chunk
	topMembers, err := c.r.GetTopMembers(ctx.Context(), i.GuildID, 10, start, end)

	if err != nil {
		return err
	}

	missingMembers := make([]string, 0, len(topMembers))
	foundMembers := make(map[string]*discordgo.Member)

	for _, m := range topMembers {
		guild, err := s.State.Guild(i.GuildID)

		if err != nil {
			missingMembers = append(missingMembers, m.UserID)
			continue
		}

		member, err := s.State.Member(guild.ID, m.UserID)

		if err != nil {
			missingMembers = append(missingMembers, m.UserID)
			continue
		}

		foundMembers[m.UserID] = member
	}

	if len(missingMembers) > 0 {
		nonce, err := discordutil.NewNonce()

		if err != nil {
			return err
		}

		memberChunk := make(chan []*discordgo.Member, 1)

		removeHandler := s.AddHandler(func(s *discordgo.Session, e *discordgo.GuildMembersChunk) {
			if e.Nonce == nonce {
				memberChunk <- e.Members
			}
		})

		defer removeHandler()

		err = s.RequestGuildMembersList(i.GuildID, missingMembers, 0, nonce, false)

		if err != nil {
			return err
		}

		select {
		case <-time.After(5 * time.Second):
			return errors.New("timed out waiting for guild members")
		case <-ctx.Context().Done():
			return ctx.Context().Err()
		case members := <-memberChunk:
			for _, m := range members {
				foundMembers[m.User.ID] = m
			}
		}
	}

	description := fmt.Sprintf("Starting <t:%d:R>, resetting <t:%d:R>.", start.Unix(), end.Unix())

	embed := discordutil.NewEmbedBuilder().
		SetDescription(description).
		SetTitle("Leaderboard").
		SetColor(discordutil.ColorPrimary).
		SetTimestamp(time.Now())

	deadMembers := make([]string, 0, len(topMembers))

	for x, m := range topMembers {
		member, ok := foundMembers[m.UserID]
		displayName := m.UserID
		if ok && member.Nick != "" {
			displayName = member.Nick
		} else if ok && member.User != nil {
			displayName = member.User.Username
		} else if !ok {
			deadMembers = append(deadMembers, m.UserID)
			usr, err := s.User(m.UserID)
			if err != nil {
				ctx.Logger.Warn("Error getting user", slog.String("err", err.Error()), slog.String("user_id", m.UserID))
				continue
			}

			displayName = usr.Username
		}

		title := fmt.Sprintf("%d. %s", x+1, displayName)
		value := m.TotalDuration.Truncate(time.Second).String()

		embed.AddField(title, value, false)
	}

	go func() {
		if err = c.g.RemoveMembers(context.Background(), i.GuildID, deadMembers); err != nil {
			ctx.Logger.Error("Failed to remove members", slog.String("err", err.Error()))
		}
	}()

	_, err = ctx.Followup(&discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed.MessageEmbed},
	}, false)

	return err
}
