package commands

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"net/http"
	"net/url"
	"text/template"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/golang-module/carbon/v2"
	"github.com/jackc/pgx/v5"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/bot"
	"github.com/xoltia/botsu/internal/guilds"
	"github.com/xoltia/botsu/internal/users"
	"github.com/xoltia/botsu/pkg/discordutil"
	"github.com/xoltia/botsu/pkg/orderedmap"
)

var ChartCommandData = &discordgo.ApplicationCommand{
	Name:        "chart",
	Description: "View a chart of your activity",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "duration",
			Description: "View a chart of your daily activity duration",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "start",
					Description: "The start date of the chart",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "end",
					Description: "The end date of the chart",
					Required:    false,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "youtube-channel",
			Description: "View a chart of your YouTube activity by channel",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "The type of chart to view",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "pie",
							Value: "pie",
						},
						{
							Name:  "bar",
							Value: "bar",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "start",
					Description: "The start date of the chart",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "end",
					Description: "The end date of the chart",
					Required:    false,
				},
			},
		},
	},
}

type ChartCommand struct {
	ar *activities.ActivityRepository
	ur *users.UserRepository
	gr *guilds.GuildRepository
}

func NewChartCommand(ar *activities.ActivityRepository, ur *users.UserRepository, gr *guilds.GuildRepository) *ChartCommand {
	return &ChartCommand{ar: ar, ur: ur, gr: gr}
}

var quickChartURL = url.URL{
	Scheme: "https",
	Host:   "quickchart.io",
	Path:   "/chart",
}

//go:embed chart_body.json.tmpl
var barBodyTemplateFile string

//go:embed chart_body_channel_pie.json.tmpl
var channelPieBodyTemplateFile string

var barBodyTemplate = template.Must(template.New("body").Parse(barBodyTemplateFile))
var channelPieBodyTemplate = template.Must(template.New("body").Parse(channelPieBodyTemplateFile))

type barRequestBody struct {
	Values         string
	Labels         string
	Color          string
	SecondaryColor string
	Horizontal     int
	ShowHorizontal bool
}

type pieRequestBody struct {
	Values string
	Labels string
}

func colorAsHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

func getQuickChartBarBody(xValues []string, yValues []float64, yBar int) (*bytes.Buffer, error) {
	buffer := bytes.Buffer{}

	values, _ := json.Marshal(yValues)
	labels, _ := json.Marshal(xValues)

	err := barBodyTemplate.Execute(&buffer, barRequestBody{
		Values:         string(values),
		Labels:         string(labels),
		Color:          fmt.Sprintf("\"%s\"", colorAsHex(discordutil.ColorSecondary)),
		SecondaryColor: fmt.Sprintf("\"%s\"", colorAsHex(discordutil.ColorPrimary)),
		Horizontal:     yBar,
		ShowHorizontal: yBar != 0,
	})

	if err != nil {
		return nil, err
	}

	compactBuffer := bytes.Buffer{}

	if err = json.Compact(&compactBuffer, buffer.Bytes()); err != nil {
		return nil, err
	}

	return &compactBuffer, nil
}

func getQuickChartChannelPieBody(labels []string, values []float64) (*bytes.Buffer, error) {
	buffer := bytes.Buffer{}

	valuesJSON, _ := json.Marshal(values)
	labelsJSON, _ := json.Marshal(labels)

	err := channelPieBodyTemplate.Execute(&buffer, pieRequestBody{
		Values: string(valuesJSON),
		Labels: string(labelsJSON),
	})

	if err != nil {
		return nil, err
	}

	compactBuffer := bytes.Buffer{}

	if err = json.Compact(&compactBuffer, buffer.Bytes()); err != nil {
		return nil, err
	}

	return &compactBuffer, nil
}

func (c *ChartCommand) handleYoutubeChannel(ctx *bot.InteractionContext, user *users.User, start, end carbon.Carbon, chartType string) error {
	channels, err := c.ar.GetTotalByUserIDGroupByVideoChannel(
		ctx.ResponseContext(),
		user.ID,
		start.ToStdTime(),
		end.ToStdTime(),
		9,
	)

	if err != nil {
		return err
	}

	totalTime, err := c.ar.GetTotalYouTubeWatchTimeByUserID(
		ctx.ResponseContext(),
		user.ID,
		start.ToStdTime(),
		end.ToStdTime(),
	)
	if err != nil {
		return err
	}

	totalMinutes := totalTime.Minutes()
	maxKeys := min(9, channels.Len())
	keys := make([]string, 0, maxKeys)
	values := make([]float64, 0, maxKeys)
	totalDisplayed := 0.0
	for i := 0; i < maxKeys; i++ {
		v, _ := channels.Get(channels.Keys()[i])
		k := v.ChannelName

		keys = append(keys, k)
		values = append(values, v.TotalDuration.Minutes())
		totalDisplayed += v.TotalDuration.Minutes()
	}

	if len(keys) == 9 {
		keys = append(keys, "Other")
		values = append(values, totalMinutes-totalDisplayed)
	}

	for i, v := range values {
		values[i] = math.Round(v)
	}

	var reqBody *bytes.Buffer

	if chartType != "pie" {
		reqBody, err = getQuickChartBarBody(keys, values, 0)
	} else {
		reqBody, err = getQuickChartChannelPieBody(keys, values)
	}

	if err != nil {
		return err
	}

	req := http.Request{
		Method: http.MethodPost,
		URL:    &quickChartURL,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(reqBody),
	}

	resp, err := http.DefaultClient.Do(&req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to generate chart")
	}

	description := fmt.Sprintf(
		"Here are your top channels from <t:%d> to <t:%d> . You logged a total of **%.0f minutes**. Here is a breakdown of your time:",
		start.Timestamp(),
		end.Timestamp(),
		totalMinutes,
	)

	embed := discordutil.NewEmbedBuilder().
		SetTitle("Top YouTube Channels").
		SetDescription(description).
		SetColor(discordutil.ColorPrimary).
		SetImage("attachment://chart.png")

	for i := 0; i < maxKeys; i++ {
		channelURL := fmt.Sprintf("https://www.youtube.com/channel/%s", channels.Keys()[i])
		percent := values[i] / totalMinutes * 100
		fieldTitle := fmt.Sprintf("%.0f minutes (%.0f%%)", values[i], percent)
		fieldValue := fmt.Sprintf("[%s](%s)", keys[i], channelURL)
		embed.AddField(fieldTitle, fieldValue, true)
	}

	return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed.MessageEmbed},
		Files: []*discordgo.File{
			{
				Name:        "chart.png",
				ContentType: resp.Header.Get("Content-Type"),
				Reader:      resp.Body,
			},
		},
	})
}

