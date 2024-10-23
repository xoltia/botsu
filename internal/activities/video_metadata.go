package activities

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kkdai/youtube/v2"
	"github.com/wader/goutubedl"
	"github.com/xoltia/botsu/pkg/ytchannel"
	"google.golang.org/api/option"
	youtubeAPI "google.golang.org/api/youtube/v3"
)

// TODO: Move this to a separate package

var ytClient = youtube.Client{}
var ytAPIService *youtubeAPI.Service
var channelCache = sync.Map{}

// video is either youtube.com/watch?v=ID or youtube.com/live/ID (for live streams) or youtu.be/ID
var (
	ytVideoLinkRegex = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtube\.com/live/|youtu\.be/)([a-zA-Z0-9_-]+)`)
	ytHandleRegex    = regexp.MustCompile(`(^|\s|youtu.*/)@([a-zA-Z0-9_-]+)($|\s)`)
	hashTagRegex     = regexp.MustCompile(`#([^#\s\x{3000}]+)`)
)

var ErrVideoNotFound = errors.New("video not found")

func init() {
	youtube.DefaultClient = youtube.WebClient
	goutubedl.Path = "yt-dlp"
}

func SetupYoutubeAPI(apiKey string) error {
	var err error
	ytAPIService, err = youtubeAPI.NewService(context.Background(), option.WithAPIKey(apiKey))
	return err
}

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

func GetVideoInfo(ctx context.Context, videoURL *url.URL, forceYtdlp bool) (v *VideoInfo, err error) {
	isYoutubeLink := videoURL.Host == "youtu.be" ||
		videoURL.Host == "youtube.com" ||
		videoURL.Host == "www.youtube.com" ||
		videoURL.Host == "m.youtube.com"

	logger, ok := ctx.Value("logger").(*slog.Logger)

	if !ok {
		logger = slog.Default()
	}

	logger.Debug(
		"Getting video info",
		slog.String("url", videoURL.String()),
		slog.Bool("is_youtube_link", isYoutubeLink),
		slog.Bool("force_ytdlp", forceYtdlp),
		slog.String("host", videoURL.Host),
	)

	if !forceYtdlp && isYoutubeLink {
		v, err = getInfoFromYoutube(ctx, videoURL)

		if err != nil {
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

// Format: PT#D#H#M#S
func parseYouTubeAPITime(duration string) time.Duration {
	if !strings.HasPrefix(duration, "PT") {
		return 0
	}

	duration = duration[2:]
	components := []struct {
		unit       string
		multiplier time.Duration
	}{
		{"H", time.Hour},
		{"M", time.Minute},
		{"S", time.Second},
	}

	var total time.Duration
	for _, component := range components {
		index := strings.Index(duration, component.unit)
		if index == -1 {
			continue
		}

		value, _ := strconv.Atoi(duration[:index])
		total += time.Duration(value) * component.multiplier
		duration = duration[index+1:]
	}

	return total
	// days := strings.Index(duration, "D")
	// hours := strings.Index(duration, "H")
	// minutes := strings.Index(duration, "M")
	// seconds := strings.Index(duration, "S")

	// var d, h, m, s int
	// if days != -1 {
	// 	d, _ = strconv.Atoi(duration[:days])
	// }
	// if hours != -1 {
	// 	h, _ = strconv.Atoi(duration[days+1 : hours])
	// }
	// if minutes != -1 {
	// 	m, _ = strconv.Atoi(duration[hours+1 : minutes])
	// }
	// if seconds != -1 {
	// 	s, _ = strconv.Atoi(duration[minutes+1 : seconds])
	// }
	// return time.Duration(d*24*60*60+h*60*60+m*60+s) * time.Second
}

func getInfoFromYoutube(ctx context.Context, videoURL *url.URL) (v *VideoInfo, err error) {
	if ytAPIService != nil {
		var result *youtubeAPI.VideoListResponse

		parts := ytVideoLinkRegex.FindStringSubmatch(videoURL.String())
		if len(parts) < 2 {
			err = errors.New("invalid youtube video URL")
			return
		}

		result, err = ytAPIService.Videos.
			List([]string{"snippet", "contentDetails"}).
			Id(parts[1]).
			Do()

		if err != nil {
			return
		}

		if len(result.Items) == 0 {
			err = ErrVideoNotFound
			return
		}

		v = &VideoInfo{
			URL:         videoURL.String(),
			Platform:    "youtube",
			ID:          result.Items[0].Id,
			Title:       result.Items[0].Snippet.Title,
			ChannelID:   result.Items[0].Snippet.ChannelId,
			ChannelName: result.Items[0].Snippet.ChannelTitle,
			Thumbnail:   result.Items[0].Snippet.Thumbnails.Maxres.Url,
			Duration:    parseYouTubeAPITime(result.Items[0].ContentDetails.Duration),
			//ChannelHandle: result.Items[0].Snippet.ChannelTitle,
		}
		return
	}

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
