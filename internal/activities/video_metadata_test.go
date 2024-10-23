package activities_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xoltia/botsu/internal/activities"
)

func TestGetVideoDataWithYtdlp(t *testing.T) {
	vidURL := "https://youtu.be/3T0wEUW1THE?si=Xk5UM1tnHWjwvWSh"
	u, _ := url.Parse(vidURL)
	data, err := activities.GetVideoInfo(context.TODO(), u, true)
	t.Log(data)
	assert.Nil(t, err)
	assert.Equal(t, vidURL, data.URL)
	assert.Equal(t, "God knows... / 博衣こより (Cover) 【歌ってみた】", data.Title)
	assert.Equal(t, "youtube", data.Platform)
	assert.Equal(t, "@HakuiKoyori", data.ChannelHandle)
	assert.Equal(t, "3T0wEUW1THE", data.ID)
	assert.Equal(t, time.Minute*4+time.Second*40, data.Duration)
}

func TestGetVideoData(t *testing.T) {
	vidURL := "https://youtu.be/3T0wEUW1THE?si=Xk5UM1tnHWjwvWSh"
	u, _ := url.Parse(vidURL)
	data, err := activities.GetVideoInfo(context.TODO(), u, false)
	t.Log(data)
	assert.Nil(t, err)
	assert.Equal(t, vidURL, data.URL)
	assert.Equal(t, "God knows... / 博衣こより (Cover) 【歌ってみた】", data.Title)
	assert.Equal(t, "youtube", data.Platform)
	assert.Equal(t, "@HakuiKoyori", data.ChannelHandle)
	assert.Equal(t, "3T0wEUW1THE", data.ID)
	assert.Equal(t, time.Minute*4+time.Second*40, data.Duration)
}
