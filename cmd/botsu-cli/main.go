package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xoltia/botsu/internal/activities"
	"github.com/xoltia/botsu/pkg/ref"
)

var (
	nameFlag                      = flag.String("name", "", "name of the activity")
	nameShortFlag                 = flag.String("n", "", "name of the activity (shorthand)")
	readingTypeFlag               = flag.Bool("reading", false, "activity is reading")
	readingTypeShortFlag          = flag.Bool("r", false, "activity is reading (shorthand)")
	listeningTypeFlag             = flag.Bool("listening", false, "activity is listening")
	listeningTypeShortFlag        = flag.Bool("l", false, "activity is listening (shorthand)")
	durationFlag                  = flag.Float64("duration", 0, "duration of the activity in minutes")
	durationShortFlag             = flag.Float64("d", 0, "duration of the activity in minutes (shorthand)")
	timeFlag                      = flag.String("time", "", "time of the activity (RFC3339 format)")
	timeShortFlag                 = flag.String("t", "", "time of the activity (RFC3339 format) (shorthand)")
	videoMediaTypeFlag            = flag.Bool("video", false, "activity is video media")
	videoMediaTypeShortFlag       = flag.Bool("v", false, "activity is video media (shorthand)")
	bookMediaTypeFlag             = flag.Bool("book", false, "activity is book media")
	bookMediaTypeShortFlag        = flag.Bool("b", false, "activity is book media (shorthand)")
	visualNovelMediaTypeFlag      = flag.Bool("visual-novel", false, "activity is visual novel media")
	visualNovelMediaTypeShortFlag = flag.Bool("vn", false, "activity is visual novel media (shorthand)")
	mangaMediaTypeFlag            = flag.Bool("manga", false, "activity is manga media")
	mangaMediaTypeShortFlag       = flag.Bool("m", false, "activity is manga media (shorthand)")
	animeMediaTypeFlag            = flag.Bool("anime", false, "activity is anime media")
	animeMediaTypeShortFlag       = flag.Bool("a", false, "activity is anime media (shorthand)")
	videoURLFlag                  = flag.String("video-url", "", "URL of the video")
	videoURLShortFlag             = flag.String("vu", "", "URL of the video (shorthand)")
	destinationFlag               = flag.String("o", "", "destination of the log file")
	serverFlag                    = flag.Bool("server", false, "run as a server")
)

func getOneOfNumberFlags[T float64](flags ...*T) T {
	for _, flag := range flags {
		if *flag != 0 {
			return *flag
		}
	}

	return 0
}

func getOneOfStringFlags(flags ...*string) string {
	for _, flag := range flags {
		if *flag != "" {
			return *flag
		}
	}

	return ""
}

func getActivityFromFlags() (a *activities.Activity, err error) {
	a = &activities.Activity{
		CreatedAt: time.Now(),
	}
	name := getOneOfStringFlags(nameFlag, nameShortFlag)
	duration := getOneOfNumberFlags(durationFlag, durationShortFlag)
	date := getOneOfStringFlags(timeFlag, timeShortFlag)
	videoURL := getOneOfStringFlags(videoURLFlag, videoURLShortFlag)
	primaryType := ""
	mediaType := ""

	if *videoMediaTypeFlag || *videoMediaTypeShortFlag {
		mediaType = activities.ActivityMediaTypeVideo
	} else if *bookMediaTypeFlag || *bookMediaTypeShortFlag {
		mediaType = activities.ActivityMediaTypeBook
	} else if *visualNovelMediaTypeFlag || *visualNovelMediaTypeShortFlag {
		mediaType = activities.ActivityMediaTypeVisualNovel
	} else if *mangaMediaTypeFlag || *mangaMediaTypeShortFlag {
		mediaType = activities.ActivityMediaTypeManga
	} else if *animeMediaTypeFlag || *animeMediaTypeShortFlag {
		mediaType = activities.ActivityMediaTypeAnime
	}

	if videoURL != "" {
		u, err := url.Parse(videoURL)

		if err != nil {
			err = fmt.Errorf("invalid video URL: %w", err)
			return nil, err
		}

		videoData, err := activities.GetVideoInfo(context.Background(), u, false)

		if err != nil {
			err = fmt.Errorf("failed to get video info: %w", err)
			return nil, err
		}

		a.PrimaryType = activities.ActivityImmersionTypeListening
		a.MediaType = ref.New(activities.ActivityMediaTypeVideo)
		a.Name = videoData.Title
		a.Duration = videoData.Duration
		a.Date = time.Now()
		a.Meta = videoData
	}

	if *readingTypeFlag || *readingTypeShortFlag {
		primaryType = activities.ActivityImmersionTypeReading
	} else if *listeningTypeFlag || *listeningTypeShortFlag {
		primaryType = activities.ActivityImmersionTypeListening
	} else if a.PrimaryType == "" {
		err = errors.New("no immersion type specified")
		return
	}

	switch mediaType {
	case activities.ActivityMediaTypeVideo:
		a.MediaType = ref.New(activities.ActivityMediaTypeVideo)
	case activities.ActivityMediaTypeBook:
		a.MediaType = ref.New(activities.ActivityMediaTypeBook)
	case activities.ActivityMediaTypeVisualNovel:
		a.MediaType = ref.New(activities.ActivityMediaTypeVisualNovel)
	case activities.ActivityMediaTypeManga:
		a.MediaType = ref.New(activities.ActivityMediaTypeManga)
	case activities.ActivityMediaTypeAnime:
		a.MediaType = ref.New(activities.ActivityMediaTypeAnime)
	}

	if name != "" {
		a.Name = name
	} else if a.Name == "" {
		err = errors.New("no name specified")
		return
	}

	if duration != 0 {
		a.Duration = time.Duration(duration) * time.Minute
	} else if a.Duration == 0 {
		err = errors.New("no duration specified")
		return
	}

	if date != "" {
		a.Date, err = time.Parse(time.RFC3339, date)

		if err != nil {
			err = fmt.Errorf("invalid date: %w", err)
			return
		}
	} else if a.Date.IsZero() {
		a.Date = time.Now()
	}

	if primaryType != "" {
		a.PrimaryType = primaryType
	} else if a.PrimaryType == "" {
		err = errors.New("no immersion type specified")
		return
	}

	return
}

