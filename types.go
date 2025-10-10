package gotero

// zotero collection
type Collection struct {
	Key              string      `json:"key"`
	ParentCollection interface{} `json:"parentCollection"`
	Name             string      `json:"name"`
}

//  item structure in zotero
type Item struct {
	Key  string `json:"key"`
	Data struct {
		Title    string `json:"title"`
		ItemType string `json:"itemType"`
	} `json:"data"`
}

//check?

// getting the actual pdf
type Attachment struct {
	Key  string `json:"key"`
	Data struct {
		ContentType string `json:"contentType"`
		Title       string `json:"title"`
	} `json:"data"`
}
