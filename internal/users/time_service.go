package users

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xoltia/botsu/internal/guilds"
)

type UserTimeService struct {
	Default string
	u       *UserRepository
	g       *guilds.GuildRepository
}

func NewUserTimeService(u *UserRepository, g *guilds.GuildRepository) *UserTimeService {
	return &UserTimeService{u: u, g: g, Default: "UTC"}
}

func (s *UserTimeService) GetTimezone(ctx context.Context, userID, guildID string) (string, error) {
	user, err := s.u.FindByID(ctx, userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return s.getGuildDefaultTimezone(ctx, guildID)
	} else if err != nil {
		return "", err
	}

	if user.Timezone == nil {
		return s.getGuildDefaultTimezone(ctx, guildID)
	}

	return *user.Timezone, nil
}

func (s *UserTimeService) GetTimeLocation(ctx context.Context, userID, guildID string) (*time.Location, error) {
	timezone, err := s.GetTimezone(ctx, userID, guildID)

	if err != nil {
		return nil, err
	}

	return time.LoadLocation(timezone)
}

func (s *UserTimeService) GetTime(ctx context.Context, userID, guildID string) (time.Time, error) {
	location, err := s.GetTimeLocation(ctx, userID, guildID)

	if err != nil {
		return time.Time{}, err
	}

	return time.Now().In(location), nil
}

func (s *UserTimeService) getGuildDefaultTimezone(ctx context.Context, guildID string) (string, error) {
	if guildID == "" {
		return s.Default, nil
	}

	guild, err := s.g.FindByID(ctx, guildID)

	if errors.Is(err, pgx.ErrNoRows) {
		return s.Default, nil
	} else if err != nil {
		return "", err
	}

	if guild.Timezone == nil {
		return s.Default, nil
	}

	return *guild.Timezone, nil
}
