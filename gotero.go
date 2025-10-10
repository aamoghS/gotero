/*
Package gotero is an open-source Go client for interacting with the Zotero API.
It allows users to fetch collections and download PDFs concurrently.
*/

package gotero

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Client manages communication with the Zotero API.
type Client struct {
	GroupID string
	APIKey  string
	BaseURL string
}

// NewClient creates a new Gotero API client
func NewClient(groupID, apiKey string) *Client {
	return &Client{
		GroupID: groupID,
		APIKey:  apiKey,
		BaseURL: "https://api.zotero.org",
	}
}

// ExportCollection finds a top-level collection by name and downloads all PDFs recursively.
func (c *Client) ExportCollection(parentName, outputDir string) error {
	topLevelCollections, err := c.fetchTopLevelCollections()
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

	fmt.Printf("Found parent collection: %s (%s)\n", parentCollection.Data.Name, parentCollection.Key)
	return c.processCollectionRecursively(parentCollection, outputDir)
}

func (c *Client) processCollectionRecursively(collection *Collection, outputDir string) error {
	fmt.Printf("Processing: %s\n", collection.Data.Name)
	c.downloadPDFsForCollection(collection.Key, collection.Data.Name, outputDir)

	subcollections, err := c.fetchSubcollections(collection.Key)
	if err != nil {
		return fmt.Errorf("could not fetch subcollections for %s: %w", collection.Data.Name, err)
	}

	for i := range subcollections {
		subOutputDir := filepath.Join(outputDir, sanitizeFileName(collection.Data.Name))
		if err := c.processCollectionRecursively(&subcollections[i], subOutputDir); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) fetchTopLevelCollections() ([]Collection, error) {
	url := fmt.Sprintf("%s/groups/%s/collections/top?limit=100", c.BaseURL, c.GroupID)
	var collections []Collection
	err := c.fetchJSON(url, &collections)
	return collections, err
}

func (c *Client) fetchSubcollections(parentKey string) ([]Collection, error) {
	url := fmt.Sprintf("%s/groups/%s/collections/%s/collections?limit=100", c.BaseURL, c.GroupID, parentKey)
	var collections []Collection
	err := c.fetchJSON(url, &collections)
	return collections, err
}

func (c *Client) fetchJSON(url string, target interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Zotero-API-Key", c.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// downloadTask represents a single PDF download job
type downloadTask struct {
	ItemKey  string
	FilePath string
}

// downloadPDFsForCollection fetches items in a collection and downloads their PDFs concurrently
func (c *Client) downloadPDFsForCollection(collectionKey, collectionName, outputDir string) {
	itemsURL := fmt.Sprintf("%s/groups/%s/collections/%s/items?limit=100", c.BaseURL, c.GroupID, collectionKey)
	var items []Item
	if err := c.fetchJSON(itemsURL, &items); err != nil {
		fmt.Printf("  Error fetching items for collection %s: %v\n", collectionName, err)
		return
	}

	// Sort items by filename
	sort.Slice(items, func(i, j int) bool {
		return items[i].Data.Filename < items[j].Data.Filename
	})

	folder := filepath.Join(outputDir, sanitizeFileName(collectionName))
	_ = os.MkdirAll(folder, 0o755)

	tasks := make(chan downloadTask)
	var wg sync.WaitGroup

	// Start 4 concurrent workers
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				if err := c.downloadPDF(task.ItemKey, task.FilePath); err != nil {
					fmt.Printf("  Failed to download %s: %v\n", task.FilePath, err)
				}
			}
		}()
	}

	for _, item := range items {
		if item.Data.ItemType == "attachment" && item.Data.ContentType == "application/pdf" {
			tasks <- downloadTask{ItemKey: item.Key, FilePath: filepath.Join(folder, item.Data.Filename)}
			continue
		}

		childrenURL := fmt.Sprintf("%s/groups/%s/items/%s/children", c.BaseURL, c.GroupID, item.Key)
		var children []Item
		if err := c.fetchJSON(childrenURL, &children); err != nil {
			fmt.Printf("  Error fetching children for item %s: %v\n", item.Data.Title, err)
			continue
		}

		sort.Slice(children, func(i, j int) bool {
			return children[i].Data.Filename < children[j].Data.Filename
		})

		for _, child := range children {
			if child.Data.ItemType == "attachment" && child.Data.ContentType == "application/pdf" {
				tasks <- downloadTask{ItemKey: child.Key, FilePath: filepath.Join(folder, child.Data.Filename)}
			}
		}
	}

	close(tasks)
	wg.Wait()
}

func (c *Client) downloadPDF(itemKey, filePath string) error {
	url := fmt.Sprintf("%s/groups/%s/items/%s/file", c.BaseURL, c.GroupID, itemKey)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Zotero-API-Key", c.APIKey)

	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.Header.Set("Zotero-API-Key", c.APIKey)
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// sanitizeFileName replaces invalid filename characters
func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "-", "*", "_", "?", "_",
		"\"", "'", "<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(name)
}
