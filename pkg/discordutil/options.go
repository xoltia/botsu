package discordutil

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var ErrOptionNotFound = errors.New("option not found")

func GetFocusedOption(options []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.ApplicationCommandInteractionDataOption {
	for _, option := range options {
		if option.Focused {
			return option
		}
	}

	return nil
}

func GetOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) *discordgo.ApplicationCommandInteractionDataOption {
	for _, option := range options {
		if option.Name == key {
			return option
		}
	}

	return nil
}

func GetRequiredOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) (*discordgo.ApplicationCommandInteractionDataOption, error) {
	if option := GetOption(options, key); option != nil {
		return option, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrOptionNotFound, key)
}

func GetStringOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) *string {
	option := GetOption(options, key)
	if option == nil {
		return nil
	}

	str := option.StringValue()
	return &str
}

func GetStringOptionOrDefault(options []*discordgo.ApplicationCommandInteractionDataOption, key, defaultValue string) string {
	option := GetOption(options, key)
	if option == nil {
		return defaultValue
	}

	return option.StringValue()
}

func GetRequiredStringOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) (string, error) {
	option, err := GetRequiredOption(options, key)
	if err != nil {
		return "", err
	}

	return option.StringValue(), nil
}

func GetIntOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) *int64 {
	option := GetOption(options, key)
	if option == nil {
		return nil
	}

	i := option.IntValue()
	return &i
}

func GetIntOptionOrDefault(options []*discordgo.ApplicationCommandInteractionDataOption, key string, defaultValue int64) int64 {
	option := GetOption(options, key)
	if option == nil {
		return defaultValue
	}

	return option.IntValue()
}

func GetRequiredIntOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) (int64, error) {
	option, err := GetRequiredOption(options, key)
	if err != nil {
		return 0, err
	}

	return option.IntValue(), nil
}

func GetBoolOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) *bool {
	option := GetOption(options, key)
	if option == nil {
		return nil
	}

	b := option.BoolValue()
	return &b
}

func GetBoolOptionOrDefault(options []*discordgo.ApplicationCommandInteractionDataOption, key string, defaultValue bool) bool {
	option := GetOption(options, key)
	if option == nil {
		return defaultValue
	}

	return option.BoolValue()
}

func GetRequiredBoolOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) (bool, error) {
	option, err := GetRequiredOption(options, key)
	if err != nil {
		return false, err
	}

	return option.BoolValue(), nil
}

func GetUintOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) *uint64 {
	option := GetOption(options, key)
	if option == nil {
		return nil
	}

	u := option.UintValue()
	return &u
}

func GetUintOptionOrDefault(options []*discordgo.ApplicationCommandInteractionDataOption, key string, defaultValue uint64) uint64 {
	option := GetOption(options, key)
	if option == nil {
		return defaultValue
	}

	return option.UintValue()
}

func GetRequiredUintOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) (uint64, error) {
	option, err := GetRequiredOption(options, key)
	if err != nil {
		return 0, err
	}

	return option.UintValue(), nil
}

func GetFloatOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) *float64 {
	option := GetOption(options, key)
	if option == nil {
		return nil
	}

	f := option.FloatValue()
	return &f
}

func GetFloatOptionOrDefault(options []*discordgo.ApplicationCommandInteractionDataOption, key string, defaultValue float64) float64 {
	option := GetOption(options, key)
	if option == nil {
		return defaultValue
	}

	return option.FloatValue()
}

func GetRequiredFloatOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string) (float64, error) {
	option, err := GetRequiredOption(options, key)
	if err != nil {
		return 0, err
	}

	return option.FloatValue(), nil
}

func GetUserOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string, s *discordgo.Session) *discordgo.User {
	option := GetOption(options, key)
	if option == nil {
		return nil
	}

	return option.UserValue(s)
}

func GetUserOptionOrDefault(options []*discordgo.ApplicationCommandInteractionDataOption, key string, defaultValue *discordgo.User, s *discordgo.Session) *discordgo.User {
	option := GetOption(options, key)
	if option == nil {
		return defaultValue
	}

	return option.UserValue(s)
}

func GetRequiredUserOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string, s *discordgo.Session) (*discordgo.User, error) {
	option, err := GetRequiredOption(options, key)
	if err != nil {
		return nil, err
	}

	return option.UserValue(s), nil
}

func GetChannelOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string, s *discordgo.Session) *discordgo.Channel {
	option := GetOption(options, key)
	if option == nil {
		return nil
	}

	return option.ChannelValue(s)
}

