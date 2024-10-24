package otame

import (
	"encoding/json"
	"fmt"
	"io"
)

type AnimeOfflineDatabaseSeason struct {
	Season string `json:"season"`
	Year   *int   `json:"year"`
}

type AnimeOfflineDatabaseEntry struct {
	Sources   []string                   `json:"sources"`
	Title     string                     `json:"title"`
	Type      string                     `json:"type"`
	Episodes  int                        `json:"episodes"`
	Status    string                     `json:"status"`
	Season    AnimeOfflineDatabaseSeason `json:"animeSeason"`
	Picture   string                     `json:"picture"`
	Thumbnail string                     `json:"thumbnail"`
	Synonyms  []string                   `json:"synonyms"`
	Relations []string                   `json:"relations"`
	Tags      []string                   `json:"tags"`
}

type AnimeOfflineDatabaseDecoder struct {
	decoder  *json.Decoder
	caughtUp bool
}

func NewAnimeOfflineDatabaseDecoder(r io.Reader) *AnimeOfflineDatabaseDecoder {
	return &AnimeOfflineDatabaseDecoder{
		decoder:  json.NewDecoder(r),
		caughtUp: false,
	}
}

func (a *AnimeOfflineDatabaseDecoder) DecodeAll() (entries []AnimeOfflineDatabaseEntry, err error) {
	if !a.caughtUp {
		err = a.locateDataArray()
		if err != nil {
			return
		}
	}

	for a.decoder.More() {
		var entry AnimeOfflineDatabaseEntry
		err = a.decoder.Decode(&entry)
		if err != nil {
			return
		}

		entries = append(entries, entry)
	}

	return
}

func (a *AnimeOfflineDatabaseDecoder) Next() (entry AnimeOfflineDatabaseEntry, err error) {
	if !a.caughtUp {
		err = a.locateDataArray()
		if err != nil {
			return
		}
	}

	if !a.decoder.More() {
		err = io.EOF
		return
	}

	err = a.decoder.Decode(&entry)
	return
}

func (a *AnimeOfflineDatabaseDecoder) locateDataArray() error {
	for {
		t, err := a.decoder.Token()
		if err != nil {
			return err
		}

		if t == "data" {
			break
		}
	}

	// expect a '['
	t, err := a.decoder.Token()
	if err != nil {
		return err
	}

	if t != json.Delim('[') {
		return fmt.Errorf("expected '[' but got '%s'", t)
	}

	a.caughtUp = true
	return nil
}
