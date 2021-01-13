package spiderdata

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
)

// Category associates a name to a URL
type Category struct {
	Name string
	URL  string
}

// CategoryMap maps a name to a Category
type CategoryMap map[string]Category

// DownloadEnt tells us whether we have downloaded an entry or not yet
type DownloadEnt struct {
	URL  string
	Used bool
}

// DownloadEntMap maps the URLs to the DownloadEnt values
type DownloadEntMap map[string]DownloadEnt

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

// Globals contains common data for the entire module
type Globals struct {
	// Gobilda Spreadsheet of parts and thier status
	ReferenceData *ReferenceDataEnt

	// Protect access to tables
	Mu sync.Mutex

	// Duplicates table
	BreadcrumbMap map[string]string
	CatMap        CategoryMap
	DownloadMap   DownloadEntMap
	LastCategory  string
	Linenum       int
	TargetConfig  *SpiderTarget
	Outfile       *os.File
}

// Context provides the globals used everywhere
type Context struct {
	Cmd fetchbot.Command
	Q   *fetchbot.Queue
	G   *Globals
}

// SpiderTarget provides the information for spidering a given vendor
type SpiderTarget struct {
	Outfile        string
	SpreadsheetID  string
	Presets        []string
	Seed           string
	ParsePageFunc  func(ctx *Context, doc *goquery.Document)
	CheckMatchFunc func(ctx *Context, partData *PartData)
}

// SaveCategory Saves a found Category URL
func SaveCategory(ctx *Context, name string, catclass string, url string) bool {
	entry, found := ctx.G.CatMap[catclass]
	if found {
		if entry.Name != name {
			OutputError(ctx, "Adding: %s name %s did not match previous name %s\n", name, catclass, entry.Name)
		}
		if entry.URL != url {
			if entry.URL == "" {
				entry.URL = url
			} else {
				OutputError(ctx, "Adding: %s Url %s did not match previous url %s\n", name, url, entry.URL)
			}
		}
	} else {
		ctx.G.CatMap[name] = Category{catclass, url}
	}
	return true
}

// MakeBreadCrumb merges breadcrumbs into a printable string
func MakeBreadCrumb(ctx *Context, base string, toadd string) (result string) {
	result = base
	if toadd != "" {
		toadd = strings.ReplaceAll(toadd, "\u00A0", " ")

		if result != "" {
			result += " > "
		}
		result += toadd
	}
	return
}

// CleanURL removes any selector from a URL returning the cleaned string and an indication that it was removed
func CleanURL(url string) (result string, stripped bool) {
	stripped = false
	// Trim off any ?sku= on the URL
	pos := strings.Index(url, "?")
	if pos > 0 { // note > and not >= because we don't want to get an empty URL
		// Trim off any ?sku parameters
		url = url[:pos]
		stripped = true
	}
	result = url
	return
}

// EnqueURL puts a URL on the queue
func EnqueURL(ctx *Context, url string, breadcrumb string) {
	// Resolve address
	fmt.Printf("+++Enqueue:%s\n", url)
	if ctx.Cmd != nil {
		u, err := ctx.Cmd.URL().Parse(url)
		if err != nil {
			fmt.Printf("error: resolve URL %s - %s\n", url, err)
			return
		}
		url = u.String()
	}
	// Trim off any sku= on the URL
	urlString, _ := CleanURL(url)
	_, found := ctx.G.BreadcrumbMap[urlString]
	if !found {
		if _, err := ctx.Q.SendStringHead(urlString); err != nil {
			fmt.Printf("error: enqueue head %s - %s\n", url, err)
		} else {
			ctx.G.BreadcrumbMap[urlString] = breadcrumb
		}
	}
}

// MarkVisitedURL allows us to mark a page which has been received as part of a 301 redirect.
// It prevents us from visiting a page twice (in theory)
func MarkVisitedURL(ctx *Context, url string, breadcrumb string) {
	u, err := ctx.Cmd.URL().Parse(url)
	if err != nil {
		fmt.Printf("error: resolve URL %s - %s\n", url, err)
		return
	}
	_, found := ctx.G.BreadcrumbMap[u.String()]
	if !found {
		ctx.G.BreadcrumbMap[u.String()] = breadcrumb
	}
}

// The output routines write the messages in two places.
//  First it puts a status on stdout so that the you can see what is happening
//  It also puts in lines in the output file so that it can be pulled into a spreadsheet
//  Note that the lines are numbered with columns separated by a backtick because sometimes
//  we may see tabs in the names

