package spiderdata

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
	"github.com/toebes/ftc_parts_spider/partcatalog"
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

// Globals contains common data for the entire module
type Globals struct {
	// Gobilda Spreadsheet of parts and thier status
	ReferenceData *partcatalog.PartCatalogData

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
	SingleOnly    bool
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
	CheckMatchFunc func(ctx *Context, partData *partcatalog.PartData)

	// SectionNameDeletes is the substring which can be removed from the section name safely.
	// e.g. "Shop by Hub Style > "
	SectionNameDeletes []string
	// SectionAllowedMap is the mapping that defines the correct section for specific parts
	SectionAllowedMap map[string]string
	// Section Equivalents is the prefix string from the old to the new section so that if the
	// start of the old section matches and the start of the new section matches, we will use the
	// old section since we assume it was curated.
	// For example:
	//    {"KITS > FTC Kits", "KITS > Linear Motion Kits"},
	// says that if the spider found it in FTC Kits, but the spreadsheet says Linear Motion Kits then we will
	// keep it in the Linear Motion Kits
	SectionEquivalents [][]string
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
	var partData partcatalog.PartData
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
		partData.SpiderStatus = partcatalog.DiscontinuedPart
	}
	OutputPartData(ctx, &partData)
}

// OutputPartData generates the product line for the output file and also prints a status message on stdout
func OutputPartData(ctx *Context, partData *partcatalog.PartData) {

	partData.Println()

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

// NilParsePage is the dummy parser when no vendor is selected
func NilParsePage(ctx *Context, doc *goquery.Document) {}

// NilCheckMatch is the dummy check match cleanup routine
func NilCheckMatch(ctx *Context, partData *partcatalog.PartData) {}
