package activities

import (
	"compress/gzip"
	"encoding/json"
	"io"

	internal "github.com/xoltia/botsu/internal/activities"
)

type Activity = internal.Activity

func ReadJSONL(r io.Reader) (as []*Activity, err error) {
	decoder := json.NewDecoder(r)

	for decoder.More() {
		a := &Activity{}
		if err = decoder.Decode(&a); err != nil {
			return
		}
		as = append(as, a)
	}

	return
}

func ReadCompressedJSONL(r io.Reader) (activitySlice []*Activity, err error) {
	if r, err = gzip.NewReader(r); err != nil {
		return
	}
	defer r.(*gzip.Reader).Close()

	activitySlice, err = ReadJSONL(r)
	return
}
