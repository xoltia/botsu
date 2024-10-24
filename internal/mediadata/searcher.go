package mediadata

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"sync"

	"github.com/blugelabs/bluge"
)

type Store interface {
	Marshal() (*bluge.Document, error)
}

type Read[T any] interface {
	*T
	Unmarshal(map[string]string) error
	SearchFields() []string
}

// A batchedReadWriter is a struct that batches writes to a bluge writer.
// Only safe for concurrent reads, not writes. Writes/close must be
// serialized.
type batchedReadWriter[T Store, PT Read[T]] struct {
	config bluge.Config
	r      *bluge.Reader
	mu     sync.RWMutex
}

func newBatchedReadWriter[T Store, PT Read[T]](config bluge.Config) *batchedReadWriter[T, PT] {
	return &batchedReadWriter[T, PT]{config: config}
}

// Reads a record from the searcher.
func (rw *batchedReadWriter[T, PT]) read(ctx context.Context, id string) (record PT, err error) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	req := bluge.NewTermQuery(id).SetField("_id")
	dmi, err := rw.r.Search(ctx, bluge.NewTopNSearch(1, req))
	if err != nil {
		return
	}

	next, err := dmi.Next()
	if err != nil {
		return
	}

	if next == nil {
		err = fmt.Errorf("record store: record with id %s not found", id)
		return
	}

	record = PT(new(T))
	fields := make(map[string]string)

	err = next.VisitStoredFields(func(field string, value []byte) bool {
		fields[field] = string(value)
		return true
	})
	if err != nil {
		return
	}
	err = record.Unmarshal(fields)
	return
}

// Searches the searcher for a match.
func (rw *batchedReadWriter[T, PT]) search(ctx context.Context, matchQuery string, limit int) (matches []Match[T], err error) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	matchList := make(MatchList[T])

	for _, field := range PT(new(T)).SearchFields() {
		query := bluge.NewMatchQuery(matchQuery).SetField(field)
		searchRequest := bluge.NewTopNSearch(limit, query)
		dmi, err := rw.r.Search(ctx, searchRequest)
		if err != nil {
			return nil, err
		}

		next, err := dmi.Next()

		for next != nil && err == nil {
			record := PT(new(T))
			fields := make(map[string]string)

			err = next.VisitStoredFields(func(field string, value []byte) bool {
				fields[field] = string(value)
				return true
			})
			if err != nil {
				break
			}

			if err = record.Unmarshal(fields); err != nil {
				break
			}

			match := Match[T]{string(fields["_id"]), record, next.Score, field}
			matchList.Insert(match)
			next, err = dmi.Next()
		}

		if err != nil {
			return nil, err
		}
	}

	matches = matchList.Top(limit)

	return
}

func (rw *batchedReadWriter[T, PT]) overwriteData(data []T) (err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	w, err := bluge.OpenWriter(rw.config)
	if err != nil {
		return
	}

	batch := bluge.NewBatch()

	var doc *bluge.Document
	for _, record := range data {
		doc, err = record.Marshal()
		if err != nil {
			return
		}

		batch.Update(doc.ID(), doc)
	}

	if err = w.Batch(batch); err != nil {
		return
	}

	if err = w.Close(); err != nil {
		log.Println("Unable to close writer", slog.String("err", err.Error()))
	}

	if rw.r != nil {
		if err = rw.r.Close(); err != nil {
			log.Println("Unable to close reader", slog.String("err", err.Error()))
		}

		rw.r, err = bluge.OpenReader(rw.config)
	}

	return
}

func (rw *batchedReadWriter[T, PT]) close() (err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	err = rw.r.Close()
	rw.r = nil
	return
}

// Opens a new reader and writer.
func (rw *batchedReadWriter[T, PT]) open() (err error) {
	rw.r, err = bluge.OpenReader(rw.config)
	return
}

type MediaSearcher struct {
	Logger  *slog.Logger
	animeRW *batchedReadWriter[Anime, *Anime]
	vnRW    *batchedReadWriter[VisualNovel, *VisualNovel]
}

func NewMediaSearcher(path string) (s *MediaSearcher) {
	animeConfig := bluge.DefaultConfig(path + "/anime")
	animeRW := newBatchedReadWriter[Anime, *Anime](animeConfig)
	vnConfig := bluge.DefaultConfig(path + "/vn")
	vnRW := newBatchedReadWriter[VisualNovel, *VisualNovel](vnConfig)

	s = &MediaSearcher{
		animeRW: animeRW,
		vnRW:    vnRW,
	}

	s.Logger = slog.Default()

	return
}

func (s *MediaSearcher) UpdateData(ctx context.Context) (err error) {
	s.Logger.Info("Updating searcher data")
	errs := make(chan error, 2)

	go func() {
		s.Logger.Info("Downloaded anime data")
		animeData, err := DownloadAnime(ctx)
		if err != nil {
			errs <- fmt.Errorf("unable to download anime data: %w", err)
			return
		}

		if err = s.animeRW.overwriteData(animeData); err != nil {
			errs <- fmt.Errorf("unable to overwrite anime data: %w", err)
			return
		}

		errs <- nil
	}()

	go func() {
		s.Logger.Info("Downloaded visual novel data")
		vnData, err := DownloadVisualNovels(ctx)
		if err != nil {
			errs <- fmt.Errorf("unable to download visual novel data: %w", err)
			return
		}

		if err = s.vnRW.overwriteData(vnData); err != nil {
			errs <- fmt.Errorf("unable to overwrite visual novel data: %w", err)
			return
		}

		errs <- nil
	}()

	for i := 0; i < 2; i++ {
		if err = <-errs; err != nil {
			return
		}
	}

	s.Logger.Info("Finished updating searcher data")

	return
}

func (s *MediaSearcher) Open() (err error) {
	if err = s.animeRW.open(); err != nil {
		return
	}

	err = s.vnRW.open()

	return
}

func (s *MediaSearcher) Close() (err error) {
	if err = s.animeRW.close(); err != nil {
		return
	}

	if err = s.vnRW.close(); err != nil {
		return
	}

	return
}

func (s *MediaSearcher) ReadAnime(ctx context.Context, id string) (*Anime, error) {
	return s.animeRW.read(ctx, id)
}

func (s *MediaSearcher) ReadVisualNovel(ctx context.Context, id string) (*VisualNovel, error) {
	return s.vnRW.read(ctx, id)
}

func (s *MediaSearcher) SearchAnime(ctx context.Context, matchQuery string, limit int) ([]Match[Anime], error) {
	return s.animeRW.search(ctx, matchQuery, limit)
}

func (s *MediaSearcher) SearchVisualNovel(ctx context.Context, matchQuery string, limit int) ([]Match[VisualNovel], error) {
	return s.vnRW.search(ctx, matchQuery, limit)
}
