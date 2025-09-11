package zoteroparse

// Collection describes a Zotero collection.
type Collection struct {
	Key              string      `json:"key"`
	ParentCollection interface{} `json:"parentCollection"`
	Name             string      `json:"name"`
}

// Item is a top-level Zotero item.
type Item struct {
	Key  string `json:"key"`
	Data struct {
		Title    string `json:"title"`
		ItemType string `json:"itemType"`
	} `json:"data"`
}

// Attachment represents an attached file (e.g., a PDF).
type Attachment struct {
	Key  string `json:"key"`
	Data struct {
		ContentType string `json:"contentType"`
		Title       string `json:"title"`
	} `json:"data"`
}
