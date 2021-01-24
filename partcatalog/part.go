package partcatalog

import (
	"fmt"
	"sync"
)

// SpiderStatus - Enum status of the part found by spidering with the part in the catalog.
// Note, when reading in from the spreadsheet, All parts in the catalog start out as 'Not Found' by the spider unless it already was Discontinued
type SpiderStatus int

const (
	// NewPart - SKU was found on website but was not in the spreadsheet
	NewPart SpiderStatus = 0
	// PartNotFoundBySpider - SKU from spreadsheet was not found on the website
	PartNotFoundBySpider SpiderStatus = 1
	// PartChanged - SKU was found on website but some data didn't match.  The Notes field indicates what has changed
	PartChanged SpiderStatus = 2
	// DiscontinuedPart - SKU was identified as discontinued
	DiscontinuedPart SpiderStatus = 3
	// UnchangedPart - Product is the same
	UnchangedPart SpiderStatus = 4
)

func (status SpiderStatus) String() string {
	names := [...]string{
		"New",
		"Not Found by Spider",
		"Changed",
		"Discontinued",
		"Same",
	}
	if status < NewPart || status > UnchangedPart {
		return "Unknown"
	}
	return names[status]
}

// PartData - detailed information about an individual part in our part catalog
type PartData struct {
	Order        uint         // General output order for sorting the spreadsheet
	Section      string       // The path where the part occurs
	Name         string       // Name of the model file
	SKU          string       // Part number/SKU
	URL          string       // URL on the vendor website for the part
	ModelURL     string       // URL on the vendor website for any 3d model
	Extra        [7]string    // Extra items associated with the part
	OnshapeURL   string       // Location of the Onshape model
	Status       string       // Status of the Onshape model (Done, Bundle, etc)
	SpiderStatus SpiderStatus // Status from the latest spidering.
	Notes        string       // Any general information about the part
}

// PartCatalogData - collection of part numbers and urls
type PartCatalogData struct {
	Mu sync.Mutex

	Partdata   []*PartData
	PartNumber map[string]*PartData
	URL        map[string]*PartData

	ExcludeFromSearch []*PartData

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

// NewPartCatalogData - constructor
func NewPartCatalogData() *PartCatalogData {
	referenceData := new(PartCatalogData)
	referenceData.PartNumber = make(map[string]*PartData)
	referenceData.URL = make(map[string]*PartData)
	return referenceData
}

func (catalog *PartCatalogData) addPart(part *PartData, excludeFilter func(*PartData) bool) {
	if catalog.Partdata == nil {
		catalog.Partdata = make([]*PartData, 0, 0)
	}
	catalog.Partdata = append(catalog.Partdata, part)

	exclude := false
	if excludeFilter != nil {
		exclude = excludeFilter(part)
	}

	if exclude {
		if catalog.ExcludeFromSearch == nil {
			catalog.ExcludeFromSearch = make([]*PartData, 0, 0)
		}
		catalog.ExcludeFromSearch = append(catalog.ExcludeFromSearch, part)
	} else {
		dup, ok := catalog.PartNumber[part.SKU]
		if ok {
			fmt.Printf("row %2d: duplicate part number '%s' found (original row %d)\n", len(catalog.Partdata), part.SKU, dup.Order)
		} else {
			catalog.PartNumber[part.SKU] = part
		}
		catalog.URL[part.URL] = part
	}
	//part.Println()
}

func (partData *PartData) toString() string {
	return fmt.Sprintf("%d SKU: '%v' Product: '%v' Model:'%v' on page '%v'", partData.Order, partData.SKU, partData.Name, partData.ModelURL, partData.URL)
}

// Println generates the product line for the output file and also prints a status message on stdout
func (partData *PartData) Println() {
	fmt.Printf("%s\n", partData.toString())
}
