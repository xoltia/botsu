package commands

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/bot"
	"github.com/xoltia/botsu/pkg/discordutil"
	"github.com/xoltia/botsu/pkg/ref"
)

var HistoryCommandData = &discordgo.ApplicationCommand{
	Name:        "history",
	Description: "View your activity history",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        "show-ids",
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Description: "Show the IDs of the activities.",
			Required:    false,
		},
		{
			Name:        "quick-nav",
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Description: "Enable quick navigation buttons.",
			Required:    false,
		},
		{
			Name:        "user",
			Type:        discordgo.ApplicationCommandOptionUser,
			Description: "The user to view the history of (defaults to yourself).",
			Required:    false,
		},
		{
			Name:        "page",
			Type:        discordgo.ApplicationCommandOptionInteger,
			MinValue:    ref.New(1.0),
			Description: "The page of history to view.",
			Required:    false,
		},
	},
}

type HistoryCommand struct {
	r *activities.ActivityRepository
}

func NewHistoryCommand(r *activities.ActivityRepository) *HistoryCommand {
	return &HistoryCommand{r: r}
}

func (c *HistoryCommand) Handle(ctx *bot.InteractionContext) error {
	if err := ctx.DeferResponse(); err != nil {
		return err
	}

	//offset := 0
	const pageSize = 6
	i := ctx.Interaction()
	s := ctx.Session()
	user := discordutil.GetUserOption(ctx.Options(), "user", s)
	showIDsOption := discordutil.GetBoolOption(ctx.Options(), "show-ids")
	quickNavOption := discordutil.GetBoolOption(ctx.Options(), "quick-nav")
	pageNumber := discordutil.GetUintOptionOrDefault(ctx.Options(), "page", 1)
	offset := int(pageNumber-1) * pageSize
	showIDs := showIDsOption != nil && *showIDsOption
	quickNav := quickNavOption != nil && *quickNavOption

	if user == nil {
		user = discordutil.GetInteractionUser(i)
	}

	page, err := c.r.PageByUserID(ctx.Context(), user.ID, ctx.Interaction().GuildID, pageSize, offset)

	if err != nil {
		return err
	}

	embed := discordutil.NewEmbedBuilder().
		SetTitle("Activity History").
		SetColor(discordutil.ColorPrimary).
		SetAuthor(user.Username, user.AvatarURL("256"), "").
		SetFooter(fmt.Sprintf("Page %d of %d", page.Page, page.PageCount), "")

	for _, activity := range page.Activities {
		if !showIDs {
			embed.AddField(activity.Date.Format(time.DateTime), activity.Name, true)
		} else {
			// IDs should be on their own line
			// to allow mobile users to copy them
			embed.AddField(
				strconv.FormatUint(activity.ID, 10),
				fmt.Sprintf(
					"%s (%s)",
					activity.Name,
					activity.Date.Format(time.DateTime),
				),
				true,
			)
		}
	}

	nextButton := discordgo.Button{
		Label:    "Next",
		Style:    discordgo.PrimaryButton,
		CustomID: "history_next",
		Disabled: page.Page == page.PageCount,
	}

	previousButton := discordgo.Button{
		Label:    "Previous",
		Style:    discordgo.SecondaryButton,
		CustomID: "history_previous",
		Disabled: true,
	}

	// number of pages to fast forward
	const fastForwardAmount = 5

	fastForwardButton := discordgo.Button{
		Style:    discordgo.PrimaryButton,
		Emoji:    &discordgo.ComponentEmoji{Name: "⏩"},
		CustomID: "history_fast_forward",
		Disabled: page.Page+fastForwardAmount > page.PageCount,
	}

	rewindButton := discordgo.Button{
		Style:    discordgo.SecondaryButton,
		Emoji:    &discordgo.ComponentEmoji{Name: "⏪"},
		CustomID: "history_rewind",
		Disabled: page.Page-fastForwardAmount < 1,
	}

	startButton := discordgo.Button{
		Style:    discordgo.SecondaryButton,
		Emoji:    &discordgo.ComponentEmoji{Name: "⏮️"},
		CustomID: "history_start",
		Disabled: page.Page == 1,
	}

	endButton := discordgo.Button{
		Style:    discordgo.SecondaryButton,
		Emoji:    &discordgo.ComponentEmoji{Name: "⏭️"},
		CustomID: "history_end",
		Disabled: page.Page == page.PageCount,
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				previousButton,
				nextButton,
			},
		},
	}

	if quickNav {
		nextButton.Label = ""
		previousButton.Label = ""
		nextButton.Emoji = &discordgo.ComponentEmoji{Name: "▶️"}
		previousButton.Emoji = &discordgo.ComponentEmoji{Name: "◀️"}

		components[0] = discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				rewindButton,
				previousButton,
				nextButton,
				fastForwardButton,
			},
		}

		components = append(components, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				startButton,
				endButton,
			},
		})
	}

	msg, err := ctx.Followup(&discordgo.WebhookParams{
		Embeds:     []*discordgo.MessageEmbed{embed.MessageEmbed},
		Components: components,
	}, true)

	if err != nil {
		return err
	}

	collectionContext, cancel := context.WithTimeout(ctx.Context(), 2*time.Minute)

	defer cancel()

	interactions, err := ctx.Bot.NewMessageComponentInteractionChannel(
		collectionContext,
		msg,
		discordutil.NewInteractionUserFilter(i),
	)

	if err != nil {
		return err
	}

	for ci := range interactions {
		ciContext, cancel := context.WithDeadline(ctx.Context(), discordutil.GetInteractionResponseDeadline(ci.Interaction))

		if ci.MessageComponentData().CustomID == "history_previous" {
			offset -= pageSize
		} else if ci.MessageComponentData().CustomID == "history_next" {
			offset += pageSize
		} else if ci.MessageComponentData().CustomID == "history_fast_forward" {
			offset += pageSize * fastForwardAmount
		} else if ci.MessageComponentData().CustomID == "history_rewind" {
			offset -= pageSize * fastForwardAmount
		} else if ci.MessageComponentData().CustomID == "history_start" {
			offset = 0
		} else if ci.MessageComponentData().CustomID == "history_end" {
			offset = (page.PageCount - 1) * pageSize
		}

		page, err = c.r.PageByUserID(ciContext, user.ID, ctx.Interaction().GuildID, pageSize, offset)

		if err != nil {
			cancel()
			return err
		}

		if page.Page%2 == 0 {
			embed.SetColor(discordutil.ColorSecondary)
		} else {
			embed.SetColor(discordutil.ColorPrimary)
		}

		embed.SetFooter(fmt.Sprintf("Page %d of %d", page.Page, page.PageCount), "")
		embed.ClearFields()

		for _, activity := range page.Activities {
			if !showIDs {
				embed.AddField(activity.Date.Format(time.DateTime), activity.Name, true)
			} else {
				// IDs should be on their own line
				// to allow mobile users to copy them
				embed.AddField(
					strconv.FormatUint(activity.ID, 10),
					fmt.Sprintf(
						"%s (%s)",
						activity.Name,
						activity.Date.Format(time.DateTime),
					),
					true,
				)
			}
		}

		previousButton.Disabled = page.Page == 1
		nextButton.Disabled = page.Page == page.PageCount
		fastForwardButton.Disabled = page.Page+fastForwardAmount > page.PageCount
		rewindButton.Disabled = page.Page-fastForwardAmount < 1
		startButton.Disabled = page.Page == 1
		endButton.Disabled = page.Page == page.PageCount

		if quickNav {
			components = []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						rewindButton,
						previousButton,
						nextButton,
						fastForwardButton,
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						startButton,
						endButton,
					},
				},
			}
		} else {
			components = []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						previousButton,
						nextButton,
					},
				},
			}
		}

		err := s.InteractionRespond(ci.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed.MessageEmbed},
				Components: components,
			},
		})

		cancel()

		if err != nil {
			return err
		}
	}

	_, err = ctx.Session().InteractionResponseEdit(ctx.Interaction().Interaction, &discordgo.WebhookEdit{
		Components: &[]discordgo.MessageComponent{},
	})

	return err
}
