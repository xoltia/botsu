package goals

import (
	"slices"
	"time"

	"github.com/adhocore/gronx"
	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/videos"
)

type Goal struct {
	ID              int64
	UserID          string
	Name            string
	ActivityType    *string
	MediaType       *string
	YoutubeChannels []string
	Target          time.Duration
	Current         time.Duration
	Cron            string
	DueAt           time.Time
	CreatedAt       time.Time
}

func (g *Goal) MatchesActivity(a *activities.Activity) bool {
	if g.ActivityType != nil && *g.ActivityType != a.PrimaryType {
		return false
	}

	if g.MediaType != nil && (a.MediaType == nil || *g.MediaType != *a.MediaType) {
		return false
	}

	if len(g.YoutubeChannels) == 0 {
		return true
	}

	meta, ok := a.Meta.(*videos.VideoInfo)
	if !ok {
		return false
	}

	if len(g.YoutubeChannels) > 0 && !slices.Contains(g.YoutubeChannels, meta.ChannelHandle) {
		return false
	}

	return true
}

func (g *Goal) IsDue(now time.Time) bool {
	return g.DueAt.Before(now)
}

func (g *Goal) NextDueTime(now time.Time) (t time.Time, err error) {
	if !g.IsDue(now) {
		return g.DueAt, nil
	}

	return gronx.NextTickAfter(g.Cron, now, true)
}

func (g *Goal) PreviousDueTime(now time.Time) (t time.Time, err error) {
	if g.IsDue(now) {
		return g.DueAt, nil
	}

	return gronx.PrevTickBefore(g.Cron, now, false)
}
