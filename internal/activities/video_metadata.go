package activities

import (
	"context"
	"log/slog"
	nurl "net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kkdai/youtube/v2"
	"github.com/wader/goutubedl"
	"github.com/xoltia/botsu/pkg/ytchannel"
)

var ytClient = youtube.Client{}

// video is either youtube.com/watch?v=ID or youtube.com/live/ID (for live streams) or youtu.be/ID
var ytVideoLinkRegex = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtube\.com/live/|youtu\.be/)([a-zA-Z0-9_-]+)`)
var ytHandleRegex = regexp.MustCompile(`(^|\s|youtu.*/)@([a-zA-Z0-9_-]+)($|\s)`)
var hashTagRegex = regexp.MustCompile(`#([^#\s\x{3000}]+)`)

var channelCache = sync.Map{}

func init() {
	youtube.DefaultClient = youtube.WebClient
	goutubedl.Path = "yt-dlp"
}

type VideoInfo struct {
	URL            string        `json:"url"`
	Platform       string        `json:"platform"`
	ID             string        `json:"video_id"`
	Title          string        `json:"video_title"`
	Duration       time.Duration `json:"video_duration"`
	ChannelID      string        `json:"channel_id"`
	ChannelName    string        `json:"channel_name"`
	ChannelHandle  string        `json:"channel_handle"`
	Thumbnail      string        `json:"thumbnail"`
	LinkedChannels []string      `json:"linked_channels,omitempty"`
	LinkedVideos   []string      `json:"linked_videos,omitempty"`
	HashTags       []string      `json:"hashtags,omitempty"`
}

func GetVideoInfo(ctx context.Context, url *nurl.URL, forceYtdlp bool) (v *VideoInfo, err error) {
	isYoutubeLink := url.Host == "youtu.be" ||
		url.Host == "youtube.com" ||
		url.Host == "www.youtube.com" ||
		url.Host == "m.youtube.com"

	logger, ok := ctx.Value("logger").(*slog.Logger)

	if !ok {
		logger = slog.Default()
	}

	logger.Debug(
		"Getting video info",
		slog.String("url", url.String()),
		slog.Bool("is_youtube_link", isYoutubeLink),
		slog.Bool("force_ytdlp", forceYtdlp),
		slog.String("host", url.Host),
	)

	if !forceYtdlp && isYoutubeLink {
		v, err = getInfoFromYoutube(ctx, url)

		if err != nil {
			logger.Warn(
				"Failed to get video info from youtube, falling back to yt-dlp",
				slog.String("url", url.String()),
				slog.String("error", err.Error()),
			)

			v, err = getGenericVideoInfo(ctx, url)
		}

		return
	}

	return getGenericVideoInfo(ctx, url)
}

func getGenericVideoInfo(ctx context.Context, url *nurl.URL) (v *VideoInfo, err error) {
	result, err := goutubedl.New(ctx, url.String(), goutubedl.Options{
		Type: goutubedl.TypeSingle,
	})

	if err != nil {
		return
	}

	info := result.Info

	v = &VideoInfo{
		URL:           url.String(),
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

func getInfoFromYoutube(ctx context.Context, url *nurl.URL) (v *VideoInfo, err error) {
	var video *youtube.Video

	if strings.HasPrefix(strings.ToLower(url.Path), "/live/") {
		parts := strings.Split(url.Path, "/")
		video, err = ytClient.GetVideoContext(ctx, parts[len(parts)-1])
	} else {
		video, err = ytClient.GetVideoContext(ctx, url.String())
	}

	if err != nil {
		return
	}

	var thumbnailURL string

	if len(video.Thumbnails) > 0 {
		thumbnailURL = video.Thumbnails[0].URL
	}

	v = &VideoInfo{
		URL:           url.String(),
		Platform:      "youtube",
		ID:            video.ID,
		Title:         video.Title,
		Duration:      video.Duration,
		ChannelID:     video.ChannelID,
		ChannelName:   video.Author,
		ChannelHandle: video.ChannelHandle,
		Thumbnail:     thumbnailURL,
	}

	if v.ChannelHandle == "" {
		if cached, ok := channelCache.Load(video.ChannelID); ok {
			v.ChannelHandle = cached.(string)
		} else {
			channel, err := ytchannel.GetYoutubeChannel(ctx, video.ChannelID)

			if err != nil {
				return nil, err
			}

			v.ChannelHandle = channel.Handle
			channelCache.Store(video.ChannelID, channel.Handle)
		}
	}

	highestRes := uint(0)
	highestResThumbnail := ""

	for _, thumbnail := range video.Thumbnails {
		res := thumbnail.Width * thumbnail.Height
		if res > highestRes {
			highestRes = res
			highestResThumbnail = thumbnail.URL
		}

		if res == 0 {
			highestResThumbnail = thumbnail.URL
		}
	}

	v.Thumbnail = highestResThumbnail
	v.LinkedChannels = findRelatedYoutubeChannels(video)
	v.LinkedVideos = findRelatedYoutubeVideos(video)
	v.HashTags = findHashTags(video)

	return
}

func findRelatedYoutubeChannels(video *youtube.Video) []string {
	relatedChannels := make([]string, 0)
	matches := ytHandleRegex.FindAllStringSubmatch(video.Description, -1)
	for _, match := range matches {
		relatedChannels = append(relatedChannels, "@"+match[2])
	}
	return relatedChannels
}

func findRelatedYoutubeVideos(video *youtube.Video) []string {
	relatedVideos := make([]string, 0)
	matches := ytVideoLinkRegex.FindAllStringSubmatch(video.Description, -1)
	for _, match := range matches {
		relatedVideos = append(relatedVideos, match[1])
	}
	return relatedVideos
}

func findHashTags(video *youtube.Video) []string {
	hashTags := make([]string, 0)
	matches := hashTagRegex.FindAllStringSubmatch(video.Description, -1)
	for _, match := range matches {
		hashTags = append(hashTags, "#"+strings.TrimSpace(match[1]))
	}
	return hashTags
}
