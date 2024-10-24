package bot

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/xoltia/botsu/internal/guilds"
	"github.com/xoltia/botsu/pkg/discordutil"
)

var unexpectedErrorMessage = &discordgo.WebhookParams{
	Content: "An unexpected error occurred!",
}

type Bot struct {
	logger                   *slog.Logger
	session                  *discordgo.Session
	createdCommands          []*discordgo.ApplicationCommand
	commands                 CommandCollection
	guildRepo                *guilds.GuildRepository
	noPanic                  bool
	destroyOnClose           bool
	globalComponentCollector *discordutil.MessageComponentCollector
	wg                       sync.WaitGroup
	removeInteractionHandler func()
	botContext               context.Context
	cancelBotContext         context.CancelFunc
}

func NewBot(logger *slog.Logger, guildRepo *guilds.GuildRepository) *Bot {
	bot := &Bot{
		logger:    logger,
		commands:  NewCommandCollection(),
		guildRepo: guildRepo,
	}

	bot.botContext, bot.cancelBotContext = context.WithCancel(context.Background())

	return bot
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	b.logger.Info("Bot is ready", slog.String("user", r.User.String()))
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	b.logger.Debug(
		"Interaction received",
		slog.String("interaction", i.Interaction.ID),
		slog.String("user", discordutil.GetInteractionUser(i).String()),
		slog.String("guild", i.Interaction.GuildID),
		slog.String("type", i.Type.String()),
	)
	if i.Type != discordgo.InteractionApplicationCommand && i.Type != discordgo.InteractionApplicationCommandAutocomplete {
		return
	}

	subLogger := b.logger.
		WithGroup("interaction").
		With(slog.String("id", i.Interaction.ID)).
		With(slog.String("user", discordutil.GetInteractionUser(i).String())).
		With(slog.String("guild", i.Interaction.GuildID)).
		With(slog.String("type", i.Type.String())).
		With(slog.String("command", i.ApplicationCommandData().Name)).
		WithGroup("handler")

	ctx := NewInteractionContext(subLogger, b, s, i, b.botContext)

	defer ctx.Cancel()

	if b.noPanic {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				ctx.Logger.Error("Panic occurred", slog.Any("panic", r), slog.Any("stack", string(stack)))
				_, err := ctx.RespondOrFollowup(unexpectedErrorMessage, false)
				if err != nil {
					ctx.Logger.Error("Failed to send error message", slog.String("err", err.Error()))
				}
			}
		}()
	}

	b.wg.Add(1)
	defer b.wg.Done()

	err := b.commands.Handle(ctx)
	if err != nil {
		ctx.Logger.Error("Failed to handle interaction", slog.String("err", err.Error()))

		if ctx.IsCommand() {
			_, err = ctx.RespondOrFollowup(unexpectedErrorMessage, false)
			if err != nil {
				ctx.Logger.Error("Failed to send error message", slog.String("err", err.Error()))
			}
		}
	}
}

func (b *Bot) onMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	b.logger.Debug("Member left", slog.String("guild", m.GuildID), slog.String("user", m.User.String()))
	err := b.guildRepo.RemoveMembers(context.Background(), m.GuildID, []string{m.User.ID})
	if err != nil {
		b.logger.Error("Failed to remove member", slog.String("err", err.Error()))
	}
}

func (b *Bot) SetNoPanic(noPanic bool) {
	b.logger.Debug("Setting no panic", slog.Bool("no_panic", noPanic))
	b.noPanic = noPanic
}

func (b *Bot) SetDestroyCommandsOnClose(destroy bool) {
	b.logger.Debug("Setting destroy commands on close", slog.Bool("destroy", destroy))
	b.destroyOnClose = destroy
}

func (b *Bot) NewMessageComponentInteractionChannel(
	ctx context.Context,
	msg *discordgo.Message,
	filters ...discordutil.InteractionFilter,
) (<-chan *discordgo.InteractionCreate, error) {
	filter := discordutil.NewMultiFilter(filters...)
	return b.globalComponentCollector.Collect(ctx, msg.ID, filter)
}

func (b *Bot) CollectSingleComponentInteraction(
	ctx context.Context,
	msg *discordgo.Message,
	filters ...discordutil.InteractionFilter,
) (*discordgo.InteractionCreate, error) {
	filter := discordutil.NewMultiFilter(filters...)
	return b.globalComponentCollector.CollectOnce(ctx, msg.ID, filter)
}

func (b *Bot) AddCommand(data *discordgo.ApplicationCommand, cmd CommandHandler) {
	b.logger.Debug("Adding command", slog.String("command_name", data.Name))
	b.commands.Add(data, cmd)
}

func (b *Bot) Login(token string, intent discordgo.Intent) error {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return err
	}

	b.globalComponentCollector = discordutil.NewMessageComponentCollector(s)
	s.AddHandler(b.onReady)
	b.removeInteractionHandler = s.AddHandler(b.onInteractionCreate)
	s.AddHandler(b.onMemberRemove)

	s.Identify.Intents = intent

	if err = s.Open(); err != nil {
		return err
	}

	b.session = s
	b.logger.Info("Creating commands")

	cmdData := make([]*discordgo.ApplicationCommand, 0, len(b.commands))

	for _, cmd := range b.commands {
		cmdData = append(cmdData, cmd.Data)
	}

	b.createdCommands, err = b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, "", cmdData)
	if err != nil {
		return err
	}

	return nil
}

func (b *Bot) Close() {
	b.logger.Debug("Close() called")

	if b.destroyOnClose {
		b.logger.Debug("Destroying commands")
		_, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, "", []*discordgo.ApplicationCommand{})
		if err != nil {
			b.logger.Error("Failed to destroy commands", slog.String("err", err.Error()))
		}
	}

	// Stop accepting component interactions
	b.globalComponentCollector.Close()
	// Stop accepting command interactions
	b.removeInteractionHandler()
	// Cancel bot context (parent context of all interaction contexts)
	b.cancelBotContext()
	// Wait for already running commands to finish
	b.wg.Wait()
	// Close session
	b.session.Close()
}
