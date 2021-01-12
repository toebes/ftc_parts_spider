package spiderdata

import (
	"sync"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
)

// Category associates a name to a URL
type Category struct {
	name string
	url  string
}

// CategoryMap maps a name to a Category
type CategoryMap map[string]Category

// DownloadEnt tells us whether we have downloaded an entry or not yet
type DownloadEnt struct {
	url  string
	used bool
}

// DownloadEntMap maps the URLs to the DownloadEnt values
type DownloadEntMap map[string]DownloadEnt

// SpiderTarget provides the information for spidering a given vendor
type SpiderTarget struct {
	Outfile        string
	SpreadsheetID  string
	Presets        []string
	Seed           string
	ParsePageFunc  func(ctx *fetchbot.Context, doc *goquery.Document)
	CheckMatchFunc func(partData *PartData)
}

// PartData collects all the information about an individual part.
// It is read in from the spreadsheet and updated by the spider
type PartData struct {
	Order   uint   // General output order for sorting the spreadsheet
	Section string // The path where the part occurs
	Name    string // Name of the model file
	SKU     string // Part number/SKU
	// CombinedName string   // This really doesn't need to be stored as it is created by concatenating Name and SKU
	URL          string    // URL on the vendor website for the part
	ModelURL     string    // URL on the vendor website for any 3d model
	Extra        [7]string // Extra items associated with the part
	OnshapeURL   string    // Location of the Onshape model
	Status       string    // Status of the Onshape model (Done, Bundle, etc)
	SpiderStatus string    // Status from the latest spidering.  Possible values are
	//                            New            SKU was found on website but was not in the spreadsheet
	//                            Not Found      SKU from spreadsheet was not found on the website
	//                            Changed        SKU was found on website but some data didn't match.  The Notes field indicates what has changed
	//                            Discontinued   SKU was identified as discontinued
	//                            Same           Product is the same
	//                      Note, when reading in from the spreadsheet, the value should be initialized to Not Found unless it already was Discontinued
	Notes string // Any general information about the part
}

// ReferenceDataEnt - collection of part numbers and urls
type ReferenceDataEnt struct {
	Mu         sync.Mutex
	Partdata   []*PartData
	PartNumber map[string]*PartData
	URL        map[string]*PartData

	OrderColumnIndex      int
	SectionColumnIndex    int
	NameColumnIndex       int
	SKUColumnIndex        int
	URLColumnIndex        int
	ModelURLColumnIndex   int
	ExtraColumnIndex      int
	OnshapeURLColumnIndex int
	StatusColumnIndex     int
	NotesColumnIndex      int
}

// Gobilda Spreadsheet of parts and thier status
var ReferenceData *ReferenceDataEnt
