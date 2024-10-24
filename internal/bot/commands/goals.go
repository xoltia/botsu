package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/adhocore/gronx"
	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/bot"
	"github.com/xoltia/botsu/internal/goals"
	"github.com/xoltia/botsu/pkg/discordutil"
)

var GoalCommandData = &discordgo.ApplicationCommand{
	Name:        "goal",
	Description: "Manage your goals.",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "create",
			Description: "Create a new goal.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "The name of the goal.",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "target",
					Description: "The target duration of the goal.",
					Required:    true,
				},
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "cron",
					Description:  "The cron expression for the goal.",
					Required:     true,
					Autocomplete: true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "activity-type",
					Description: "The type of activity to track.",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Listening",
							Value: activities.ActivityImmersionTypeListening,
						},
						{
							Name:  "Reading",
							Value: activities.ActivityImmersionTypeReading,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "media-type",
					Description: "The type of media to track.",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Visual Novel",
							Value: activities.ActivityMediaTypeVisualNovel,
						},
						{
							Name:  "Book",
							Value: activities.ActivityMediaTypeBook,
						},
						{
							Name:  "Manga",
							Value: activities.ActivityMediaTypeManga,
						},
						{
							Name:  "Anime",
							Value: activities.ActivityMediaTypeAnime,
						},
						{
							Name:  "Video",
							Value: activities.ActivityMediaTypeVideo,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "youtube-channels",
					Description: "The YouTube channels to track (comma separated, e.g. @HakuiKoyori,@ui_shig,@MinatoAqua).",
					Required:    false,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "list",
			Description: "List your goals.",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "delete",
			Description: "Delete a goal.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "id",
					Description: "The ID of the goal.",
					Required:    true,
				},
			},
		},
	},
}

type GoalCommand struct {
	goals *goals.GoalService
}

func NewGoalCommand(goals *goals.GoalService) *GoalCommand {
	return &GoalCommand{goals: goals}
}

func (c *GoalCommand) Handle(cmd *bot.InteractionContext) error {
	if cmd.IsAutocomplete() {
		return c.handleAutocomplete(cmd)
	}

	if len(cmd.Options()) == 0 {
		return bot.ErrInvalidOptions
	}

	subcommand := cmd.Options()[0]

	switch subcommand.Name {
	case "create":
		return c.handleCreate(cmd, subcommand)
	case "list":
		return c.handleList(cmd, subcommand)
	case "delete":
		return c.handleDelete(cmd, subcommand)
	default:
		return bot.ErrInvalidOptions
	}
}

func (c *GoalCommand) handleDelete(cmd *bot.InteractionContext, subcommand *discordgo.ApplicationCommandInteractionDataOption) error {
	id, err := discordutil.GetRequiredIntOption(subcommand.Options, "id")
	if err != nil {
		return err
	}

	cmd.Logger.Debug("Finding goal for deletion", slog.Int64("goal_id", id))

	goal, err := c.goals.FindByID(cmd.ResponseContext(), id)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("error finding goal: %w", err)
		}

		return cmd.Respond(
			discordgo.InteractionResponseChannelMessageWithSource,
			&discordgo.InteractionResponseData{
				Content: fmt.Sprintf("No goal found with ID: %d", id),
			},
		)
	}

	if goal.UserID != cmd.User().ID {
		return cmd.Respond(
			discordgo.InteractionResponseChannelMessageWithSource,
			&discordgo.InteractionResponseData{
				Content: fmt.Sprintf("No goal found with ID: %d", id),
			},
		)
	}

	cmd.Logger.Debug("Deleting goal", slog.Int64("goal_id", id))

	err = c.goals.DeleteByID(cmd.ResponseContext(), id)
	if err != nil {
		return err
	}

	return cmd.Respond(
		discordgo.InteractionResponseChannelMessageWithSource,
		&discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Goal **%s** deleted.", goal.Name),
		},
	)
}

