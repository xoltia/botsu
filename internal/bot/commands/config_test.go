package commands_test

import (
	"testing"
	"time"

	"github.com/xoltia/botsu/internal/bot/commands"
)

func TestTimeZone(t *testing.T) {
	valid := true

	for _, tz := range commands.ValidTimezones {
		if _, err := time.LoadLocation(tz); err != nil {
			t.Logf("Invalid time zone: %s", tz)
			valid = false
		}
	}

	if !valid {
		t.Fail()
	}
}
