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

// video is either youtube.com/watch?v=ID or youtube.com/live/ID (for live streams) or youtu.be/ID
var (
	ytVideoLinkRegex = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtube\.com/live/|youtu\.be/)([a-zA-Z0-9_-]+)`)
	ytHandleRegex    = regexp.MustCompile(`(^|\s|youtu.*/)@([a-zA-Z0-9_-]+)($|\s)`)
	hashTagRegex     = regexp.MustCompile(`#([^#\s\x{3000}]+)`)
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
