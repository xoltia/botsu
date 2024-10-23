package commands

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/bot"
	"github.com/xoltia/botsu/pkg/discordutil"
)

var ExportCommandData = &discordgo.ApplicationCommand{
	Name:        "export",
	Description: "Export your activities to a JSONL file.",
}

type ExportCommand struct {
	r         *activities.ActivityRepository
	history   map[string]time.Time
	historyMu sync.Mutex
}

func NewExportCommand(r *activities.ActivityRepository) *ExportCommand {
	return &ExportCommand{r: r, history: make(map[string]time.Time)}
}

func (c *ExportCommand) Handle(ctx *bot.InteractionContext) error {
	userID := discordutil.GetInteractionUser(ctx.Interaction()).ID

	c.historyMu.Lock()

	t, ok := c.history[userID]

	if ok && time.Since(t) < time.Hour*24 {
		c.historyMu.Unlock()

		return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
			Content: "You can only export your activities once per day.",
		})
	}

	c.history[userID] = time.Now()
	c.historyMu.Unlock()

	activities, err := c.r.GetAllByUserID(
		ctx.Context(),
		discordutil.GetInteractionUser(ctx.Interaction()).ID,
		ctx.Interaction().GuildID)

	if err != nil {
		return err
	}

	buffer := new(bytes.Buffer)
	compressed := gzip.NewWriter(buffer)
	jsonEncoder := json.NewEncoder(compressed)

	for _, activity := range activities {
		err = jsonEncoder.Encode(activity)

		if err != nil {
			return err
		}
	}

	// flush stream
	compressed.Close()

	filename := fmt.Sprintf("activities-%s-%s.jsonl.gz", userID, time.Now().Format(time.RFC3339))

	// send file
	return ctx.Respond(discordgo.InteractionResponseChannelMessageWithSource, &discordgo.InteractionResponseData{
		Files: []*discordgo.File{
			{
				Name:        filename,
				ContentType: "application/gzip",
				Reader:      buffer,
			},
		},
	})
}
