package discordutil

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

type GlobalMessageComponentCollector struct {
}

type InteractionFilter func(i *discordgo.InteractionCreate) bool

func CollectComponentInteractions(ctx context.Context, s *discordgo.Session, f InteractionFilter) <-chan *discordgo.InteractionCreate {
	// this channel closes
	ch := make(chan *discordgo.InteractionCreate)
	// this channel does not close
	middleCh := make(chan *discordgo.InteractionCreate)

	removeHandler := s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionMessageComponent && f(i) {
			middleCh <- i
		}
	})

	go func() {
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case i := <-middleCh:
				ch <- i
			}
		}

		close(ch)
		removeHandler()
	}()

	return ch
}

func CollectSingleComponentInteraction(ctx context.Context, s *discordgo.Session, f InteractionFilter) (*discordgo.InteractionCreate, error) {
	ch := make(chan *discordgo.InteractionCreate, 1)

	s.AddHandlerOnce(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionMessageComponent && f(i) {
			ch <- i
		}
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case i := <-ch:
		return i, nil
	}
}

func AcceptAllInteractionFilter(i *discordgo.InteractionCreate) bool {
	return true
}

func NewUserFilter(userID string) InteractionFilter {
	return func(i *discordgo.InteractionCreate) bool {
		return GetInteractionUser(i).ID == userID
	}
}

func NewMessageFilter(messageID string) InteractionFilter {
	return func(i *discordgo.InteractionCreate) bool {
		return i.Message.ID == messageID
	}
}

func NewMultiFilter(filters ...InteractionFilter) InteractionFilter {
	return func(i *discordgo.InteractionCreate) bool {
		for _, f := range filters {
			if !f(i) {
				return false
			}
		}
		return true
	}
}
