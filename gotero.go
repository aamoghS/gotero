package gotero

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// client api
type Client struct {
	GroupID string
	APIKey  string
	BaseURL string
}

// base to utalize the api
func NewClient(groupID, apiKey string) *Client {
	return &Client{
		GroupID: groupID,
		APIKey:  apiKey,
		BaseURL: "https://api.zotero.org",
	}
}

// only gets the pdfs from subcollection
func (c *Client) ExportCollection(parentName, outputDir string) error {
	allCollections, err := c.fetchCollections()
	if err != nil {
		return err
	}

	// goes from the top then runs down
	var datasetKey string
	for _, col := range allCollections {
		if col.Name == parentName {
			datasetKey = col.Key
			break
		}
	}
	if datasetKey == "" {
		return fmt.Errorf("collection %q not found", parentName)
	}

	// walk through
	for _, sub1 := range c.getSubcollections(datasetKey) {
		fmt.Printf("User: %s\n", sub1.Name)
		c.downloadPDFItems(sub1.Key, sub1.Name, outputDir)

		for _, sub2 := range c.getSubcollections(sub1.Key) {
			fmt.Printf("  inner: %s\n", sub2.Name)
			c.downloadPDFItems(sub2.Key, sub2.Name, outputDir)
		}
	}
	return nil
}

func (c *Client) fetchCollections() ([]Collection, error) {
	url := fmt.Sprintf("%s/groups/%s/collections?limit=1000", c.BaseURL, c.GroupID)
	var res []struct {
		Data Collection `json:"data"`
	}
	if err := c.fetchJSON(url, &res); err != nil {
		return nil, err
	}
	out := make([]Collection, 0, len(res))
	for _, r := range res {
		out = append(out, r.Data)
	}
	return out, nil
}

func (c *Client) fetchJSON(url string, target interface{}) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Zotero-API-Key", c.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) getSubcollections(parentKey string) []Collection {
	url := fmt.Sprintf("%s/groups/%s/collections?limit=1000", c.BaseURL, c.GroupID)
	var all []struct {
		Data Collection `json:"data"`
	}
	if err := c.fetchJSON(url, &all); err != nil {
		fmt.Println("Error fetching subcollections:", err)
		return nil
	}

	subs := []Collection{}
	for _, cdata := range all {
		if str, ok := cdata.Data.ParentCollection.(string); ok && str == parentKey {
			subs = append(subs, cdata.Data)
		}
	}
	return subs
}

// grabs the pdfs then saves
func (c *Client) downloadPDFItems(collectionKey, collectionName, outputDir string) {
	itemsURL := fmt.Sprintf("%s/groups/%s/collections/%s/items?limit=100",
		c.BaseURL, c.GroupID, collectionKey)

	var items []struct {
		Data struct {
			Key      string `json:"key"`
			Title    string `json:"title"`
			ItemType string `json:"itemType"`
		} `json:"data"`
	}
	if err := c.fetchJSON(itemsURL, &items); err != nil {
		fmt.Println("  Error fetching items:", err)
		return
	}

	folder := filepath.Join(outputDir, sanitizeFileName(collectionName))
	_ = os.MkdirAll(folder, 0o755)

	for _, item := range items {
		// Direct attachment that’s a PDF.
		if item.Data.ItemType == "attachment" &&
			strings.HasSuffix(strings.ToLower(item.Data.Title), ".pdf") {
			_ = c.downloadPDF(item.Data.Key, filepath.Join(folder, item.Data.Title))
			continue
		}

		// check children
		childrenURL := fmt.Sprintf("%s/groups/%s/items/%s/children",
			c.BaseURL, c.GroupID, item.Data.Key)
		var children []Attachment
		if err := c.fetchJSON(childrenURL, &children); err != nil {
			fmt.Println("  error fetching children:", err)
			continue
		}
		for _, att := range children {
			if att.Data.ContentType == "application/pdf" {
				_ = c.downloadPDF(att.Key, filepath.Join(folder, att.Data.Title))
			}
		}
	}
}

func (c *Client) downloadPDF(itemKey, filePath string) error {
	url := fmt.Sprintf("%s/groups/%s/items/%s/file", c.BaseURL, c.GroupID, itemKey)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Zotero-API-Key", c.APIKey)
	req.Header.Set("Accept", "application/pdf")

	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.Header.Set("Zotero-API-Key", c.APIKey)
			r.Header.Set("Accept", "application/pdf")
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed with status %d", resp.StatusCode)
	}
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return filepath.Clean(name)
}
