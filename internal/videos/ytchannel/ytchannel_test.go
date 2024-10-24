package ytchannel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xoltia/botsu/internal/videos/ytchannel"
)

func TestGetChannelFromHandle(t *testing.T) {
	channel, err := ytchannel.GetYoutubeChannel(context.TODO(), "@HakuiKoyori")

	assert.NoError(t, err)
	assert.Equal(t, "UC6eWCld0KwmyHFbAqK3V-Rw", channel.ID)
	assert.Equal(t, "Koyori ch. 博衣こより - holoX -", channel.Name)
	assert.Equal(t, "@HakuiKoyori", channel.Handle)
	assert.NotZero(t, len(channel.Banners))
	assert.NotZero(t, len(channel.Avatars))
}

func TestGetChannelFromID(t *testing.T) {
	channel, err := ytchannel.GetYoutubeChannel(context.TODO(), "UC6eWCld0KwmyHFbAqK3V-Rw")

	assert.NoError(t, err)
	assert.Equal(t, "UC6eWCld0KwmyHFbAqK3V-Rw", channel.ID)
	assert.Equal(t, "Koyori ch. 博衣こより - holoX -", channel.Name)
	assert.Equal(t, "@HakuiKoyori", channel.Handle)
	assert.NotZero(t, len(channel.Banners))
	assert.NotZero(t, len(channel.Avatars))
}
