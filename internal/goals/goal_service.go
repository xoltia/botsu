package goals

import (
	"context"
	"time"

	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/users"
)

type GoalService struct {
	*GoalRepository
	ts *users.UserTimeService
}

func NewGoalService(repo *GoalRepository, ts *users.UserTimeService) *GoalService {
	return &GoalService{repo, ts}
}

func (s *GoalService) NextCron(ctx context.Context, g *Goal) (t time.Time, err error) {
	location, err := s.ts.GetTimeLocation(ctx, g.UserID, "")
	if err != nil {
		return
	}
	now := time.Now().In(location)
	return g.NextDueTime(now)
}

func (s *GoalService) PreviousCron(ctx context.Context, g *Goal) (t time.Time, err error) {
	location, err := s.ts.GetTimeLocation(ctx, g.UserID, "")
	if err != nil {
		return
	}
	now := time.Now().In(location)
	return g.PreviousDueTime(now)
}

func (s *GoalService) CheckCompleted(ctx context.Context, a *activities.Activity) (completed []*Goal, err error) {
	now, err := s.ts.GetTime(ctx, a.UserID, "")
	if err != nil {
		return
	}

	goals, tx, err := s.BeginUpdateTxByUserID(ctx, a.UserID)
	if err != nil {
		return
	}

	defer tx.Rollback(ctx) //nolint:errcheck

	for _, g := range goals {
		changed := false
		if g.IsDue(now) {
			g.DueAt, err = g.NextDueTime(now)
			if err != nil {
				return
			}
			g.Current = 0
			changed = true
		}

		alreadyCompleted := g.Current >= g.Target
		if g.MatchesActivity(a) {
			g.Current += a.Duration
			changed = true
		}
		if g.Current >= g.Target && !alreadyCompleted {
			completed = append(completed, g)
		}

		if changed {
			err = s.UpdateTx(ctx, tx, g)
			if err != nil {
				return
			}
		}
	}

	err = tx.Commit(ctx)
	return
}

func (s *GoalService) CheckAll(ctx context.Context, userID string) (goals []*Goal, err error) {
	now, err := s.ts.GetTime(ctx, userID, "")
	if err != nil {
		return
	}

	goals, tx, err := s.BeginUpdateTxByUserID(ctx, userID)
	if err != nil {
		return
	}

	defer tx.Rollback(ctx) //nolint:errcheck

	for _, g := range goals {
		if g.IsDue(now) {
			g.DueAt, err = g.NextDueTime(now)
			if err != nil {
				return
			}
			g.Current = 0
			err = s.UpdateTx(ctx, tx, g)
			if err != nil {
				return
			}
		}
	}

	err = tx.Commit(ctx)
	return
}