// OutputHeader generates the first line of the output file with the column headers
// Note that we use ` to separate columns because we sometimes see tabs in the names
func OutputHeader(ctx *Context) {
	fmt.Fprintf(ctx.G.Outfile, "%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v\n",
		"Order", "Section", "Name", "Part #", "Combined Name", "URL", "Model URL", "Extra 1", "Extra 2", "Extra 3", "Extra 4", "Extra 5", "Extra 6", "Extra 7", "Onshape URL", "Model Status", "Spider Status", "Notes")
}

// OutputCategory puts in a category line at the start of each new section
func OutputCategory(ctx *Context, breadcrumbs string, trimlast bool) {
	fmt.Printf("+++OptputCategory: '%v' trim:%v\n", breadcrumbs, trimlast)
	category := breadcrumbs
	if trimlast {
		offset := strings.LastIndex(category, " > ")
		if offset != -1 {
			category = string(category[0:offset])
		}
	}
	if category != ctx.G.LastCategory {
		fmt.Printf("|CATEGORY:|%s\n", category)
		// fmt.Fprintf(outfile, "%d`CATEGORY: %s\n", linenum, category)
		ctx.G.LastCategory = category
	}
}

// OutputProduct takes the spidered information and generates the output structure
func OutputProduct(ctx *Context, name string, sku string, url string, modelURL string, isDiscontinued bool, extra []string) {
	var partData PartData
	partData.Name = name
	partData.SKU = sku
	partData.URL = url
	partData.ModelURL = modelURL
	partData.Section = ctx.G.LastCategory
	if extra != nil {
		for i, s := range extra {
			partData.Extra[i] = s
		}
	}
	partData.Order = uint(ctx.G.Linenum)
	ctx.G.Linenum++

	ctx.G.TargetConfig.CheckMatchFunc(ctx, &partData)

	if isDiscontinued {
		partData.SpiderStatus = "Discontinued"
	}
	OutputPartData(ctx, &partData)
}

// OutputPartData generates the product line for the output file and also prints a status message on stdout
func OutputPartData(ctx *Context, partData *PartData) {

	fmt.Printf("%s |SKU: '%v' Product: '%v' Model:'%v' on page '%v'\n", partData.SpiderStatus, partData.SKU, partData.Name, partData.ModelURL, partData.URL)

	fmt.Fprintf(ctx.G.Outfile, "%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v\n",
		partData.Order,
		partData.Section,
		partData.Name,
		partData.SKU,
		strings.TrimSpace(partData.Name+" "+partData.SKU),
		partData.URL,
		partData.ModelURL,
		partData.Extra[0], partData.Extra[1], partData.Extra[2], partData.Extra[3], partData.Extra[4], partData.Extra[5], partData.Extra[6],
		partData.OnshapeURL,
		partData.Status,
		partData.SpiderStatus,
		partData.Notes)
}

// OutputError generates an error line in the output file (typically a missing download) and
// also prints the status message on stdout
func OutputError(ctx *Context, message string, args ...interface{}) {
	fmt.Printf("***"+message, args...)
	outmsg := fmt.Sprintf("%d`***", ctx.G.Linenum) + message
	fmt.Fprint(ctx.G.Outfile, fmt.Sprintf(outmsg, args...))
	ctx.G.Linenum++
}

// ExcludeFromMatch checks to see whether something should be spidered
func ExcludeFromMatch(ctx *Context, partdata *PartData) bool {
	exclude := false
	if strings.HasPrefix(partdata.Name, "--") {
		exclude = true
	}
	if strings.HasPrefix(partdata.SKU, "(Configurable)") {
		exclude = true
	}
	if strings.HasPrefix(partdata.SKU, "(Configurable)") {
		exclude = true
	}
	if strings.HasPrefix(partdata.SKU, "(No Part Number)") {
		exclude = true
	}
	if exclude {
		partdata.Order = uint(ctx.G.Linenum)
		partdata.SpiderStatus = "Same"
		OutputPartData(ctx, partdata)
		ctx.G.Linenum++
	}
	return exclude
}

// NilParsePage is the dummy parser when no vendor is selected
func NilParsePage(ctx *Context, doc *goquery.Document) {}

// NilCheckMatch is the dummy check match cleanup routine
func NilCheckMatch(ctx *Context, partData *PartData) {}