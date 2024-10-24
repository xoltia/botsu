package ytchannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var HTTPClient = http.DefaultClient
var ytInitialDataRegex = regexp.MustCompile(`var ytInitialData\s*=\s*(\{.+?\});`)

var ErrInvalidChannelIdentifier = errors.New("invalid channel identifier string, you should either provide a handle (starting with '@') or an ID (starting with 'UC')")
var ErrYtInitialDataNotFound = errors.New("could not find ytInitialData")

type ytInitialData struct {
	Metadata struct {
		ChannelMetadataRenderer struct {
			Title            string `json:"title"`
			ExternalID       string `json:"externalId"`
			VanityChannelURL string `json:"vanityChannelUrl"`
		}
	}
	Header struct {
		PageHeaderRenderer struct {
			Content struct {
				PageHeaderViewModel struct {
					Banner struct {
						ImageBannerViewModel struct {
							Image struct {
								Sources []*Thumbail `json:"sources"`
							} `json:"image"`
						} `json:"imageBannerViewModel"`
					} `json:"banner"`
					Image struct {
						DecoratedAvatarViewModel struct {
							Avatar struct {
								AvatarViewModel struct {
									Image struct {
										Sources []*Thumbail `json:"sources"`
									} `json:"image"`
								} `json:"avatarViewModel"`
							} `json:"avatar"`
						} `json:"decoratedAvatarViewModel"`
					} `json:"image"`
				} `json:"pageHeaderViewModel"`
			} `json:"content"`
		} `json:"pageHeaderRenderer"`
	} `json:"header"`
}

type Thumbail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Channel struct {
	ID      string
	Name    string
	Handle  string
	Avatars []*Thumbail
	Banners []*Thumbail
}

func GetYoutubeChannel(ctx context.Context, handleOrID string) (ch *Channel, err error) {
	var profileURL string

	if strings.HasPrefix(handleOrID, "@") {
		profileURL = fmt.Sprintf("https://youtube.com/%s", handleOrID)
	} else if strings.HasPrefix(handleOrID, "UC") {
		profileURL = fmt.Sprintf("https://youtube.com/channel/%s", handleOrID)
	} else {
		err = ErrInvalidChannelIdentifier
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var initialData ytInitialData
	data := ytInitialDataRegex.FindSubmatch(body)

	if len(data) < 2 {
		err = ErrYtInitialDataNotFound
		return
	}

	if err = json.Unmarshal(data[1], &initialData); err != nil {
		return
	}

	handleIndex := strings.LastIndex(initialData.Metadata.ChannelMetadataRenderer.VanityChannelURL, "/")
	if handleIndex == -1 {
		err = errors.New("could not find handle")
		return
	}
	handle := initialData.Metadata.ChannelMetadataRenderer.VanityChannelURL[handleIndex+1:]
	if len(handle) == 0 || handle[0] != '@' {
		err = errors.New("could not find handle")
		return
	}

	// TODO: fix banners and avatars
	ch = &Channel{
		Name:    initialData.Metadata.ChannelMetadataRenderer.Title,
		ID:      initialData.Metadata.ChannelMetadataRenderer.ExternalID,
		Handle:  handle,
		Avatars: initialData.Header.PageHeaderRenderer.Content.PageHeaderViewModel.Image.DecoratedAvatarViewModel.Avatar.AvatarViewModel.Image.Sources,
		Banners: initialData.Header.PageHeaderRenderer.Content.PageHeaderViewModel.Banner.ImageBannerViewModel.Image.Sources,
	}

	return
}
