package discordutil

import (
	"image/color"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	ColorPrimary   = color.RGBA{R: 0xF3, G: 0xB6, B: 0xAF, A: 0xFF}
	ColorSecondary = color.RGBA{R: 0xEC, G: 0xD1, B: 0xA0, A: 0xFF}
	ColorSuccess   = color.RGBA{R: 0x36, G: 0x9E, B: 0x42, A: 0xFF}
	ColorWarning   = color.RGBA{R: 0xED, G: 0xB6, B: 0x3E, A: 0xFF}
	ColorDanger    = color.RGBA{R: 0xE0, G: 0x38, B: 0x38, A: 0xFF}
	ColorInfo      = color.RGBA{R: 0x57, G: 0x8B, B: 0xF2, A: 0xFF}
)

type EmbedBuilder struct {
	embed *discordgo.MessageEmbed
}

func NewEmbedBuilder() *EmbedBuilder {
	return &EmbedBuilder{
		embed: &discordgo.MessageEmbed{},
	}
}

func (b *EmbedBuilder) SetTitle(title string) *EmbedBuilder {
	b.embed.Title = title
	return b
}

func (b *EmbedBuilder) SetDescription(description string) *EmbedBuilder {
	b.embed.Description = description
	return b
}

func (b *EmbedBuilder) SetColor(c color.Color) *EmbedBuilder {
	red, green, blue, _ := c.RGBA()
	b.embed.Color = int(red>>8)<<16 + int(green>>8)<<8 + int(blue>>8)
	return b
}

func (b *EmbedBuilder) SetColorFromInt(c int) *EmbedBuilder {
	b.embed.Color = c
	return b
}

func (b *EmbedBuilder) SetAuthor(name, iconUrl, url string) *EmbedBuilder {
	b.embed.Author = &discordgo.MessageEmbedAuthor{
		Name:    name,
		IconURL: iconUrl,
		URL:     url,
	}
	return b
}

func (b *EmbedBuilder) SetFooter(text, iconUrl string) *EmbedBuilder {
	b.embed.Footer = &discordgo.MessageEmbedFooter{
		Text:    text,
		IconURL: iconUrl,
	}
	return b
}

func (b *EmbedBuilder) SetThumbnail(url string) *EmbedBuilder {
	b.embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
		URL: url,
	}
	return b
}

func (b *EmbedBuilder) SetImage(url string) *EmbedBuilder {
	b.embed.Image = &discordgo.MessageEmbedImage{
		URL: url,
	}
	return b
}

func (b *EmbedBuilder) AddField(name, value string, inline bool) *EmbedBuilder {
	b.embed.Fields = append(b.embed.Fields, &discordgo.MessageEmbedField{
		Name:   name,
		Value:  value,
		Inline: inline,
	})
	return b
}

func (b *EmbedBuilder) ClearFields() *EmbedBuilder {
	b.embed.Fields = make([]*discordgo.MessageEmbedField, 0)
	return b
}

func (b *EmbedBuilder) SetTimestamp(t time.Time) *EmbedBuilder {
	b.embed.Timestamp = t.Format(time.RFC3339)
	return b
}

func (b *EmbedBuilder) Build() *discordgo.MessageEmbed {
	return b.embed
}