func GetChannelOptionOrDefault(options []*discordgo.ApplicationCommandInteractionDataOption, key string, defaultValue *discordgo.Channel, s *discordgo.Session) *discordgo.Channel {
	option := GetOption(options, key)
	if option == nil {
		return defaultValue
	}

	return option.ChannelValue(s)
}

func GetRequiredChannelOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string, s *discordgo.Session) (*discordgo.Channel, error) {
	option, err := GetRequiredOption(options, key)
	if err != nil {
		return nil, err
	}

	return option.ChannelValue(s), nil
}

func GetRoleOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string, s *discordgo.Session, gid string) *discordgo.Role {
	option := GetOption(options, key)
	if option == nil {
		return nil
	}

	return option.RoleValue(s, gid)
}

func GetRoleOptionOrDefault(options []*discordgo.ApplicationCommandInteractionDataOption, key string, defaultValue *discordgo.Role, s *discordgo.Session, gid string) *discordgo.Role {
	option := GetOption(options, key)
	if option == nil {
		return defaultValue
	}

	return option.RoleValue(s, gid)
}

func GetRequiredRoleOption(options []*discordgo.ApplicationCommandInteractionDataOption, key string, s *discordgo.Session, gid string) (*discordgo.Role, error) {
	option, err := GetRequiredOption(options, key)
	if err != nil {
		return nil, err
	}

	return option.RoleValue(s, gid), nil
}

func UnmarshalOptions(options []*discordgo.ApplicationCommandInteractionDataOption, v any) error {
	rv := reflect.ValueOf(v)
	rt := reflect.TypeOf(v)

	if kind := rt.Kind(); kind != reflect.Pointer {
		return fmt.Errorf("UnmarshalOptions: expected pointer, got: %s", kind)
	}

	elemValue := rv.Elem()
	elemType := rt.Elem()

	if elemKind := elemType.Kind(); elemKind != reflect.Struct {
		return fmt.Errorf("UnmarshalOptions: expected struct pointer, got pointer of: %s", elemKind)
	}

	numFields := elemType.NumField()
	for i := 0; i < numFields; i++ {
		field := elemValue.Field(i)
		fieldType := elemType.Field(i)
		if !fieldType.IsExported() {
			continue
		}
		tag := fieldType.Tag.Get("discordopt")
		fields := strings.Split(tag, ",")
		name := fields[0]
		if name == "" || name == "-" {
			continue
		}
		required := false
		for _, f := range fields[1:] {
			if f == "required" {
				required = true
			}
		}
		option := GetOption(options, name)
		if option == nil {
			if required {
				return fmt.Errorf("UnmarshalOptions: required option not found: %s", name)
			}
			continue
		}

		if err := setField(field, fieldType.Type, option); err != nil {
			err = fmt.Errorf("UnmarshalOptions: %s: %w", name, err)
			return err
		}
	}

	return nil
}

func setField(value reflect.Value, reflectType reflect.Type, option *discordgo.ApplicationCommandInteractionDataOption) error {
	// TODO: Subcommands, command groups, raw options, user/role/channel values?
	switch k := value.Kind(); k {
	case reflect.Struct:
		switch v := value.Interface(); v.(type) {
		case discordgo.ApplicationCommandInteractionDataOption:
			value.Set(reflect.ValueOf(*option))
		default:
			return errors.New("struct field only supports ApplicationCommandInteractionDataOption")
		}
	case reflect.Pointer:
		value.Set(reflect.New(reflectType.Elem()))
		return setField(value.Elem(), reflectType.Elem(), option)
	case reflect.String:
		if option.Type != discordgo.ApplicationCommandOptionString {
			return errors.New("string field expected string option")
		}
		value.SetString(option.StringValue())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if option.Type != discordgo.ApplicationCommandOptionInteger {
			return errors.New("int field expected int option")
		}
		value.SetInt(option.IntValue())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if option.Type != discordgo.ApplicationCommandOptionInteger {
			return errors.New("uint field expected integer option")
		}
		value.SetUint(option.UintValue())
	case reflect.Float32, reflect.Float64:
		if option.Type != discordgo.ApplicationCommandOptionNumber {
			return errors.New("float field expected number option")
		}
		value.SetFloat(option.FloatValue())
	case reflect.Bool:
		if option.Type != discordgo.ApplicationCommandOptionBoolean {
			return errors.New("bool field expected boolean option")
		}
		value.SetBool(option.BoolValue())
	default:
		return fmt.Errorf("unexpected field kind: %s", k)
	}
	return nil
}
