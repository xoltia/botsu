package mediadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/blugelabs/bluge"
	"github.com/xoltia/botsu/pkg/otame"
)

const (
	AnimeSearchFieldPrimaryTitle          = "primary_title"
	AnimeSearchFieldRomajiOfficialTitle   = "romaji_official_title"
	AnimeSearchFieldJapaneseOfficialTitle = "japanese_official_title"
	AnimeSearchFieldEnglishOfficialTitle  = "english_official_title"
)

var AnimeSearchFields = []string{
	AnimeSearchFieldPrimaryTitle,
	AnimeSearchFieldRomajiOfficialTitle,
	AnimeSearchFieldJapaneseOfficialTitle,
	AnimeSearchFieldEnglishOfficialTitle,
}

type Anime struct {
	ID                    string   `json:"id"`
	PrimaryTitle          string   `json:"primaryTitle"`
	RomajiOfficialTitle   string   `json:"romajiOfficialTitle"`
	JapaneseOfficialTitle string   `json:"japaneseOfficialTitle"`
	EnglishOfficialTitle  string   `json:"englishOfficialTitle"`
	Sources               []string `json:"sources"`
	Picture               string   `json:"picture"`
	Thumbnail             string   `json:"thumbnail"`
	Tags                  []string `json:"tags"`
}

func (a Anime) Marshal() (*bluge.Document, error) {
	doc := bluge.NewDocument(a.ID)

	doc.AddField(bluge.NewTextField(AnimeSearchFieldPrimaryTitle, a.PrimaryTitle).StoreValue())
	doc.AddField(bluge.NewTextField(AnimeSearchFieldRomajiOfficialTitle, a.RomajiOfficialTitle).StoreValue())
	doc.AddField(bluge.NewTextField(AnimeSearchFieldJapaneseOfficialTitle, a.JapaneseOfficialTitle).StoreValue())
	doc.AddField(bluge.NewTextField(AnimeSearchFieldEnglishOfficialTitle, a.EnglishOfficialTitle).StoreValue())
	doc.AddField(bluge.NewStoredOnlyField("picture", []byte(a.Picture)))
	doc.AddField(bluge.NewStoredOnlyField("thumbnail", []byte(a.Thumbnail)))

	if sourcesBytes, err := json.Marshal(a.Sources); err == nil {
		doc.AddField(bluge.NewStoredOnlyField("sources", sourcesBytes))
	} else {
		return nil, fmt.Errorf("unable to marshal sources: %w", err)
	}

	if tagsBytes, err := json.Marshal(a.Tags); err == nil {
		doc.AddField(bluge.NewStoredOnlyField("tags", tagsBytes))
	} else {
		return nil, fmt.Errorf("unable to marshal tags: %w", err)
	}

	return doc, nil
}

func (a *Anime) Unmarshal(fields map[string]string) error {
	a.ID = fields["_id"]
	a.PrimaryTitle = fields[AnimeSearchFieldPrimaryTitle]
	a.RomajiOfficialTitle = fields[AnimeSearchFieldRomajiOfficialTitle]
	a.JapaneseOfficialTitle = fields[AnimeSearchFieldJapaneseOfficialTitle]
	a.EnglishOfficialTitle = fields[AnimeSearchFieldEnglishOfficialTitle]
	a.Picture = fields["picture"]
	a.Thumbnail = fields["thumbnail"]

	if err := json.Unmarshal([]byte(fields["sources"]), &a.Sources); err != nil {
		return fmt.Errorf("unable to unmarshal sources: %w: %s", err, fields["sources"])
	}

	if err := json.Unmarshal([]byte(fields["tags"]), &a.Tags); err != nil {
		return fmt.Errorf("unable to unmarshal tags: %w: %s", err, fields["tags"])
	}

	return nil
}

func (a *Anime) SearchFields() []string {
	return AnimeSearchFields
}

func DownloadAnime(ctx context.Context) (anime []Anime, err error) {
	aodbData, err := otame.DownloadAODB(ctx)

	if err != nil {
		err = fmt.Errorf("unable to download anime data offline datavase: %w", err)
		return
	}

	defer aodbData.Close()

	animeIter := otame.NewAnimeOfflineDatabaseDecoder(aodbData)
	incompleteAnime := make(map[string]Anime)

	for {
		var aodbEntry otame.AnimeOfflineDatabaseEntry

		if aodbEntry, err = animeIter.Next(); err != nil {
			if err == otame.ErrFinished {
				err = nil
				break
			}

			err = fmt.Errorf("unable to iterate over AODB: %w", err)
			return
		}

		var anime Anime

		for _, src := range aodbEntry.Sources {
			srcURL, err := url.Parse(src)

			if err != nil {
				err = fmt.Errorf("unable to parse source URL: %w", err)
				return nil, err
			}

			if srcURL.Host == "anidb.net" {
				anime.ID = strings.TrimPrefix(srcURL.Path, "/anime/")
				break
			}
		}

		if anime.ID == "" {
			continue
		}

		anime.Sources = aodbEntry.Sources
		anime.Picture = aodbEntry.Picture
		anime.Thumbnail = aodbEntry.Thumbnail
		anime.Tags = aodbEntry.Tags

		incompleteAnime[anime.ID] = anime
	}

	anidbData, err := otame.DownloadAniDB(ctx)

	if err != nil {
		err = fmt.Errorf("unable to download AniDB: %w", err)
		return
	}

	anidbIter := otame.NewAniDBEntryDecoder(anidbData)

	defer anidbData.Close()

	for {
		var anidbEntry otame.AniDBEntry

		if anidbEntry, err = anidbIter.Next(); err != nil {
			if err == otame.ErrFinished {
				err = nil
				break
			}

			err = fmt.Errorf("unable to iterate over AniDB: %w", err)
			return
		}

		if anime, ok := incompleteAnime[anidbEntry.AID]; ok {
			switch anidbEntry.Type {
			case otame.AniDBEntryTypePrimary:
				anime.PrimaryTitle = anidbEntry.Title
			case otame.AniDBEntryTypeOfficial:
				switch anidbEntry.Language {
				case "ja":
					anime.JapaneseOfficialTitle = anidbEntry.Title
				case "en":
					anime.EnglishOfficialTitle = anidbEntry.Title
				case "x-jat":
					anime.RomajiOfficialTitle = anidbEntry.Title
				}
			}

			incompleteAnime[anidbEntry.AID] = anime
		}
	}

	anime = make([]Anime, 0, len(incompleteAnime))

	for _, a := range incompleteAnime {
		anime = append(anime, a)
	}

	return
}