func (c *GoalCommand) handleList(cmd *bot.InteractionContext, _ *discordgo.ApplicationCommandInteractionDataOption) error {
	goals, err := c.goals.CheckAll(cmd.ResponseContext(), cmd.User().ID)
	if err != nil {
		return fmt.Errorf("failed to find goals: %w", err)
	}

	if len(goals) == 0 {
		return cmd.Respond(
			discordgo.InteractionResponseChannelMessageWithSource,
			&discordgo.InteractionResponseData{
				Content: "You have not set any goals! Try setting one with: `/goal create`",
			},
		)
	}

	if err := cmd.DeferResponse(); err != nil {
		return err
	}

	embed := discordutil.NewEmbedBuilder().
		SetTitle("Goals").
		SetColor(discordutil.ColorPrimary).
		SetTimestamp(time.Now())

	for _, goal := range goals {
		nextDueDate, err := c.goals.NextCron(cmd.Context(), goal)
		if err != nil {
			return fmt.Errorf("failed to calculate next due date: %w", err)
		}

		title := fmt.Sprintf("%s (%d)", goal.Name, goal.ID)

		embed.AddField(title, fmt.Sprintf(
			"Progress: %s / %s **(%.2f%%)**\nNext Reset: <t:%d>",
			goal.Current,
			goal.Target,
			goal.Current.Seconds()/goal.Target.Seconds()*100,
			nextDueDate.Unix(),
		), false)
	}

	pages := embed.SplitOnFields(2)

	if len(pages) == 1 {
		_, err := cmd.Followup(
			&discordgo.WebhookParams{
				Embeds: []*discordgo.MessageEmbed{pages[0].MessageEmbed},
			},
			true,
		)
		return err
	}

	nextButton := discordgo.Button{
		Label:    "Next",
		Style:    discordgo.PrimaryButton,
		CustomID: "next",
		Disabled: false,
	}

	previousButton := discordgo.Button{
		Label:    "Previous",
		Style:    discordgo.SecondaryButton,
		CustomID: "prev",
		Disabled: false,
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				previousButton,
				nextButton,
			},
		},
	}

	msg, err := cmd.Followup(
		&discordgo.WebhookParams{
			Embeds:     []*discordgo.MessageEmbed{pages[0].MessageEmbed},
			Components: components,
		},
		true,
	)

	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute*3)
	defer cancel()

	componentInteractions, err := cmd.Bot.NewMessageComponentInteractionChannel(ctx, msg)
	if err != nil {
		return err
	}

	page := 0

	for ci := range componentInteractions {
		switch ci.MessageComponentData().CustomID {
		case "next":
			page++
		case "prev":
			page--
		}

		page = (page + len(pages)) % len(pages)

		err := cmd.Session().InteractionRespond(ci.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{pages[page].MessageEmbed},
				Components: components,
			},
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func (c *GoalCommand) handleCreate(cmd *bot.InteractionContext, subcommand *discordgo.ApplicationCommandInteractionDataOption) error {
	name, err := discordutil.GetRequiredStringOption(subcommand.Options, "name")
	if err != nil {
		return err
	}

	target, err := discordutil.GetRequiredIntOption(subcommand.Options, "target")
	if err != nil {
		return err
	}

	cron, err := discordutil.GetRequiredStringOption(subcommand.Options, "cron")
	if err != nil {
		return err
	}

	gron := gronx.New()

	if !gron.IsValid(cron) {
		return cmd.Respond(
			discordgo.InteractionResponseChannelMessageWithSource,
			&discordgo.InteractionResponseData{
				Content: "Invalid cron provided. See https://crontab.guru/ for help on creating a valid cron expression.",
			},
		)
	}

	activityType := discordutil.GetStringOption(subcommand.Options, "activity-type")
	mediaType := discordutil.GetStringOption(subcommand.Options, "media-type")
	ytChannels := discordutil.GetStringOption(subcommand.Options, "youtube-channels")

	goal := &goals.Goal{}

	if activityType != nil {
		goal.ActivityType = activityType
	}

	if mediaType != nil {
		goal.MediaType = mediaType
	}

	if ytChannels != nil {
		goal.YoutubeChannels = strings.Split(*ytChannels, ",")
	}

	goal.Name = name
	goal.Target = time.Duration(target) * time.Minute
	goal.Cron = cron
	goal.UserID = cmd.User().ID
	goal.DueAt, err = c.goals.NextCron(cmd.ResponseContext(), goal)

	if err != nil {
		return fmt.Errorf("failed to calculate due date: %w", err)
	}

	cmd.Logger.Debug("Creating goal", slog.Any("goal", goal))

	if err := c.goals.Create(cmd.ResponseContext(), goal); err != nil {
		return fmt.Errorf("failed to create goal: %w", err)
	}

	return cmd.Respond(
		discordgo.InteractionResponseChannelMessageWithSource,
		&discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Goal **%s** created!", goal.Name),
		},
	)
}

func (c *GoalCommand) handleAutocomplete(cmd *bot.InteractionContext) error {
	choices := [...]*discordgo.ApplicationCommandOptionChoice{
		{
			Name:  "Daily",
			Value: "@daily",
		},
		{
			Name:  "Weekly",
			Value: "@weekly",
		},
		{
			Name:  "Monthly",
			Value: "@monthly",
		},
		{
			Name:  "Yearly",
			Value: "@yearly",
		},
	}

	//filteredChoices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(choices))
	//input := discordutil.GetFocusedOption(cmd.Options())
	//
	//if input != nil && input.Type == discordgo.ApplicationCommandOptionString && len(input.Value.(string)) > 0 {
	//	for _, choice := range choices {
	//		if strings.Contains(choice.Value.(string), input.Value.(string)) {
	//			filteredChoices = append(filteredChoices, choice)
	//		}
	//	}
	//} else {
	//	filteredChoices = choices[:]
	//}

	return cmd.Respond(discordgo.InteractionApplicationCommandAutocompleteResult, &discordgo.InteractionResponseData{
		Choices: choices[:],
	})
}
