package mediadata

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/blugelabs/bluge"
	"github.com/xoltia/botsu/pkg/otame"
)

const (
	VNSearchFieldJapaneseTitle = "japanese_title"
	VNSearchFieldEnglishTitle  = "english_title"
	VNSearchFieldRomajiTitle   = "romaji_title"
)

var VNSearchFields = []string{
	VNSearchFieldJapaneseTitle,
	VNSearchFieldEnglishTitle,
	VNSearchFieldRomajiTitle,
}

type VisualNovel struct {
	ID            string
	JapaneseTitle string
	EnglishTitle  string
	RomajiTitle   string
	ImageID       string
	ImageNSFW     bool
}

func (vn VisualNovel) ImageURL() string {
	if vn.ImageID == "" {
		return ""
	}

	return otame.VNDBCDNURLFromImageID(vn.ImageID)
}

func (vn VisualNovel) Marshal() (*bluge.Document, error) {
	doc := bluge.NewDocument(vn.ID)

	doc.AddField(bluge.NewTextField(VNSearchFieldJapaneseTitle, vn.JapaneseTitle).StoreValue())
	doc.AddField(bluge.NewTextField(VNSearchFieldEnglishTitle, vn.EnglishTitle).StoreValue())
	doc.AddField(bluge.NewTextField(VNSearchFieldRomajiTitle, vn.RomajiTitle).StoreValue())
	doc.AddField(bluge.NewStoredOnlyField("image", []byte(vn.ImageID)))

	if vn.ImageNSFW {
		doc.AddField(bluge.NewStoredOnlyField("image_nsfw", []byte("true")))
	} else {
		doc.AddField(bluge.NewStoredOnlyField("image_nsfw", []byte("false")))
	}

	return doc, nil
}

func (vn *VisualNovel) Unmarshal(fields map[string]string) error {
	vn.ID = fields["_id"]
	vn.JapaneseTitle = fields[VNSearchFieldJapaneseTitle]
	vn.EnglishTitle = fields[VNSearchFieldEnglishTitle]
	vn.RomajiTitle = fields[VNSearchFieldRomajiTitle]
	vn.ImageID = fields["image"]

	if fields["image_nsfw"] == "true" {
		vn.ImageNSFW = true
	} else {
		vn.ImageNSFW = false
	}

	return nil
}

func (vn *VisualNovel) SearchFields() []string {
	return VNSearchFields
}

func DownloadVisualNovels(ctx context.Context) (vns []VisualNovel, err error) {
	vndbDataFS, err := otame.DownloadVNDB(ctx)
	if err != nil {
		err = fmt.Errorf("unable to download VNDB data: %w", err)
		return
	}

	defer func() {
		if err = vndbDataFS.Close(); err != nil {
			slog.Error("Unable to close VNDB data", slog.String("err", err.Error()))

			if err == nil {
				err = fmt.Errorf("unable to close VNDB data: %w", err)
			}
		}
	}()

	vnData, err := vndbDataFS.Open("db/vn")
	if err != nil {
		err = fmt.Errorf("unable to open VNDB data: %w", err)
		return
	}

	defer vnData.Close()

	vndbIter := otame.NewVNDBVisualNovelDecoder(vnData)
	vnMap := make(map[string]VisualNovel)
	imageMap := make(map[string]string)
	for {
		var vn otame.VNDBVisualNovel
		vn, err = vndbIter.Next()
		if err != nil {
			if err == otame.ErrFinished {
				err = nil
				break
			}

			err = fmt.Errorf("unable to decode VNDB data: %w", err)
			return
		}

		if vn.ImageID != nil {
			imageMap[*vn.ImageID] = vn.ID
			vnMap[vn.ID] = VisualNovel{
				ID:      vn.ID,
				ImageID: *vn.ImageID,
			}
		} else {
			vnMap[vn.ID] = VisualNovel{
				ID: vn.ID,
			}
		}
	}

	vnTitleData, err := vndbDataFS.Open("db/vn_titles")
	if err != nil {
		err = fmt.Errorf("unable to open VNDB title data: %w", err)
		return
	}
	defer vnTitleData.Close()

	vnTitleIter := otame.NewVNDBTitleDecoder(vnTitleData)

	for {
		var vnTitle otame.VNDBTitle
		vnTitle, err = vnTitleIter.Next()
		if err != nil {
			if err == otame.ErrFinished {
				err = nil
				break
			}

			err = fmt.Errorf("unable to decode VNDB title data: %w", err)
			return
		}

		if !vnTitle.Official {
			continue
		}

		if vn, ok := vnMap[vnTitle.VNID]; ok {
			switch vnTitle.Language {
			case "ja":
				vn.JapaneseTitle = vnTitle.Title

				if vnTitle.Latin != nil {
					vn.RomajiTitle = *vnTitle.Latin
				}
			case "en":
				vn.EnglishTitle = vnTitle.Title
			}

			vnMap[vnTitle.VNID] = vn
		}
	}

	vnImageData, err := vndbDataFS.Open("db/images")
	if err != nil {
		err = fmt.Errorf("unable to open VNDB image data: %w", err)
		return
	}
	defer vnImageData.Close()

	vnImageIter := otame.NewVNDBImageDecoder(vnImageData)
	for {
		var vnImage otame.VNDBImage
		vnImage, err = vnImageIter.Next()
		if err != nil {
			if err == otame.ErrFinished {
				err = nil
				break
			}

			err = fmt.Errorf("unable to decode VNDB image data: %w", err)
			return
		}

		if vn, ok := vnMap[imageMap[vnImage.ID]]; ok {
			vn.ImageNSFW = vnImage.NSFW()
			vnMap[imageMap[vnImage.ID]] = vn
		}
	}

	vns = make([]VisualNovel, 0, len(vnMap))
	for _, vn := range vnMap {
		vns = append(vns, vn)
	}

	return vns, nil
}
