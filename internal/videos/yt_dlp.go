package videos

import (
	"context"
	"net/url"
	"time"

	"github.com/wader/goutubedl"
)

func init() {
	goutubedl.Path = "yt-dlp"
}

func getGenericVideoInfo(ctx context.Context, videoURL *url.URL) (v *VideoInfo, err error) {
	result, err := goutubedl.New(ctx, videoURL.String(), goutubedl.Options{
		Type: goutubedl.TypeSingle,
	})

	if err != nil {
		return
	}

	info := result.Info

	v = &VideoInfo{
		URL:           videoURL.String(),
		Platform:      info.Extractor,
		ID:            info.ID,
		Title:         info.Title,
		Duration:      time.Duration(info.Duration) * time.Second,
		ChannelID:     info.ChannelID,
		ChannelName:   info.Channel,
		ChannelHandle: info.UploaderID,
		Thumbnail:     info.Thumbnail,
	}

	if v.ChannelName == "" {
		v.ChannelName = info.Uploader
	}

	return
}
