package otame

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

const (
	AniDBEntryTypePrimary  = "primary"
	AniDBEntryTypeSynonym  = "synonym"
	AniDBEntryTypeShort    = "short"
	AniDBEntryTypeOfficial = "official"
)

var entryTypeMap = map[string]string{
	"1": AniDBEntryTypePrimary,
	"2": AniDBEntryTypeSynonym,
	"3": AniDBEntryTypeShort,
	"4": AniDBEntryTypeOfficial,
}

type AniDBEntry struct {
	AID      string
	Type     string
	Language string
	Title    string
}

type AniDBEntryDecoder struct {
	scanner *bufio.Scanner
}

func NewAniDBEntryDecoder(r io.Reader) *AniDBEntryDecoder {
	return &AniDBEntryDecoder{
		scanner: bufio.NewScanner(r),
	}
}

// Returns slice of length 4 (aid, titleType, language, title).
// Skips empty lines and lines starting with '#'
func (a *AniDBEntryDecoder) readLine() (line []string, err error) {
	if !a.scanner.Scan() {
		err = io.EOF
		return
	}

	lineText := a.scanner.Text()
	if lineText == "" || strings.HasPrefix(strings.TrimLeft(lineText, " "), "#") {
		line, err = a.readLine()
		return
	}

	line = strings.SplitN(lineText, "|", 4)

	if len(line) != 4 {
		err = fmt.Errorf("invalid line: %s", line)
		return
	}

	err = a.scanner.Err()

	return
}

func (a *AniDBEntryDecoder) Next() (entry AniDBEntry, err error) {
	var line []string

	line, err = a.readLine()
	if err != nil {
		return
	}

	entry.AID = line[0]
	entry.Type = entryTypeMap[line[1]]
	entry.Language = line[2]
	entry.Title = line[3]

	return
}

func (a *AniDBEntryDecoder) DecodeAll() (entries []AniDBEntry, err error) {
	for {
		var entry AniDBEntry
		entry, err = a.Next()

		if err == io.EOF {
			err = nil
			break
		}

		if err != nil {
			return
		}

		entries = append(entries, entry)
	}

	return
}
