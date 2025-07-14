package otame

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"

	"github.com/klauspost/compress/zstd"
)

const aodbDownloadURL = "https://github.com/manami-project/anime-offline-database/releases/download/latest/anime-offline-database-minified.json"
const anidbDownloadURL = "https://anidb.net/api/anime-titles.dat.gz"
const vndbDownloadURL = "https://dl.vndb.org/dump/vndb-db-latest.tar.zst"

// Used for sources that attempt to block bots.
const defaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:89.0) Gecko/20100101 Firefox/89.0"

// Inherits an io.ReadCloser (such as gzip.Reader), and takes
// an additional io.Closer to close when Close() is called.
// Useful for closing the underlying http.Response.Body when
// the gzip.Reader is closed.
type dualCloser struct {
	io.ReadCloser
	inner io.Closer
}

func (g *dualCloser) Close() error {
	return errors.Join(
		g.ReadCloser.Close(),
		g.inner.Close(),
	)
}

func (g *dualCloser) Read(p []byte) (n int, err error) {
	return g.ReadCloser.Read(p)
}

type fsCloser struct {
	fs.FS
	Close func() error
}

// Returns a ReadCloser for the anime-offline-database-minified.json
// file. The caller is responsible for closing the ReadCloser.
func DownloadAODB(ctx context.Context) (r io.ReadCloser, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, aodbDownloadURL, nil)
	if err != nil {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	r = resp.Body
	return
}

// Returns a ReadCloser for the anidb-titles.dat file
// The caller is responsible for closing the ReadCloser.
func DownloadAniDB(ctx context.Context) (r io.ReadCloser, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, anidbDownloadURL, nil)

	var userAgent = defaultUserAgent

	if ctx.Value("req-user-agent") != nil {
		ctxUserAgent, ok := ctx.Value("user-agent").(string)

		if ok {
			userAgent = ctxUserAgent
		}
	}

	req.Header.Set("User-Agent", userAgent)
	if err != nil {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	r, err = gzip.NewReader(resp.Body)
	if err != nil {
		resp.Body.Close()
		return
	}

	r = &dualCloser{
		ReadCloser: r,
		inner:      resp.Body,
	}
	return
}

// Returns an fs.FS that also implements io.Closer.
// The caller is responsible for closing the fsCloser.
// Same as: DownloadVNDBUsingTempDir(ctx, "")
func DownloadVNDB(ctx context.Context) (*fsCloser, error) {
	return DownloadVNDBUsingTempDir(ctx, "")
}

// Returns an fs.FS that also implements io.Closer.
// The caller is responsible for closing the fsCloser, which
// will remove the temporary directory.
func DownloadVNDBUsingTempDir(ctx context.Context, temp string) (fsc *fsCloser, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, vndbDownloadURL, nil)
	if err != nil {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	r, err := zstd.NewReader(resp.Body)
	if err != nil {
		return
	}
	defer r.Close()

	tempDir, err := os.MkdirTemp(temp, "vndb")
	if err != nil {
		return
	}

	tarReader := tar.NewReader(r)
	for {
		if err = ctx.Err(); err != nil {
			break
		}

		var header *tar.Header

		header, err = tarReader.Next()
		if err != nil {
			if err == io.EOF {
				err = nil
			}

			break
		}

		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(path.Join(tempDir, header.Name), 0755)
		case tar.TypeReg:
			var file *os.File

			file, err = os.Create(path.Join(tempDir, header.Name))
			if err != nil {
				break
			}

			_, err = io.Copy(file, tarReader)
			if err != nil {
				file.Close()
				break
			}

			file.Close()
		default:
			err = fmt.Errorf("unknown tar header type: %d", header.Typeflag)
		}
	}

	if err != nil {
		os.RemoveAll(tempDir)
		return
	}

	fsc = &fsCloser{
		FS:    os.DirFS(tempDir),
		Close: func() error { return os.RemoveAll(tempDir) },
	}
	return
}
