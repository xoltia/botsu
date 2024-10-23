package discordutil_test

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/xoltia/botsu/pkg/discordutil"
	"github.com/xoltia/botsu/pkg/ref"
)

func TestUnmarshalOptions(t *testing.T) {
	type testType struct {
		Field1 string                                             `discordopt:"field1"`
		Field2 int                                                `discordopt:"field2"`
		Field3 uint32                                             `discordopt:"field3"`
		Field4 bool                                               `discordopt:"field4"`
		Field5 float64                                            `discordopt:"field5"`
		Field6 *bool                                              `discordopt:"field6"`
		Field7 *string                                            `discordopt:"field7"`
		Field8 **string                                           `discordopt:"field8"`
		Field9 *discordgo.ApplicationCommandInteractionDataOption `discordopt:"field9"`
	}

	options := []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Value: "value1",
			Name:  "field1",
			Type:  discordgo.ApplicationCommandOptionString,
		},
		{
			Value: float64(2.0),
			Name:  "field2",
			Type:  discordgo.ApplicationCommandOptionInteger,
		},
		{
			Value: float64(3.0),
			Name:  "field3",
			Type:  discordgo.ApplicationCommandOptionInteger,
		},
		{
			Value: true,
			Name:  "field4",
			Type:  discordgo.ApplicationCommandOptionBoolean,
		},
		{
			Value: float64(5.2),
			Name:  "field5",
			Type:  discordgo.ApplicationCommandOptionNumber,
		},
		{
			Value: "value2",
			Name:  "field7",
			Type:  discordgo.ApplicationCommandOptionString,
		},
		{
			Value: "value3",
			Name:  "field8",
			Type:  discordgo.ApplicationCommandOptionString,
		},
		{
			Name:  "field9",
			Type:  discordgo.ApplicationCommandOptionString,
			Value: "value4",
		},
	}

	expected := testType{
		Field1: "value1",
		Field2: 2,
		Field3: 3,
		Field4: true,
		Field5: 5.2,
		Field6: nil,
		Field7: ref.New("value2"),
		Field8: ref.New(ref.New("value3")),
		Field9: &discordgo.ApplicationCommandInteractionDataOption{
			Value: "value4",
			Name:  "field9",
			Type:  discordgo.ApplicationCommandOptionString,
		},
	}

	var actual testType
	err := discordutil.UnmarshalOptions(options, &actual)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	assert.Equal(t, expected, actual)
}
