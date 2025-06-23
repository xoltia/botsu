package videos

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/kkdai/youtube/v2"
	"github.com/xoltia/botsu/internal/videos/ytchannel"
)

var ytClient = youtube.Client{}
var channelCache = sync.Map{}

var (
	videoRegex   = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtube\.com/live/|youtu\.be/)([a-zA-Z0-9_-]+)`)
	handleRegex  = regexp.MustCompile(`(?:^|\s|youtube.com/)(@[\p{L}0-9_-]+)`)
	channelRegex = regexp.MustCompile(`(?:youtube.com/channel/)(UC[a-zA-Z0-9_-]{21,})`)
	tagRegex     = regexp.MustCompile(`(?:^|\s)(#[^#\s\x{3000}\x{200B}]+)`)
)

func init() {
	youtube.DefaultClient = youtube.WebClient
}

func getInfoFromYouTubeBuiltin(ctx context.Context, videoURL *url.URL) (v *VideoInfo, err error) {
	var video *youtube.Video
	if strings.HasPrefix(strings.ToLower(videoURL.Path), "/live/") {
		parts := strings.Split(videoURL.Path, "/")
		video, err = ytClient.GetVideoContext(ctx, parts[len(parts)-1])
	} else {
		video, err = ytClient.GetVideoContext(ctx, videoURL.String())
	}

	if err != nil {
		return
	}

	var thumbnailURL string

	if len(video.Thumbnails) > 0 {
		thumbnailURL = video.Thumbnails[0].URL
	}

	v = &VideoInfo{
		URL:           videoURL.String(),
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
	v.LinkedChannels = findRelatedYoutubeChannels(video.Description)
	v.LinkedVideos = findRelatedYoutubeVideos(video.Description)
	v.HashTags = findHashTags(video.Description)

	return
}

func findRelatedYoutubeChannels(description string) []string {
	relatedChannels := make([]string, 0)
	handleMatches := handleRegex.FindAllStringSubmatch(description, -1)
	channelMatches := channelRegex.FindAllStringSubmatch(description, -1)
	for _, match := range handleMatches {
		relatedChannels = append(relatedChannels, match[1])
	}
	for _, match := range channelMatches {
		relatedChannels = append(relatedChannels, match[1])
	}
	return relatedChannels
}

func findRelatedYoutubeVideos(description string) []string {
	relatedVideos := make([]string, 0)
	matches := videoRegex.FindAllStringSubmatch(description, -1)
	for _, match := range matches {
		relatedVideos = append(relatedVideos, match[1])
	}
	return relatedVideos
}

func findHashTags(description string) []string {
	hashTags := make([]string, 0)
	matches := tagRegex.FindAllStringSubmatch(description, -1)
	for _, match := range matches {
		hashTags = append(hashTags, match[1])
	}
	return hashTags
}
