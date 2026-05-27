package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/aamoghS/gotero"
)

func main() {
	var apiKey, groupID, outputDir string
	var debug bool

	flag.StringVar(&apiKey, "api-key", os.Getenv("ZOTERO_API_KEY"), "Zotero API Key (default: $ZOTERO_API_KEY)")
	flag.StringVar(&groupID, "group-id", os.Getenv("ZOTERO_GROUP_ID"), "Zotero Group ID (default: $ZOTERO_GROUP_ID)")
	flag.StringVar(&outputDir, "output-dir", ".", "Directory to save exported PDFs")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <collection-name>\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	collectionName := flag.Arg(0)

	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	if apiKey == "" {
		slog.Error("api-key is required (use --api-key or set ZOTERO_API_KEY)")
		os.Exit(1)
	}
	if groupID == "" {
		slog.Error("group-id is required (use --group-id or set ZOTERO_GROUP_ID)")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.Info("Starting export", "collection", collectionName, "outputDir", outputDir, "groupID", groupID)

	client := gotero.NewClient(groupID, apiKey)
	if err := client.ExportCollection(ctx, collectionName, outputDir); err != nil {
		slog.Error("Export failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Export completed successfully")
}