func getLogFile() (w io.WriteCloser, err error) {
	if *destinationFlag != "" {
		w, err = os.OpenFile(*destinationFlag, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

		if err != nil {
			err = fmt.Errorf("failed to open file: %s: %w", *destinationFlag, err)
		}

		return
	}

	basePath := os.Getenv("HOME") + "/.local/share/botsu"

	if xdgDataHome, ok := os.LookupEnv("XDG_DATA_HOME"); ok {
		basePath = xdgDataHome + "/botsu"
	}

	dateString := time.Now().Format("2006-01-02")
	fileName := fmt.Sprintf("%s/%s.jsonl", basePath, dateString)
	err = os.MkdirAll(basePath, 0755)

	if err != nil {
		err = fmt.Errorf("failed to create directory: %s: %w", basePath, err)
		return
	}

	w, err = os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		err = fmt.Errorf("failed to open file: %s: %w", fileName, err)
	}

	return
}

func logActivity(a *activities.Activity) (err error) {
	logFile, err := getLogFile()

	if err != nil {
		return
	}

	defer logFile.Close()

	encoder := json.NewEncoder(logFile)

	if err = encoder.Encode(a); err != nil {
		err = fmt.Errorf("failed to encode activity: %w", err)
	}

	return
}

func Usage() {
	out := flag.CommandLine.Output()

	fmt.Fprintf(out, "Usage: %s [flags]\n", os.Args[0])
	fmt.Fprintln(out, "Logs an activity to the local activity log.")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Examples:")
	fmt.Fprintf(out, "  %s -a -n \"Bocchi the Rock\" -d 20\n", os.Args[0])
	fmt.Fprintf(out, "  %s -vu \"https://youtu.be/Wb5A5fAuzJM\"\n", os.Args[0])
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Flags:")
	fmt.Fprintln(out, "  -n, --name string")
	fmt.Fprintln(out, "        name of the activity")
	fmt.Fprintln(out, "  -r, --reading")
	fmt.Fprintln(out, "        activity is reading")
	fmt.Fprintln(out, "  -l, --listening")
	fmt.Fprintln(out, "        activity is listening")
	fmt.Fprintln(out, "  -d, --duration float")
	fmt.Fprintln(out, "        duration of the activity in minutes")
	fmt.Fprintln(out, "  -t, --time string")
	fmt.Fprintln(out, "        time of the activity (RFC3339 format)")
	fmt.Fprintln(out, "  -v, --video")
	fmt.Fprintln(out, "        activity is video media")
	fmt.Fprintln(out, "  -b, --book")
	fmt.Fprintln(out, "        activity is book media")
	fmt.Fprintln(out, "  -vn, --visual-novel")
	fmt.Fprintln(out, "        activity is visual novel media")
	fmt.Fprintln(out, "  -m, --manga")
	fmt.Fprintln(out, "        activity is manga media")
	fmt.Fprintln(out, "  -a, --anime")
	fmt.Fprintln(out, "        activity is anime media")
	fmt.Fprintln(out, "  -vu, --video-url string")
	fmt.Fprintln(out, "        URL of the video")
	fmt.Fprintln(out, "  -o string")
	fmt.Fprintln(out, "        destination of the log file (default: ~/.local/share/botsu/YYYY-MM-DD.jsonl)")
	fmt.Fprintln(out)
}

func runServer() error {
	mu := sync.Mutex{}

	http.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		var a *activities.Activity

		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			fmt.Fprintf(os.Stderr, "\u001b[31mError: %s\u001b[0m\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// workaround for UserID check
		a.UserID = strings.Repeat("0", 18)

		if err := activities.ValidateExternalActivity(a); err != nil {
			fmt.Fprintf(os.Stderr, "\u001b[31mError: %s\u001b[0m\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		a.UserID = ""

		if err := logActivity(a); err != nil {
			fmt.Fprintf(os.Stderr, "\u001b[31mError: %s\u001b[0m\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	})

	return http.ListenAndServe(":5300", nil)
}

func main() {
	flag.Usage = Usage
	flag.Parse()

	if *serverFlag {
		err := runServer()

		if err != nil {
			fmt.Fprintf(os.Stderr, "\u001b[31mError: %s\u001b[0m\n\n", err)
			os.Exit(1)
		}
	}

	a, err := getActivityFromFlags()

	if err != nil {
		fmt.Fprintf(os.Stderr, "\u001b[31mError: %s\u001b[0m\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	if err = logActivity(a); err != nil {
		fmt.Fprintf(os.Stderr, "\u001b[31mError: %s\u001b[0m\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\u001b[32mLogged activity: %s\u001b[0m\n", a.Name)
}
