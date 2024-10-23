package orderedmap_test

import (
	"testing"

	"github.com/xoltia/botsu/pkg/orderedmap"
)

func TestOrderedMap(t *testing.T) {
	t.Run("Test ordered map", func(t *testing.T) {
		m := orderedmap.New[int]()

		m.Set("foo", 1)
		m.Set("bar", 2)
		m.Set("baz", 3)

		keys := m.Keys()

		if keys[0] != "foo" || keys[1] != "bar" || keys[2] != "baz" {
			t.Error("Keys are not in order")
		}

		values := m.Values()

		if values[0] != 1 || values[1] != 2 || values[2] != 3 {
			t.Error("Values are not in order")
		}

		m.Delete("bar")

		keys = m.Keys()

		if keys[0] != "foo" || keys[1] != "baz" {
			t.Error("Keys are not in order")
		}

		values = m.Values()

		if values[0] != 1 || values[1] != 3 {
			t.Error("Values are not in order")
		}
	})
}
