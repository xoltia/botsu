package videos

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var service *youtube.Service
var ErrVideoNotFound = errors.New("video not found")

func EnableYouTubeAPI(apiKey string) error {
	var err error
	service, err = youtube.NewService(context.Background(), option.WithAPIKey(apiKey))
	return err
}

func youtubeAPIEnabled() bool {
	return service != nil
}

func getInfoFromYouTubeAPI(ctx context.Context, videoURL *url.URL) (v *VideoInfo, err error) {
	var result *youtube.VideoListResponse
	parts := ytVideoLinkRegex.FindStringSubmatch(videoURL.String())
	if len(parts) < 2 {
		err = errors.New("invalid youtube video URL")
		return
	}

	result, err = service.Videos.
		List([]string{"snippet", "contentDetails"}).
		Context(ctx).
		Id(parts[1]).
		Do()

	if err != nil {
		return
	}

	if len(result.Items) == 0 {
		err = ErrVideoNotFound
		return
	}

	snippet := result.Items[0].Snippet
	contentDetails := result.Items[0].ContentDetails
	if snippet == nil || contentDetails == nil {
		err = ErrVideoNotFound
		return
	}

	thumbnails := snippet.Thumbnails
	if thumbnails == nil {
		err = ErrVideoNotFound
		return
	}

	thumbnailURL := ""
	switch {
	case snippet.Thumbnails.Maxres != nil:
		thumbnailURL = snippet.Thumbnails.Maxres.Url
	case snippet.Thumbnails.High != nil:
		thumbnailURL = snippet.Thumbnails.High.Url
	case snippet.Thumbnails.Medium != nil:
		thumbnailURL = snippet.Thumbnails.Medium.Url
	case snippet.Thumbnails.Standard != nil:
		thumbnailURL = snippet.Thumbnails.Standard.Url
	case snippet.Thumbnails.Default != nil:
		thumbnailURL = snippet.Thumbnails.Default.Url
	default:
		err = ErrVideoNotFound
		return
	}

	v = &VideoInfo{
		URL:         videoURL.String(),
		Platform:    "youtube",
		ID:          result.Items[0].Id,
		Title:       snippet.Title,
		ChannelID:   snippet.ChannelId,
		ChannelName: snippet.ChannelTitle,
		Thumbnail:   thumbnailURL,
		Duration:    parseYouTubeAPITime(contentDetails.Duration),
		//ChannelHandle: result.Items[0].Snippet.ChannelTitle,
	}
	return
}

// Format: PT#H#M#S
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
}