func (c *ChartCommand) Handle(ctx *bot.InteractionContext) error {
	userID := discordutil.GetInteractionUser(ctx.Interaction()).ID
	guildID := ctx.Interaction().GuildID
	user, err := c.ur.FindByID(ctx.ResponseContext(), userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
			Content: "You have no activity!",
		})
	} else if err != nil {
		return err
	}

	timezone := carbon.UTC

	if user != nil && user.Timezone != nil {
		timezone = *user.Timezone
	} else if guildID != "" {
		guild, err := c.gr.FindByID(ctx.ResponseContext(), guildID)

		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		if guild != nil && guild.Timezone != nil {
			timezone = *guild.Timezone
		}
	}

	start := carbon.Now(timezone).SubDays(6).StartOfDay()
	end := carbon.Now(timezone).EndOfDay()

	if len(ctx.Options()) == 0 {
		return bot.ErrInvalidOptions
	}

	subcommand := ctx.Options()[0]
	startInput := discordutil.GetStringOption(subcommand.Options, "start")
	endInput := discordutil.GetStringOption(subcommand.Options, "end")
	customTimeframe := startInput != nil || endInput != nil

	if startInput != nil {
		start = carbon.Parse(*startInput, timezone)
		if !start.IsValid() {
			return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
				Content: "Invalid start date.",
			})
		}
	}

	if endInput != nil {
		end = carbon.Parse(*endInput, timezone)

		if !end.IsValid() {
			return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
				Content: "Invalid end date.",
			})
		}
	}

	if end.Lt(start) {
		start, end = end, start
	}

	if subcommand.Name == "youtube-channel" {
		chartType := discordutil.GetStringOptionOrDefault(subcommand.Options, "type", "pie")

		return c.handleYoutubeChannel(ctx, user, start, end, chartType)
	}

	deltaMonths := end.DiffAbsInMonths(start)

	if deltaMonths > 36 {
		return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
			Content: "You can only view up to 36 months of activity.",
		})
	}

	useMonthGrouping := deltaMonths > 3

	var dailyDurations orderedmap.Map[time.Duration]

	if useMonthGrouping {
		dailyDurations, err = c.ar.GetTotalByUserIDGroupedByMonth(
			ctx.ResponseContext(),
			user.ID,
			ctx.Interaction().GuildID,
			start.ToStdTime(),
			end.ToStdTime(),
		)
	} else {
		dailyDurations, err = c.ar.GetTotalByUserIDGroupedByDay(
			ctx.ResponseContext(),
			user.ID,
			ctx.Interaction().GuildID,
			start.ToStdTime(),
			end.ToStdTime(),
		)
	}

	if err != nil {
		return err
	}

	totalMinutes := 0.0
	highestMinutes := 0.0
	highestDay := "N/A"

	values := make([]float64, 0, dailyDurations.Len())

	for _, k := range dailyDurations.Keys() {
		v, _ := dailyDurations.Get(k)
		totalMinutes += v.Minutes()
		if v.Minutes() > highestMinutes {
			highestMinutes = v.Minutes()
			highestDay = k
		}
		values = append(values, v.Minutes())
	}

	avgMinutes := totalMinutes / float64(dailyDurations.Len())
	goal := user.DailyGoal

	if useMonthGrouping {
		goal = 0
	}

	reqBody, err := getQuickChartBarBody(dailyDurations.Keys(), values, goal)

	if err != nil {
		return err
	}

	req := http.Request{
		Method: http.MethodPost,
		URL:    &quickChartURL,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(reqBody),
	}

	resp, err := http.DefaultClient.Do(&req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to generate chart")
	}

	embed := discordutil.NewEmbedBuilder().
		SetTitle("Activity History").
		SetColor(discordutil.ColorPrimary).
		SetImage("attachment://chart.png").
		AddField("Total", fmt.Sprintf("%.0f minutes", math.Round(totalMinutes)), true).
		AddField("Average", fmt.Sprintf("%.0f minutes", math.Round(avgMinutes)), true).
		AddField("Highest", fmt.Sprintf("%.0f minutes (%s)", math.Round(highestMinutes), highestDay), true)

	if customTimeframe {
		embed.SetDescription(fmt.Sprintf("Here is your activity from <t:%d> to <t:%d>", start.Timestamp(), end.Timestamp()))
	}

	return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed.MessageEmbed},
		Files: []*discordgo.File{
			{
				Name:        "chart.png",
				ContentType: resp.Header.Get("Content-Type"),
				Reader:      resp.Body,
			},
		},
	})
}
