package partdata

import (
	"sync"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
)

// SpiderTarget provides the information for spidering a given vendor
type SpiderTarget struct {
	outfile        string
	spreadsheetID  string
	presets        []string
	seed           string
	parsePageFunc  func(ctx *fetchbot.Context, doc *goquery.Document)
	checkMatchFunc func(partData *PartData)
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

// ReferenceData - collection of part numbers and urls
type ReferenceData struct {
	mu         sync.Mutex
	partdata   []*PartData
	partNumber map[string]*PartData
	url        map[string]*PartData

	orderColumnIndex      int
	sectionColumnIndex    int
	nameColumnIndex       int
	skuColumnIndex        int
	urlColumnIndex        int
	modelURLColumnIndex   int
	extraColumnIndex      int
	onShapeURLColumnIndex int
	statusColumnIndex     int
	notesColumnIndex      int
}
