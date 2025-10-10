/*
This file contains all the structs and type definitions used
by the gotero package to map Zotero API responses.
*/

package gotero

// Collection represents a Zotero collection
type Collection struct {
	Key     string         `json:"key"`
	Version int            `json:"version"`
	Library LibraryInfo    `json:"library"`
	Links   Links          `json:"links"`
	Meta    CollectionMeta `json:"meta"`
	Data    CollectionData `json:"data"`
}

// CollectionData contains detailed collection information
type CollectionData struct {
	Key              string      `json:"key"`
	Version          int         `json:"version"`
	Name             string      `json:"name"`
	ParentCollection interface{} `json:"parentCollection"` // nil if top-level
	Relations        Relations   `json:"relations"`
}

// Item represents an item in a Zotero collection
type Item struct {
	Key     string      `json:"key"`
	Version int         `json:"version"`
	Library LibraryInfo `json:"library"`
	Links   Links       `json:"links"`
	Meta    ItemMeta    `json:"meta"`
	Data    ItemData    `json:"data"`
}

// ItemData contains detailed item information
type ItemData struct {
	Key          string    `json:"key"`
	Version      int       `json:"version"`
	ItemType     string    `json:"itemType"`
	Title        string    `json:"title"`
	Creators     []Creator `json:"creators"`
	AbstractNote string    `json:"abstractNote"`
	Date         string    `json:"date"`
	DateAdded    string    `json:"dateAdded"`
	DateModified string    `json:"dateModified"`
	URL          string    `json:"url"`
	AccessDate   string    `json:"accessDate"`
	Tags         []Tag     `json:"tags"`
	Collections  []string  `json:"collections"`
	Relations    Relations `json:"relations"`
	ParentItem   string    `json:"parentItem"`
	LinkMode     string    `json:"linkMode"`
	ContentType  string    `json:"contentType"` // e.g., application/pdf
	Filename     string    `json:"filename"`
	MD5          string    `json:"md5"`
}

// Creator represents an author or contributor
type Creator struct {
	CreatorType string `json:"creatorType"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
}

// Tag represents a Zotero tag
type Tag struct {
	Tag  string `json:"tag"`
	Type int    `json:"type"`
}

// LibraryInfo contains library metadata
type LibraryInfo struct {
	Type  string `json:"type"`
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Links Links  `json:"links"`
}

// Links represents self and alternate links
type Links struct {
	Self      Link `json:"self"`
	Alternate Link `json:"alternate"`
}

// Link represents a URL and its content type
type Link struct {
	Href string `json:"href"`
	Type string `json:"type"`
}

// Relations maps related items or collections
type Relations struct {
	Replaces string `json:"dc:replaces"`
}

// ItemMeta contains metadata for items
type ItemMeta struct {
	CreatorSummary string `json:"creatorSummary"`
	ParsedDate     string `json:"parsedDate"`
	NumChildren    int    `json:"numChildren"`
}

// CollectionMeta contains metadata for collections
type CollectionMeta struct {
	NumCollections int `json:"numCollections"`
	NumItems       int `json:"numItems"`
}
