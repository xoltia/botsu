package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xoltia/botsu/pkg/discordutil"
)

var ErrResponseNotSent = errors.New("response not yet sent")

type InteractionContext struct {
	Logger *slog.Logger
	Bot    *Bot

	s *discordgo.Session
	i *discordgo.InteractionCreate
	// cancels when interaction token is invalidated
	ctx       context.Context
	ctxCancel context.CancelFunc
	// cancels when interaction response deadline is reached
	responseCtx       context.Context
	responseCtxCancel context.CancelFunc
	data              discordgo.ApplicationCommandInteractionData
	deferred          bool
}

func NewInteractionContext(
	logger *slog.Logger,
	bot *Bot,
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	ctx context.Context,
) *InteractionContext {
	// Should be about 3 seconds from the time Discord sends the interaction.
	responseDeadline := discordutil.GetInteractionResponseDeadline(i.Interaction)
	// Should be about 15 minutes later.
	interactionDeadline := discordutil.GetInteractionFollowupDeadline(i.Interaction)

	// Add some just in case the time from the interaction timestamp
	// and the time it is actually reaching the bot are greatly different.
	responseDeadline = responseDeadline.Add(time.Second * 3)

	interactionDeadlineContext, cancel := context.WithDeadline(ctx, interactionDeadline)
	responseDeadlineContext, cancel2 := context.WithDeadline(ctx, responseDeadline)

	logger.Debug(
		"Creating interaction context",
		slog.Time("interaction_deadline", interactionDeadline),
		slog.Time("response_deadline", responseDeadline),
	)

	return &InteractionContext{
		Logger:            logger,
		Bot:               bot,
		s:                 s,
		i:                 i,
		ctx:               interactionDeadlineContext,
		ctxCancel:         cancel,
		responseCtx:       responseDeadlineContext,
		responseCtxCancel: cancel2,
		data:              i.ApplicationCommandData(),
	}
}

func (c *InteractionContext) Cancel() {
	c.ctxCancel()
	c.responseCtxCancel()
}

func (c *InteractionContext) Session() *discordgo.Session {
	return c.s
}

func (c *InteractionContext) Interaction() *discordgo.InteractionCreate {
	return c.i
}

func (c *InteractionContext) User() *discordgo.User {
	return discordutil.GetInteractionUser(c.i)
}

// Returns a context that is cancelled when the interaction token is invalidated
func (c *InteractionContext) Context() context.Context {
	return c.ctx
}

// Returns a context that is cancelled when the interaction response deadline is reached
// or when a response is sent
func (c *InteractionContext) ResponseContext() context.Context {
	return c.responseCtx
}

func (c *InteractionContext) Data() discordgo.ApplicationCommandInteractionData {
	return c.data
}

func (c *InteractionContext) Options() []*discordgo.ApplicationCommandInteractionDataOption {
	return c.data.Options
}

func (c *InteractionContext) IsAutocomplete() bool {
	return c.i.Type == discordgo.InteractionApplicationCommandAutocomplete
}

func (c *InteractionContext) IsCommand() bool {
	return c.i.Type == discordgo.InteractionApplicationCommand
}

func (c *InteractionContext) Responded() bool {
	return c.responseCtx.Err() == context.Canceled
}

func (c *InteractionContext) CanRespond() bool {
	return c.responseCtx.Err() == nil
}

func (c *InteractionContext) Deferred() bool {
	return c.deferred
}

func (c *InteractionContext) DeferResponse() error {
	err := c.Respond(discordgo.InteractionResponseDeferredChannelMessageWithSource, nil)
	if err != nil {
		return fmt.Errorf("defer response: %w", err)
	}
	return nil
}

func (c *InteractionContext) Respond(responseType discordgo.InteractionResponseType, data *discordgo.InteractionResponseData) error {
	if !c.CanRespond() {
		return fmt.Errorf("response context: %w", c.responseCtx.Err())
	}

	if responseType == discordgo.InteractionResponseDeferredChannelMessageWithSource {
		c.deferred = true
	}

	err := c.s.InteractionRespond(c.i.Interaction, &discordgo.InteractionResponse{
		Type: responseType,
		Data: data,
	}, discordgo.WithContext(c.responseCtx))

	if err != nil {
		return fmt.Errorf("send response: %w", err)
	}

	c.responseCtxCancel()

	return nil
}

func (c *InteractionContext) Followup(response *discordgo.WebhookParams, wait bool) (*discordgo.Message, error) {
	if c.CanRespond() {
		return nil, fmt.Errorf("followup: %w", ErrResponseNotSent)
	}

	msg, err := c.s.FollowupMessageCreate(c.i.Interaction, wait, response, discordgo.WithContext(c.ctx))
	if err != nil {
		return nil, fmt.Errorf("followup create message: %w", err)
	}
	return msg, nil
}

func (c *InteractionContext) RespondOrFollowup(params *discordgo.WebhookParams, wait bool) (*discordgo.Message, error) {
	if !c.Responded() {
		data := discordgo.InteractionResponseData{
			TTS:             params.TTS,
			Content:         params.Content,
			Components:      params.Components,
			Embeds:          params.Embeds,
			AllowedMentions: params.AllowedMentions,
			Flags:           params.Flags,
		}

		err := c.Respond(discordgo.InteractionResponseChannelMessageWithSource, &data)
		if err != nil {
			return nil, fmt.Errorf("respond or followup: %w", err)
		}
		return nil, nil
	}

	msg, err := c.Followup(params, wait)
	if err != nil {
		return nil, fmt.Errorf("respond or followup: %w", err)
	}
	return msg, nil
}

func (c *InteractionContext) EditResponse(params *discordgo.WebhookEdit) (*discordgo.Message, error) {
	if c.CanRespond() {
		return nil, fmt.Errorf("edit response: %w", ErrResponseNotSent)
	}

	msg, err := c.s.InteractionResponseEdit(c.i.Interaction, params, discordgo.WithContext(c.ctx))
	if err != nil {
		return nil, fmt.Errorf("edit response: %w", err)
	}
	return msg, nil
}
