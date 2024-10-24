package commands

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/bot"
	"github.com/xoltia/botsu/internal/users"
	"github.com/xoltia/botsu/pkg/discordutil"
	"github.com/xoltia/botsu/pkg/ref"
)

var ConfigCommandData = &discordgo.ApplicationCommand{
	Name:        "config",
	Description: "Configure your timezone and active guilds",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:         "timezone",
			Description:  "Set your timezone",
			Type:         discordgo.ApplicationCommandOptionString,
			Required:     false,
			Autocomplete: true,
		},
		{
			Name:         "vn-speed",
			Description:  "Set your VN reading speed (char/min)",
			Type:         discordgo.ApplicationCommandOptionNumber,
			Required:     false,
			Autocomplete: true,
		},
		{
			Name:         "book-speed",
			Description:  "Set your book reading speed (page/min)",
			Type:         discordgo.ApplicationCommandOptionNumber,
			Required:     false,
			Autocomplete: true,
		},
		{
			Name:         "manga-speed",
			Description:  "Set your manga reading speed (page/min)",
			Type:         discordgo.ApplicationCommandOptionNumber,
			Required:     false,
			Autocomplete: true,
		},
		{
			Name:         "daily-goal",
			Description:  "Set your daily immersion goal (minutes)",
			Type:         discordgo.ApplicationCommandOptionInteger,
			MinValue:     ref.New(0.0),
			MaxValue:     1440,
			Required:     false,
			Autocomplete: false,
		},
	},
}

type ConfigCommand struct {
	userRepository     *users.UserRepository
	activityRepository *activities.ActivityRepository
}

func NewConfigCommand(r *users.UserRepository, a *activities.ActivityRepository) *ConfigCommand {
	return &ConfigCommand{userRepository: r, activityRepository: a}
}

func (c *ConfigCommand) Handle(ctx *bot.InteractionContext) error {
	if ctx.IsAutocomplete() {
		return c.handleAutocomplete(ctx)
	}

	embedBuilder := discordutil.NewEmbedBuilder().
		SetColor(discordutil.ColorDanger).
		SetTitle("Error!")

	i := ctx.Interaction()
	options := ctx.Options()

	if len(options) != 1 {
		embedBuilder.SetDescription("You must provide one option.")

		return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
			Flags:  discordgo.MessageFlagsEphemeral,
		})
	}

	switch options[0].Name {
	case "timezone":
		timezone, err := discordutil.GetRequiredStringOption(options, "timezone")
		if err != nil {
			return err
		}

		if !IsValidTimezone(timezone) {
			embedBuilder.SetDescription("Invalid timezone.")

			return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
				Flags:  discordgo.MessageFlagsEphemeral,
			})
		}

		err = c.userRepository.SetUserTimezone(ctx.Context(), discordutil.GetInteractionUser(i).ID, timezone)
		if err != nil {
			return err
		}

		embedBuilder.SetDescription("Your timezone has been updated.")
	case "vn-speed":
		vnSpeed, err := discordutil.GetRequiredFloatOption(options, "vn-speed")
		if err != nil {
			return err
		}

		err = c.userRepository.SetVisualNovelReadingSpeed(ctx.Context(), discordutil.GetInteractionUser(i).ID, float32(vnSpeed))
		if err != nil {
			return err
		}

		embedBuilder.SetDescription("Your visual novel reading speed has been updated.")
	case "book-speed":
		bookSpeed, err := discordutil.GetRequiredFloatOption(options, "book-speed")
		if err != nil {
			return err
		}

		err = c.userRepository.SetBookReadingSpeed(ctx.Context(), discordutil.GetInteractionUser(i).ID, float32(bookSpeed))
		if err != nil {
			return err
		}

		embedBuilder.SetDescription("Your book reading speed has been updated.")
	case "manga-speed":
		mangaSpeed, err := discordutil.GetRequiredFloatOption(options, "manga-speed")
		if err != nil {
			return err
		}

		err = c.userRepository.SetMangaReadingSpeed(ctx.Context(), discordutil.GetInteractionUser(i).ID, float32(mangaSpeed))
		if err != nil {
			return err
		}

		embedBuilder.SetDescription("Your manga reading speed has been updated.")
	case "daily-goal":
		dailyGoal, err := discordutil.GetRequiredUintOption(options, "daily-goal")
		if err != nil {
			return err
		}

		err = c.userRepository.SetDailyGoal(ctx.Context(), discordutil.GetInteractionUser(i).ID, int(dailyGoal))
		if err != nil {
			return err
		}

		embedBuilder.SetDescription("Your daily goal has been updated.")
	default:
		return fmt.Errorf("unexpected option: %s", options[0].Name)
	}

	embedBuilder.SetColor(discordutil.ColorSuccess)
	embedBuilder.SetTitle("Success!")

	return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embedBuilder.MessageEmbed},
	})
}

