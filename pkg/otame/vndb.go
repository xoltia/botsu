package otame

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

type VNDBTitle struct {
	VNID     string
	Language string
	Official bool
	Title    string
	Latin    *string
}

type VNDBVisualNovel struct {
	ID               string
	OriginalLanguage string
	ImageID          *string
}

func VNDBCDNURLFromImageID(imgID string) string {
	imgID = imgID[2:]
	lastTwoDigits := "0"

	if len(imgID) > 1 {
		lastTwoDigits = imgID[len(imgID)-2:]
	} else {
		lastTwoDigits += imgID
	}

	return fmt.Sprintf("https://t.vndb.org/cv/%s/%s.jpg", lastTwoDigits, imgID)
}

type VNDBImage struct {
	ID          string
	Width       int
	Height      int
	SexualAvg   int
	SexualDev   int
	ViolenceAvg int
	ViolenceDev int
}

func (e VNDBImage) NSFW() bool {
	return e.SexualAvg >= 40 || e.ViolenceAvg >= 40
}

func NewVNDBImageDecoder(r io.Reader) *genericLineDecoder[VNDBImage] {
	// id	width	height	c_votecount	c_sexual_avg	c_sexual_stddev	c_violence_avg	c_violence_stddev	c_weight
	return &genericLineDecoder[VNDBImage]{
		scanner:       bufio.NewScanner(r),
		separatorChar: "\t",
		nCols:         9,
		unmarshal: func(line []string) (entry VNDBImage, err error) {
			entry.ID = line[0]

			if entry.Width, err = strconv.Atoi(line[1]); err != nil {
				err = fmt.Errorf("invalid width: %w", err)
				return
			}

			if entry.Height, err = strconv.Atoi(line[2]); err != nil {
				err = fmt.Errorf("invalid height: %w", err)
				return
			}

			if entry.SexualAvg, err = strconv.Atoi(line[4]); err != nil {
				err = fmt.Errorf("invalid sexual avg: %w", err)
				return
			}

			if entry.SexualDev, err = strconv.Atoi(line[5]); err != nil {
				err = fmt.Errorf("invalid sexual dev: %w", err)
				return
			}

			if entry.ViolenceAvg, err = strconv.Atoi(line[6]); err != nil {
				err = fmt.Errorf("invalid violence avg: %w", err)
				return
			}

			return
		},
	}
}

func NewVNDBTitleDecoder(r io.Reader) *genericLineDecoder[VNDBTitle] {
	return &genericLineDecoder[VNDBTitle]{
		scanner:       bufio.NewScanner(r),
		separatorChar: "\t",
		nCols:         5,
		unmarshal: func(line []string) (entry VNDBTitle, err error) {
			entry.VNID = line[0]
			entry.Language = line[1]
			entry.Official = line[2] == "t"
			entry.Title = line[3]
			if line[4] != "\\N" {
				entry.Latin = &line[4]
			}

			return
		},
	}
}

func NewVNDBVisualNovelDecoder(r io.Reader) *genericLineDecoder[VNDBVisualNovel] {
	return &genericLineDecoder[VNDBVisualNovel]{
		scanner:       bufio.NewScanner(r),
		separatorChar: "\t",
		// actually more than 4 columns, but we only care about the first 3
		nCols: 4,
		unmarshal: func(line []string) (entry VNDBVisualNovel, err error) {
			entry.ID = line[0]
			entry.OriginalLanguage = line[1]

			if entry.OriginalLanguage != "\\N" {
				entry.ImageID = &line[2]
			}

			return
		},
	}
}

type genericLineDecoder[T any] struct {
	line          int
	scanner       *bufio.Scanner
	commentChar   string
	separatorChar string
	nCols         int
	unmarshal     func([]string) (T, error)
}

func (d *genericLineDecoder[T]) readLine() (cols []string, err error) {
	if !d.scanner.Scan() {
		err = io.EOF
		return
	}

	lineText := d.scanner.Text()
	d.line++

	// ignore blank lines and comments
	trimmedLineText := strings.TrimLeftFunc(lineText, unicode.IsSpace)
	isComment := strings.HasPrefix(trimmedLineText, d.commentChar)

	if lineText == "" || isComment && len(d.commentChar) > 0 {
		cols, err = d.readLine()
		return
	}

	cols = strings.SplitN(lineText, d.separatorChar, d.nCols)

	if len(cols) != d.nCols {
		err = fmt.Errorf("invalid line: %s", cols)
	}

	return
}

func (d *genericLineDecoder[T]) Next() (entry T, err error) {
	var line []string

	line, err = d.readLine()
	if err != nil {
		return
	}

	entry, err = d.unmarshal(line)
	if err != nil {
		err = fmt.Errorf("line %d: %w", d.line, err)
	}

	return
}
