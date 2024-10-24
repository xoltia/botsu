package videos

import (
	"context"
	"log/slog"
	"net/url"
	"slices"
	"time"
)

type VideoInfo struct {
	URL            string        `json:"url"`
	Platform       string        `json:"platform"`
	ID             string        `json:"video_id"`
	Title          string        `json:"video_title"`
	Duration       time.Duration `json:"video_duration"`
	ChannelID      string        `json:"channel_id"`
	ChannelName    string        `json:"channel_name"`
	ChannelHandle  string        `json:"channel_handle,omitempty"`
	Thumbnail      string        `json:"thumbnail"`
	LinkedChannels []string      `json:"linked_channels,omitempty"`
	LinkedVideos   []string      `json:"linked_videos,omitempty"`
	HashTags       []string      `json:"hashtags,omitempty"`
}

type Options struct {
	// Do not attempt to use kkdai/youtube before trying to use yt-dlp
	SkipFastYouTube bool
	// Disable retry attempts when kkdai/youtube fails
	DisableFastYouTubeRetry bool
}

var youtubeHosts = []string{
	"youtu.be",
	"youtube.com",
	"www.youtube.com",
	"m.youtube.com",
}

func IsYouTubeLink(videoURL *url.URL) bool {
	return slices.Contains(youtubeHosts, videoURL.Host)
}

func GetVideoInfo(ctx context.Context, videoURL *url.URL, opts Options) (v *VideoInfo, err error) {
	isYouTubeLink := IsYouTubeLink(videoURL)
	logger, ok := ctx.Value("logger").(*slog.Logger)
	if !ok {
		logger = slog.Default()
	}

	logger.Debug(
		"Getting video info",
		slog.String("url", videoURL.String()),
		slog.Bool("is_youtube_link", isYouTubeLink),
		slog.Bool("force_ytdlp", opts.SkipFastYouTube),
		slog.Bool("google_api_enabled", youtubeAPIEnabled()),
		slog.String("host", videoURL.Host),
	)

	if isYouTubeLink && youtubeAPIEnabled() {
		v, err = getInfoFromYouTubeAPI(ctx, videoURL)
		return
	}

	if !opts.SkipFastYouTube && isYouTubeLink {
		v, err = getInfoFromYouTubeBuiltin(ctx, videoURL)
		if err != nil && !opts.DisableFastYouTubeRetry {
			logger.Warn(
				"Failed to get video info from youtube, falling back to yt-dlp",
				slog.String("url", videoURL.String()),
				slog.String("error", err.Error()),
			)
			v, err = getGenericVideoInfo(ctx, videoURL)
		}
		return
	}

	return getGenericVideoInfo(ctx, videoURL)
}