func (c *ConfigCommand) handleAutocomplete(ctx *bot.InteractionContext) error {
	focusedOption := discordutil.GetFocusedOption(ctx.Options())

	if focusedOption == nil {
		return nil
	}

	if focusedOption.Name == "timezone" {
		const maxResults = 25
		timezone := focusedOption.StringValue()
		results := make([]*discordgo.ApplicationCommandOptionChoice, 0, maxResults)

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

		return ctx.Respond(discordgo.InteractionApplicationCommandAutocompleteResult, &discordgo.InteractionResponseData{
			Choices: results,
		})
	}

	// Option is now one of the speed options
	userID := discordutil.GetInteractionUser(ctx.Interaction()).ID
	now := time.Now()
	start := now.AddDate(0, 0, -21)
	mediaType := activities.ActivityMediaTypeVisualNovel
	speedUnit := "cpm"

	if focusedOption.Name == "book-speed" {
		mediaType = activities.ActivityMediaTypeBook
		speedUnit = "ppm"
	} else if focusedOption.Name == "manga-speed" {
		mediaType = activities.ActivityMediaTypeManga
		speedUnit = "ppm"
	}

	avg, err := c.activityRepository.GetAvgSpeedByMediaTypeAndUserID(
		ctx.ResponseContext(),
		mediaType,
		userID,
		start,
		now,
	)

	if err != nil {
		return err
	}

	var choices []*discordgo.ApplicationCommandOptionChoice

	if avg > 0 {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("Recommended: %.2f (%s)", avg, speedUnit),
			Value: avg,
		})
	}

	return ctx.Respond(discordgo.InteractionApplicationCommandAutocompleteResult, &discordgo.InteractionResponseData{
		Choices: choices,
	})
}

var ValidTimezones = []string{
	"UTC",
	"GMT",
	"EET",
	"WET",
	"CET",
	"EST",
	"MST",
	"Cuba",
	"Egypt",
	"Eire",
	"Greenwich",
	"Iceland",
	"Iran",
	"Israel",
	"Jamaica",
	"Japan",
	"Libya",
	"Poland",
	"Portugal",
	"PRC",
	"Singapore",
	"Turkey",
	"Asia/Shanghai",
	"Asia/Chongqing",
	"Asia/Harbin",
	"Asia/Urumqi",
	"Asia/Hong_Kong",
	"Asia/Macao",
	"Asia/Taipei",
	"Asia/Tokyo",
	"Asia/Saigon",
	"Asia/Seoul",
	"Asia/Bangkok",
	"Asia/Dubai",
	"America/New_York",
	"America/Los_Angeles",
	"America/Chicago",
	"Europe/Moscow",
	"Europe/London",
	"Europe/Berlin",
	"Europe/Paris",
	"Europe/Rome",
	"Australia/Sydney",
	"Australia/Melbourne",
	"Australia/Darwin",
}

func IsValidTimezone(timezone string) bool {
	for _, tz := range ValidTimezones {
		if timezone == tz {
			return true
		}
	}
	return false
}

func getComparableTimezoneString(tzString string) string {
	tzStr := strings.Builder{}
	for _, c := range tzString {
		if strings.ContainsRune(" \t_/", c) {
			continue
		}

		tzStr.WriteRune(unicode.ToLower(c))
	}

	return tzStr.String()
}
