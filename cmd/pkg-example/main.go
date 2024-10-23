package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/xoltia/botsu/pkg/activities"
)

var path = flag.String("path", "", "path to export file")

func main() {
	flag.Parse()

	if *path == "" {
		log.Fatal("path is required")
	}

	f, err := os.Open(*path)

	if err != nil {
		log.Fatal(err)
	}

	activities, err := activities.ReadCompressedJSONL(f)

	if err != nil {
		log.Fatal(err)
	}

	for _, activity := range activities {
		fmt.Println(activity.Name)
	}
}
