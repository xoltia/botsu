package main

import (
	"context"
	"net/url"
	"time"

	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/internal/videos"
)

type aibActivity struct {
	ID              string
	UserID          string
	Name            string
	Type            string
	URL             *string
	Date            uint64
	Duration        float32
	RawDuration     *uint64
	RawDurationUnit *string
	Speed           *float32
	Tags            []string
}

var mediaTypeTags = map[string]string{
	"anime": activities.ActivityMediaTypeAnime,
	"vn":    activities.ActivityMediaTypeVisualNovel,
	"manga": activities.ActivityMediaTypeManga,
	"book":  activities.ActivityMediaTypeBook,
	"video": activities.ActivityMediaTypeVideo,
}

func (activity *aibActivity) asNewFormatActivity() (a *activities.Activity) {
	a = &activities.Activity{
		UserID:      activity.UserID,
		Name:        activity.Name,
		PrimaryType: activity.Type,
		Date:        time.UnixMilli(int64(activity.Date)),
		Duration:    time.Duration(activity.Duration * float32(time.Minute)),
		Meta:        make(map[string]interface{}),
	}

	for _, tag := range activity.Tags {
		if mediaType, ok := mediaTypeTags[tag]; ok {
			a.MediaType = &mediaType
			break
		}
	}

	if activity.URL != nil {
		a.SetMeta("url", *activity.URL)
	}

	if activity.Speed != nil {
		a.SetMeta("speed", *activity.Speed)
	}

	if activity.RawDuration != nil && activity.RawDurationUnit != nil {
		a.SetMeta(*activity.RawDurationUnit+"s", *activity.RawDuration)
	}

	return
}

func populateVideoMetadata(ctx context.Context, a *activities.Activity, vidURL string) (err error) {
	var u *url.URL
	u, err = url.Parse(vidURL)

	if err != nil {
		return
	}

	var meta *videos.VideoInfo
	meta, err = videos.GetVideoInfo(ctx, u, videos.Options{})

	if err != nil {
		return
	}

	a.Meta = meta

	return
}
