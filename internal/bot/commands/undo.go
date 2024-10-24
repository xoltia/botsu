package commands

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/bot"
	"github.com/xoltia/botsu/pkg/discordutil"
	"github.com/xoltia/botsu/pkg/ref"
)

var UndoCommandData = &discordgo.ApplicationCommand{
	Name:        "undo",
	Description: "Undo the last activity you logged",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			MinValue:    ref.New(0.0),
			Name:        "id",
			Description: "The ID of the activity to undo",
			Required:    false,
		},
	},
}

type UndoCommand struct {
	r *activities.ActivityRepository
}

func NewUndoCommand(r *activities.ActivityRepository) *UndoCommand {
	return &UndoCommand{r: r}
}

func (c *UndoCommand) Handle(ctx *bot.InteractionContext) error {
	id := discordutil.GetUintOption(ctx.Options(), "id")

	if id != nil {
		return c.undoActivity(ctx, *id)
	}

	userID := discordutil.GetInteractionUser(ctx.Interaction()).ID
	activity, err := c.r.GetLatestByUserID(ctx.ResponseContext(), userID, ctx.Interaction().GuildID)

	if errors.Is(err, pgx.ErrNoRows) {
		return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
			Content: "You have no activities to undo.",
		})
	} else if err != nil {
		return err
	}

	return c.undoActivity(ctx, activity.ID)
}

func (c *UndoCommand) undoActivity(ctx *bot.InteractionContext, id uint64) error {
	activity, err := c.r.GetByID(ctx.ResponseContext(), id, ctx.Interaction().GuildID)

	if errors.Is(err, pgx.ErrNoRows) {
		return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
			Content: "Activity not found.",
		})
	} else if err != nil {
		return err
	} else if activity.UserID != discordutil.GetInteractionUser(ctx.Interaction()).ID {
		return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
			Content: "You can only undo your own activities!",
		})
	}

	embed := discordutil.NewEmbedBuilder().
		SetTitle("Undo Activity").
		SetDescription("Are you sure you want to undo this activity?").
		AddField("Name", activity.Name, true).
		AddField("Date", fmt.Sprintf("<t:%d>", activity.Date.Unix()), true).
		AddField("Created At", fmt.Sprintf("<t:%d>", activity.CreatedAt.Unix()), true).
		AddField("Duration", activity.Duration.String(), true).
		SetFooter("This cannot be undone!", "").
		SetColor(discordutil.ColorWarning).
		MessageEmbed

	row := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Yes",
				Style:    discordgo.DangerButton,
				CustomID: "undo_confirm",
			},
			discordgo.Button{
				Label:    "No",
				Style:    discordgo.SecondaryButton,
				CustomID: "undo_cancel",
			},
		},
	}

	err = ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{row},
		Flags:      discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		return err
	}

	msg, err := ctx.Session().InteractionResponse(ctx.Interaction().Interaction)
	if err != nil {
		return err
	}

	collectionContext, cancel := context.WithTimeout(ctx.Context(), 15*time.Second)
	defer cancel()

	ci, err := ctx.Bot.CollectSingleComponentInteraction(
		collectionContext,
		msg,
		discordutil.NewInteractionUserFilter(
			ctx.Interaction(),
		),
	)

	if err != nil {
		_, err := ctx.EditResponse(&discordgo.WebhookEdit{
			Content:    ref.New("Timed out."),
			Components: &[]discordgo.MessageComponent{},
			Embeds:     &[]*discordgo.MessageEmbed{},
		})

		return err
	}

	ciCtx, cancel := context.WithDeadline(ctx.Context(), discordutil.GetInteractionResponseDeadline(ci.Interaction))
	defer cancel()

	if ci.MessageComponentData().CustomID == "undo_confirm" {
		err = c.r.DeleteByID(ciCtx, activity.ID)
		if err != nil {
			return err
		}

		err := ctx.Session().InteractionRespond(ci.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Activity deleted.",
				Components: []discordgo.MessageComponent{},
				Embeds:     []*discordgo.MessageEmbed{},
			},
		})

		return err
	} else if ci.MessageComponentData().CustomID == "undo_cancel" {
		err := ctx.Session().InteractionRespond(ci.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Cancelled.",
				Components: []discordgo.MessageComponent{},
				Embeds:     []*discordgo.MessageEmbed{},
			},
		})

		return err
	} else {
		return errors.New("invalid custom id")
	}
}
