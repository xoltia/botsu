package commands

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/xoltia/botsu/internal/bot"
	"github.com/xoltia/botsu/internal/guilds"
	"github.com/xoltia/botsu/pkg/discordutil"
	"github.com/xoltia/botsu/pkg/ref"
)

var GuildConfigCommandData = &discordgo.ApplicationCommand{
	Name:                     "guild-config",
	Description:              "Configure guild settings",
	DMPermission:             ref.New(false),
	DefaultMemberPermissions: ref.New(int64(discordgo.PermissionAdministrator)),
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:         "timezone",
			Description:  "Set the guild's timezone",
			Type:         discordgo.ApplicationCommandOptionString,
			Required:     false,
			Autocomplete: true,
		},
	},
}

type GuildConfigCommand struct {
	r *guilds.GuildRepository
}

func NewGuildConfigCommand(r *guilds.GuildRepository) *GuildConfigCommand {
	return &GuildConfigCommand{r: r}
}

func (c *GuildConfigCommand) Handle(ctx *bot.InteractionContext) error {
	if ctx.IsAutocomplete() {
		return c.handleAutocomplete(ctx)
	}

	i := ctx.Interaction()
	options := ctx.Options()

	if len(options) != 1 {
		return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
			Content: "You must provide one option!",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
	}

	switch options[0].Name {
	case "timezone":
		timezone, err := discordutil.GetRequiredStringOption(options, "timezone")

		if err != nil {
			return err
		}

		if !IsValidTimezone(timezone) {
			return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
				Content: "Invalid timezone",
			})
		}

		err = c.r.SetGuildTimezone(ctx.Context(), i.GuildID, timezone)
		if err != nil {
			return err
		}

		return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
			Content: "Timezone set!",
		})
	}
	return nil
}

func (c *GuildConfigCommand) handleAutocomplete(ctx *bot.InteractionContext) error {
	focuedOption := discordutil.GetFocusedOption(ctx.Options())

	if focuedOption == nil {
		return nil
	}

	switch focuedOption.Name {
	case "timezone":
		const maxResults = 25
		timezone := focuedOption.StringValue()
		results := make([]*discordgo.ApplicationCommandOptionChoice, 0, maxResults)

		if timezone == "" {
			for i, tz := range ValidTimezones {
				if i >= maxResults {
					break
				}

				results = append(results, &discordgo.ApplicationCommandOptionChoice{
					Name:  tz,
					Value: tz,
				})
			}

		} else {
			for _, tz := range ValidTimezones {
				target := getComparableTimezoneString(timezone)
				compare := getComparableTimezoneString(tz)

				if strings.Contains(compare, target) {
					results = append(results, &discordgo.ApplicationCommandOptionChoice{
						Name:  tz,
						Value: tz,
					})
				}

				if len(results) >= maxResults {
					break
				}
			}
		}

		return ctx.Respond(discordgo.InteractionApplicationCommandAutocompleteResult, &discordgo.InteractionResponseData{
			Choices: results,
		})
	default:
		return nil
	}
}
