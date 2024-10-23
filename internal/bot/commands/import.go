package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/bot"
	activitiesPub "github.com/xoltia/botsu/pkg/activities"
	"github.com/xoltia/botsu/pkg/discordutil"
)

var ImportCommandData = &discordgo.ApplicationCommand{
	Name:        "import",
	Description: "Import new data and manage your previous imports",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "botsu-file",
			Description: "Import new data",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionAttachment,
					Name:        "file",
					Description: "Import your data from a file exported by Botsu",
					Required:    true,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "list",
			Description: "List your previous imports",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "undo",
			Description: "Undo an import by timestamp",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "timestamp",
					Description: "The timestamp of the import to undo (see `/import list`)",
					Required:    true,
				},
			},
		},
	},
}

type ImportCommand struct {
	r *activities.ActivityRepository
}

func NewImportCommand(r *activities.ActivityRepository) *ImportCommand {
	return &ImportCommand{r}
}

func (c *ImportCommand) handleList(
	ctx context.Context,
	cmd *bot.InteractionContext,
) error {
	history, err := c.r.GetRecentImportsByUserID(ctx, cmd.User().ID, 10)

	if err != nil {
		return err
	}

	if len(history) == 0 {
		_, err = cmd.Followup(&discordgo.WebhookParams{
			Content: "No imports found.",
		}, false)

		return err
	}

	embed := discordutil.NewEmbedBuilder().
		SetTitle("Recent Imports").
		SetDescription("Use `/import undo {timestamp-id}` to undo an import.").
		SetColor(discordutil.ColorInfo).
		SetTimestamp(time.Now())

	for _, h := range history {
		embed.AddField(
			fmt.Sprintf("%d", h.Timestamp.UnixNano()),
			fmt.Sprintf(
				"Imported %d activities <t:%d:R>",
				h.Count,
				h.Timestamp.Unix(),
			),
			false,
		)
	}

	_, err = cmd.Followup(&discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed.MessageEmbed},
	}, false)

	return err
}

func (c *ImportCommand) handleUndo(
	ctx context.Context,
	cmd *bot.InteractionContext,
	opts []*discordgo.ApplicationCommandInteractionDataOption,
) error {
	// use string instead of int because it is too large for integer options
	timestampString, err := discordutil.GetRequiredStringOption(opts, "timestamp")
	if err != nil {
		return err
	}
	timestamp, err := strconv.ParseInt(timestampString, 10, 64)
	embedBuilder := discordutil.NewEmbedBuilder().
		SetColor(discordutil.ColorDanger).
		SetTitle("Error!")

	if err != nil {
		embedBuilder.SetDescription("Invalid timestamp!")
		_, err = cmd.Followup(&discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
		}, false)

		return err
	}

	var removed int64

	if removed, err = c.r.UndoImportByUserIDAndTimestamp(ctx, cmd.User().ID, time.Unix(0, timestamp)); err != nil {
		embedBuilder.SetDescription("Failed to undo import!")

		_, err = cmd.Followup(&discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
		}, false)

		return err
	}

	if removed == 0 {
		embedBuilder.SetDescription("No activities were removed. Make sure you are using the correct timestamp!")
		embedBuilder.SetColor(discordutil.ColorWarning)
		_, err = cmd.Followup(&discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
		}, false)

		return err
	}

	embedBuilder.SetDescription(fmt.Sprintf("Successfully removed import! %d entries were removed.", removed))
	embedBuilder.SetTitle("Success!")
	embedBuilder.SetColor(discordutil.ColorSuccess)

	_, err = cmd.Followup(&discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
	}, false)

	return err
}

func (c *ImportCommand) Handle(cmd *bot.InteractionContext) error {
	if err := cmd.DeferResponse(); err != nil {
		return err
	}

	ctx := cmd.Context()
	opts := cmd.Options()

	if len(opts) == 0 {
		return bot.ErrInvalidOptions
	}

	subcommand := opts[0].Name
	opts = opts[0].Options

	if subcommand == "list" {
		return c.handleList(ctx, cmd)
	}

	if subcommand == "undo" {
		return c.handleUndo(ctx, cmd, opts)
	}

	attachmentOption, err := discordutil.GetRequiredOption(opts, "file")

	if err != nil {
		return err
	}

	attachmentID, ok := attachmentOption.Value.(string)

	if !ok {
		return errors.New("expected string value from attachment option")
	}

	attachment := cmd.Data().Resolved.Attachments[attachmentID]
	extension := strings.ToLower(path.Ext(attachment.Filename))

	embedBuilder := discordutil.NewEmbedBuilder().
		SetColor(discordutil.ColorDanger).
		SetTitle("Error!")

	if extension != ".gz" && extension != ".jsonl" {
		_, err := cmd.Followup(&discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				embedBuilder.SetDescription("Invalid file type.").MessageEmbed,
			},
		}, false)
		return err
	}

	req, err := http.NewRequest(http.MethodGet, attachment.URL, nil)

	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var as []*activities.Activity

	switch extension {
	case ".gz":
		as, err = activitiesPub.ReadCompressedJSONL(resp.Body)
	case ".jsonl":
		as, err = activitiesPub.ReadJSONL(resp.Body)
	}

	if err != nil {
		_, err = cmd.Followup(&discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				embedBuilder.SetDescription("Failed to read file. Make sure it is a valid JSONL file.").MessageEmbed,
			},
		}, false)
		return err
	}

	for i, a := range as {
		// Important: make sure ID is overwritten
		a.UserID = cmd.User().ID

		if err = activities.ValidateExternalActivity(a); err == nil {
			continue
		}

		embedBuilder.SetTitle("Invalid Activity!")

		description := fmt.Sprintf(
			"Activity with ID %d on line %d was unable to be imported: %s",
			a.ID,
			i+1,
			err)

		embedBuilder.SetDescription(description)

		_, err := cmd.Followup(&discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
		}, false)

		return err
	}

	if err := c.r.ImportMany(ctx, as); err != nil {
		cmd.Logger.Error("Failed to import activities", slog.String("err", err.Error()))

		embedBuilder.SetDescription("Failed to import activities. Check your import list for incomplete imports and try again later.")

		_, err := cmd.Followup(&discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
		}, false)

		return err
	}

	embedBuilder.SetTitle("Success!")
	embedBuilder.SetDescription(fmt.Sprintf("Successfully imported **%d** activities.\nView your import history with `/import list`.", len(as)))
	embedBuilder.SetColor(discordutil.ColorSuccess)
	embedBuilder.SetTimestamp(time.Now())

	_, err = cmd.Followup(&discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
	}, false)

	return err
}
