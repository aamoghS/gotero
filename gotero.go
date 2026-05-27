/*
Package gotero is an open-source Go client for interacting with the Zotero API.
It allows users to fetch collections and download PDFs concurrently.
*/

package gotero

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Client manages communication with the Zotero API.
type Client struct {
	GroupID    string
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new Gotero API client
func NewClient(groupID, apiKey string) *Client {
	return &Client{
		GroupID: groupID,
		APIKey:  apiKey,
		BaseURL: "https://api.zotero.org",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ExportCollection finds a top-level collection by name and downloads all PDFs recursively.
func (c *Client) ExportCollection(ctx context.Context, parentName, outputDir string) error {
	topLevelCollections, err := c.fetchTopLevelCollections(ctx)
	if err != nil {
		return fmt.Errorf("could not fetch top-level collections: %w", err)
	}

	var parentCollection *Collection
	for i := range topLevelCollections {
		if topLevelCollections[i].Data.Name == parentName {
			parentCollection = &topLevelCollections[i]
			break
		}
	}

	if parentCollection == nil {
		return fmt.Errorf("top-level collection %q not found", parentName)
	}

	slog.Info("Found parent collection", "name", parentCollection.Data.Name, "key", parentCollection.Key)
	return c.processCollectionRecursively(ctx, parentCollection, outputDir)
}

func (c *Client) processCollectionRecursively(ctx context.Context, collection *Collection, outputDir string) error {
	slog.Info("Processing collection", "name", collection.Data.Name)
	if err := c.downloadPDFsForCollection(ctx, collection.Key, collection.Data.Name, outputDir); err != nil {
		return fmt.Errorf("failed to download PDFs for %s: %w", collection.Data.Name, err)
	}

	subcollections, err := c.fetchSubcollections(ctx, collection.Key)
	if err != nil {
		return fmt.Errorf("could not fetch subcollections for %s: %w", collection.Data.Name, err)
	}

	for i := range subcollections {
		subOutputDir := filepath.Join(outputDir, sanitizeFileName(collection.Data.Name))
		if err := c.processCollectionRecursively(ctx, &subcollections[i], subOutputDir); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) fetchTopLevelCollections(ctx context.Context) ([]Collection, error) {
	url := fmt.Sprintf("%s/groups/%s/collections/top?limit=100", c.BaseURL, c.GroupID)
	var collections []Collection
	err := c.fetchJSON(ctx, url, &collections)
	return collections, err
}

func (c *Client) fetchSubcollections(ctx context.Context, parentKey string) ([]Collection, error) {
	url := fmt.Sprintf("%s/groups/%s/collections/%s/collections?limit=100", c.BaseURL, c.GroupID, parentKey)
	var collections []Collection
	err := c.fetchJSON(ctx, url, &collections)
	return collections, err
}

func (c *Client) fetchJSON(ctx context.Context, url string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Zotero-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// downloadPDFsForCollection fetches items in a collection and downloads their PDFs concurrently
func (c *Client) downloadPDFsForCollection(ctx context.Context, collectionKey, collectionName, outputDir string) error {
	itemsURL := fmt.Sprintf("%s/groups/%s/collections/%s/items?limit=100", c.BaseURL, c.GroupID, collectionKey)
	var items []Item
	if err := c.fetchJSON(ctx, itemsURL, &items); err != nil {
		return fmt.Errorf("error fetching items for collection %s: %w", collectionName, err)
	}

	// Sort items by filename
	sort.Slice(items, func(i, j int) bool {
		return items[i].Data.Filename < items[j].Data.Filename
	})

	folder := filepath.Join(outputDir, sanitizeFileName(collectionName))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", folder, err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	sem := make(chan struct{}, 4) // max 4 concurrent downloads
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			if err := fn(); err != nil {
				select {
				case errChan <- err:
					cancel() // fail fast
				default:
				}
			}
		}()
	}

	for _, item := range items {
		item := item // capture loop variable
		if item.Data.ItemType == "attachment" && item.Data.ContentType == "application/pdf" {
			run(func() error {
				path := filepath.Join(folder, item.Data.Filename)
				return c.downloadPDF(ctx, item.Key, path, item.Data.Filename)
			})
			continue
		}

		run(func() error {
			childrenURL := fmt.Sprintf("%s/groups/%s/items/%s/children", c.BaseURL, c.GroupID, item.Key)
			var children []Item
			if err := c.fetchJSON(ctx, childrenURL, &children); err != nil {
				slog.Warn("Error fetching children for item", "itemTitle", item.Data.Title, "error", err)
				return nil // Don't fail the entire group for one missing child
			}

			for _, child := range children {
				child := child // capture
				if child.Data.ItemType == "attachment" && child.Data.ContentType == "application/pdf" {
					run(func() error {
						path := filepath.Join(folder, child.Data.Filename)
						return c.downloadPDF(ctx, child.Key, path, child.Data.Filename)
					})
				}
			}
			return nil
		})
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	if err := <-errChan; err != nil {
		return err
	}

	return nil
}

func (c *Client) downloadPDF(ctx context.Context, itemKey, filePath, filename string) error {
	url := fmt.Sprintf("%s/groups/%s/items/%s/file", c.BaseURL, c.GroupID, itemKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Zotero-API-Key", c.APIKey)

	// Use a custom client for this request to handle redirects with auth
	client := &http.Client{
		Timeout: 5 * time.Minute, // PDFs can be large
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.Header.Set("Zotero-API-Key", c.APIKey)
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request to download %s: %w", filename, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed for %s with status %d", filename, resp.StatusCode)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer f.Close()

	slog.Info("Downloading PDF", "filename", filename)

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// sanitizeFileName replaces invalid filename characters
func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "-", "*", "_", "?", "_",
		"\"", "'", "<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(name)
}
