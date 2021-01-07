package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

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
	//                      Note, when reading in from the spreadsheet, the value should be initialized to Not Found unless it already was Discontinued
	Notes string // Any general information about the part
}

// ReferenceData - collection of part numbers and urls
type ReferenceData struct {
	mu         sync.Mutex
	partdata   []*PartData
	partNumber map[string]*PartData
	url        map[string]*PartData
}

var (
	orderColumnIndex      int = 0
	sectionColumnIndex    int = 1
	nameColumnIndex       int = 2
	skuColumnIndex        int = 3
	combinedNameIndex     int = 4
	urlColumnIndex        int = 5
	modelURLColumnIndex   int = 6
	extraColumnIndex      int = 7
	onShapeURLColumnIndex int = 14
	statusColumnIndex     int = 15
	notesColumnIndex      int = 16
)

type category struct {
	name string
	url  string
}

type categorymap map[string]category
type downloadent struct {
	url  string
	used bool
}
type downloadentmap map[string]downloadent

var (
	// Gobilda Spreadsheet of parts and thier status
	referenceData *ReferenceData

	// Protect access to tables
	mu sync.Mutex
	// Duplicates table
	bcmap        = map[string]string{}
	catmap       = categorymap{}
	downloadmap  = downloadentmap{}
	lastcategory = ""
	linenum      = 1

	// Google Sheets JSON URLS:
	//   Rev Robotics:  https://spreadsheets.google.com/feeds/cells/19Mc9Uj0zoaRr_KmPncf_svNOp9WqIgrzaD7fEiNlBr0/2/public/full?alt=json
	//   ServoCity:     https://spreadsheets.google.com/feeds/cells/15Mm-Thdcpl5fVPs3vnyFUXWthuaV1tacXPJ7xQuoB8A/2/public/full?alt=json
	//   GoBILDA:       https://spreadsheets.google.com/feeds/cells/15XT3v9O0VOmyxqXrgR8tWDyb_CRLQT5-xPfWPdbx4RM/2/public/full?alt=json
	//   AndyMark:      https://spreadsheets.google.com/feeds/cells/1x4SUwNaQ_X687yA6kxPELoe7ZpoCKnnCq1-OsgxUCOw/2/public/full?alt=json
	//   Pitsco:        https://spreadsheets.google.com/feeds/cells/1adykd3BVYUyXsb3vC2A-lNhFNj_Q8Yzd1oXThmSwPio/2/public/full?alt=json
	//
	presets = []string{
		// "https://www.gobilda.com/structure/",
		// "https://www.gobilda.com/motion/",
		// "https://www.gobilda.com/electronics/",
		// "https://www.gobilda.com/hardware/",
		// "https://www.gobilda.com/kits/",

		// "https://www.servocity.com/structure/",
		// "https://www.servocity.com/motion/",
		// "https://www.servocity.com/electronics/",
		// "https://www.servocity.com/hardware/",
		// "https://www.servocity.com/kits/",
	}
	// Command-line flags
	seed = flag.String("seed", "https://www.gobilda.com/aluminum-rex-shafting/", "seed URL") // Servos
	// seed = flag.String("seed", "https://www.gobilda.com/structure/", "seed URL") // Servos
	// seed = flag.String("seed", "https://www.servocity.com/electronics/", "seed URL") // Servos

	cancelAfter   = flag.Duration("cancelafter", 0, "automatically cancel the fetchbot after a given time")
	cancelAtURL   = flag.String("cancelat", "", "automatically cancel the fetchbot at a given URL")
	stopAfter     = flag.Duration("stopafter", 0, "automatically stop the fetchbot after a given time")
	stopAtURL     = flag.String("stopat", "", "automatically stop the fetchbot at a given URL")
	memStats      = flag.Duration("memstats", 0, "display memory statistics at a given interval")
	fileout       = flag.String("out", "file.txt", "Output File")
	spreadsheetID = flag.String("spreadsheet", "15XT3v9O0VOmyxqXrgR8tWDyb_CRLQT5-xPfWPdbx4RM", "spider this spreadsheet")
	outfile       *os.File
)

func main() {
	flag.Parse()

	referenceData = LoadStatusSpreadsheet(*spreadsheetID)

	// Parse the provided seed
	u, err := url.Parse(*seed)
	if err != nil {
		log.Fatal(err)
	}
	// Start our log file to import into Excel
	outfile, err = os.Create(*fileout)
	if err != nil {
		log.Fatal(err)
	}
	outputHeader()

	// Create the muxer
	mux := fetchbot.NewMux()

	// Handle all errors the same
	mux.HandleErrors(fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
	}))

	// Handle GET requests for html responses, to parse the body and enqueue all links as HEAD
	// requests.
	mux.Response().Method("GET").ContentType("text/html").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			// Process the body to find the links
			doc, err := goquery.NewDocumentFromResponse(res)
			if err != nil {
				fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
				return
			}
			// Enqueue all links as HEAD requests
			enqueueLinks(ctx, doc)
		}))

	// Handle HEAD requests for html responses coming from the source host - we don't want
	// to crawl links from other hosts.
	mux.Response().Method("HEAD").Host(u.Host).ContentType("text/html").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			if _, err := ctx.Q.SendStringGet(ctx.Cmd.URL().String()); err != nil {
				fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
			}
		}))

	// Create the Fetcher, handle the logging first, then dispatch to the Muxer
	h := logHandler(mux)
	if *stopAtURL != "" || *cancelAtURL != "" {
		stopURL := *stopAtURL
		if *cancelAtURL != "" {
			stopURL = *cancelAtURL
		}
		h = stopHandler(stopURL, *cancelAtURL != "", logHandler(mux))
	}
	f := fetchbot.New(h)

	// First mem stat print must be right after creating the fetchbot
	if *memStats > 0 {
		// Print starting stats
		printMemStats(nil)
		// Run at regular intervals
		runMemStats(f, *memStats)
		// On exit, print ending stats after a GC
		defer func() {
			runtime.GC()
			printMemStats(nil)
		}()
	}

	f.DisablePoliteness = true
	f.WorkerIdleTTL = 5 * time.Second
	f.AutoClose = true

	// Start processing
	q := f.Start()

	// if a stop or cancel is requested after some duration, launch the goroutine
	// that will stop or cancel.
	if *stopAfter > 0 || *cancelAfter > 0 {
		after := *stopAfter
		stopFunc := q.Close
		if *cancelAfter != 0 {
			after = *cancelAfter
			stopFunc = q.Cancel
		}

		go func() {
			c := time.After(after)
			<-c
			stopFunc()
		}()
	}

	// Enqueue the seed, which is the first entry in the dup map

	for _, val := range presets {
		preloadQueueURL(q, val, "Initial")
	}
	preloadQueueURL(q, *seed, "Home > Competition > FTC")

	// Pre-queue any of the URLs that we had already found
	for _, entry := range referenceData.partNumber {
		preloadQueueURL(q, entry.URL, entry.Section)
	}

	q.Block()

	for _, entry := range referenceData.partNumber {
		entry.SpiderStatus = "Not Found"
		outputPartData(entry)
	}
}

func preloadQueueURL(q *fetchbot.Queue, URL string, breadcrumb string) {
	_, found := bcmap[URL]
	if !found {

		bcmap[URL] = breadcrumb
		_, err := q.SendStringGet(URL)

		fmt.Printf("Queueing: %s\n", URL)
		if err != nil {
			fmt.Printf("[ERR] GET %s - %s\n", URL, err)
		}
	}
}
func runMemStats(f *fetchbot.Fetcher, tick time.Duration) {
	var mu sync.Mutex
	var di *fetchbot.DebugInfo

	// Start goroutine to collect fetchbot debug info
	go func() {
		for v := range f.Debug() {
			mu.Lock()
			di = v
			mu.Unlock()
		}
	}()
	// Start ticker goroutine to print mem stats at regular intervals
	go func() {
		c := time.Tick(tick)
		for range c {
			mu.Lock()
			printMemStats(di)
			mu.Unlock()
		}
	}()
}

func printMemStats(di *fetchbot.DebugInfo) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	buf := bytes.NewBuffer(nil)
	buf.WriteString(strings.Repeat("=", 72) + "\n")
	buf.WriteString("Memory Profile:\n")
	buf.WriteString(fmt.Sprintf("`Alloc: %d Kb\n", mem.Alloc/1024))
	buf.WriteString(fmt.Sprintf("`TotalAlloc: %d Kb\n", mem.TotalAlloc/1024))
	buf.WriteString(fmt.Sprintf("`NumGC: %d\n", mem.NumGC))
	buf.WriteString(fmt.Sprintf("`Goroutines: %d\n", runtime.NumGoroutine()))
	if di != nil {
		buf.WriteString(fmt.Sprintf("`NumHosts: %d\n", di.NumHosts))
	}
	buf.WriteString(strings.Repeat("=", 72))
	fmt.Println(buf.String())
}

// stopHandler stops the fetcher if the stopurl is reached. Otherwise it dispatches
// the call to the wrapped Handler.
func stopHandler(stopurl string, cancel bool, wrapped fetchbot.Handler) fetchbot.Handler {
	return fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		if ctx.Cmd.URL().String() == stopurl {
			fmt.Printf(">>>>> STOP URL %s\n", ctx.Cmd.URL())
			// generally not a good idea to stop/block from a handler goroutine
			// so do it in a separate goroutine
			go func() {
				if cancel {
					ctx.Q.Cancel()
				} else {
					ctx.Q.Close()
				}
			}()
			return
		}
		wrapped.Handle(ctx, res, err)
	})
}

// logHandler prints the fetch information and dispatches the call to the wrapped Handler.
func logHandler(wrapped fetchbot.Handler) fetchbot.Handler {
	return fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		if err == nil {
			fmt.Printf("[%d] %s %s - %s\n", res.StatusCode, ctx.Cmd.Method(), ctx.Cmd.URL(), res.Header.Get("Content-Type"))
		}
		wrapped.Handle(ctx, res, err)
	})
}

func saveCategory(name string, catclass string, url string) bool {
	entry, found := catmap[catclass]
	if found {
		if entry.name != name {
			outputError("Adding: %s name %s did not match previous name %s\n", name, catclass, entry.name)
		}
		if entry.url != url {
			if entry.url == "" {
				entry.url = url
			} else {
				outputError("Adding: %s Url %s did not match previous url %s\n", name, url, entry.url)
			}
		}
	} else {
		catmap[name] = category{catclass, url}
	}
	return true
}
func makeBreadCrumb(base string, toadd string) (result string) {
	result = base
	if toadd != "" {
		if result != "" {
			result += " > "
		}
		result += toadd
	}
	return
}

// getBreadCrumbName returns the breadcrumb associated with a document
// A typical one looks like this:
//     <div class="breadcrumbs">
//     <ul>
//                     <li class="home">
//                             <a href="https://www.servocity.com/" title="Go to Home Page">Home</a>
//                                         <span>&gt; </span>
//                         </li>
//                     <li class="category30">
//                             <a href="https://www.servocity.com/motion-components" title="">Motion Components</a>
//                                         <span>&gt; </span>
//                         </li>
//                     <li class="category44">
//                             <a href="https://www.servocity.com/motion-components/linear-motion" title="">Linear Motion</a>
//                                         <span>&gt; </span>
//                         </li>
//                     <li class="category87">
//                             <strong>Linear Bearings</strong>
//                                     </li>
//             </ul>
// </div>
//
// What we want to get is the name (the sections in the <a> or the <strong>) while building up a database of matches to
// the category since their website seems to put a unique category for each
func getBreadCrumbName(ctx *fetchbot.Context, url string, bc *goquery.Selection) string {
	result := ""
	prevresult := ""
	bc.Find("li.breadcrumb").Each(func(i int, li *goquery.Selection) {
		name := ""
		url := ""
		// See if we have an <a> or a <strong> under the section
		li.Find("a.breadcrumb-label").Each(func(i int, a *goquery.Selection) {
			name = a.Text()
			urlloc, hasurl := a.Attr("href")
			if hasurl {
				url = urlloc
			}
		})
		li.Find("strong").Each(func(i int, a *goquery.Selection) {
			name = a.Text()
		})
		catclass, hasclass := li.Attr("class")
		if hasclass {
			// Catclass is the base string.  We want to find any URL or title for it
			// fmt.Printf("Class: %s for Name: %s url: %s\n", catclass, name, url)
		} else {
			outputError("No Class for name: %s url: %s\n", name, url)
		}
		saveCategory(name, catclass, url)

		prevresult = result
		result = makeBreadCrumb(result, name)
	})
	// fmt.Printf("+++Extracted breadcrumb was '%v' lastname='%v' prevresult='%v'\n", result, lastname, prevresult)
	// Now see if the breadcrumb was Home > Shop All (without the last name)
	if strings.EqualFold(prevresult, "Home > Shop All") {
		// It was, so we need to extract the proper name
		savename, found := bcmap[url]
		// fmt.Printf("+++Checking savename='%v' found=%v for url='%v'\n", savename, found, url)
		if found {
			result = savename
		}
	}
	return result
}

func enqueURL(ctx *fetchbot.Context, url string, breadcrumb string) {
	// Resolve address
	fmt.Printf("+++Enqueue:%s\n", url)
	u, err := ctx.Cmd.URL().Parse(url)
	if err != nil {
		fmt.Printf("error: resolve URL %s - %s\n", url, err)
		return
	}
	_, found := bcmap[u.String()]
	if !found {
		if _, err := ctx.Q.SendStringHead(u.String()); err != nil {
			fmt.Printf("error: enqueue head %s - %s\n", u, err)
		} else {
			// fmt.Printf("+++Really Enqueue:%s\n", url)
			bcmap[u.String()] = breadcrumb
		}
		// } else {
		// fmt.Printf("It was a dup\n")
	}
}

func processSubCategory(ctx *fetchbot.Context, breadcrumbs string, categoryproducts *goquery.Selection) (found bool) {
	found = false
	fmt.Printf("processSubCategory\n")
	categoryproducts.Find("li.navList-item").Each(func(i int, item *goquery.Selection) {
		// fmt.Printf("-Found Category product LI element\n")
		item.Find("a.navList-action").Each(func(i int, elem *goquery.Selection) {
			url, _ := elem.Attr("href")
			elemtext := "<NOT FOUND>"
			elem.Find("span").Each(func(i int, span *goquery.Selection) {
				elemtext = span.Text()
			})
			fmt.Printf("Found item name=%s url=%s\n", elemtext, url)
			found = true
			enqueURL(ctx, url, breadcrumbs)
		})
	})
	return
}

// The output routines write the messages in two places.
//  First it puts a status on stdout so that the you can see what is happening
//  It also puts in lines in the output file so that it can be pulled into a spreadsheet
//  Note that the lines are numbered with columns separated by a backtick because sometimes
// we may see tabs in the names

// --------------------------------------------------------------------------------------------
// outputHeader generates the first line of the output file with the column headers
// Note that we use ` to separate columns because we sometimes see tabs in the names
func outputHeader() {
	fmt.Fprintf(outfile, "%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v\n",
		"Order", "Section", "Name", "Part #", "Combined Name", "URL", "Model URL", "", "Extra 1", "Extra 2", "Extra 3", "Extra 4", "Extra 5", "Extra 6", "Extra 7", "Onshape URL", "Model Status", "Spider Status", "Notes")
}

// --------------------------------------------------------------------------------------------
// outputCategory puts in a category line at the start of each new section
func outputCategory(breadcrumbs string, trimlast bool) {
	fmt.Printf("+++OptputCategory: '%v' trim:%v\n", breadcrumbs, trimlast)
	category := breadcrumbs
	if trimlast {
		offset := strings.LastIndex(category, " > ")
		if offset != -1 {
			category = string(category[0:offset])
		}
	}
	if category != lastcategory {
		fmt.Printf("|CATEGORY:|%s\n", category)
		// fmt.Fprintf(outfile, "%d`CATEGORY: %s\n", linenum, category)
		linenum++
		lastcategory = category
	}
}

// checkMatch compares a partData to what has been captured from the spreadsheet
// Any differences are put into the notes
func checkMatch(partData *PartData) {
	entry, found := referenceData.partNumber[partData.SKU]
	if !found {
		entry, found = referenceData.url[partData.URL]
	}
	if found {
		// We matched a previous entry
		referenceData.mu.Lock()
		delete(referenceData.url, entry.URL)
		delete(referenceData.partNumber, entry.SKU)
		referenceData.mu.Unlock()

		// // Check the contents of the record and see what needs to be consolidated
		extra := ""
		separator := ", "
		// We are gathering everything in the notes section, we need to use the comma between entries just to make it easy
		if partData.Notes != "" {
			extra = separator
		}
		if entry.Notes != "" {
			partData.Notes += extra + entry.Notes
			extra = separator
		}
		// If it was in a different path (the part moved on the website) Then we want to
		// keep the old section and record a message for the new section
		// Note that it may not have moved, but we chose to organize it slightly different
		// A good example of this is hubs which are grouped by hub type
		if !strings.EqualFold(partData.Section, entry.Section) {
			partData.Notes += extra + "New Section:" + partData.Section
			partData.Section = entry.Section
			extra = separator
		}
		// Likewise if the name changed, we want to still use the old one.  This is because
		// Often the website name has something like (2 pack) or a plural that we want to make singular
		// TODO: Write code to map those names as we find them
		if !strings.EqualFold(partData.Name, entry.Name) {
			partData.Notes += extra + "New Name:" + partData.Name
			partData.Name = entry.Name
			extra = separator
		}
		// If the SKU changes then we really want to know it.  We should use the new SKU
		// and stash away the old SKU but it needs to be updated
		if !strings.EqualFold(partData.SKU, entry.SKU) {
			partData.Notes += extra + " Old SKU:" + entry.SKU
			extra = separator
		}
		// If the URL changes then we really want to use it.
		// Just stash away the old URL so we know what happened
		if !strings.EqualFold(partData.URL, entry.URL) {
			partData.Notes += extra + " Old URL:" + entry.URL
			extra = separator
		}
		// For the model, we have the special case of NOMODEL to ignore but we really
		// don't need to record any information
		if !strings.EqualFold(partData.ModelURL, entry.ModelURL) {
			if strings.Contains(strings.ToUpper(partData.ModelURL), "NOMODEL") {
				partData.ModelURL = entry.ModelURL
			}
		}
		// Copy over the Onshape model URL and the part status (unless we already set them)
		// It is possible that the vendor may start putting the onshape URL on the website and we
		// will need to handle that case here.
		if partData.OnshapeURL == "" {
			partData.OnshapeURL = entry.OnshapeURL
		}
		if partData.Status == "" {
			partData.Status = entry.Status
		}
	} else {
		partData.SpiderStatus = "Not Found"
		partData.Status = "Not Done"
	}

}

// outputProduct takes the spidered information and generates the output structure
func outputProduct(name string, sku string, url string, modelURL string, extra []string) {
	var partData PartData
	partData.Name = name
	partData.SKU = sku
	partData.URL = url
	partData.ModelURL = modelURL
	partData.Section = lastcategory
	if extra != nil {
		for i, s := range extra {
			partData.Extra[i] = s
		}
	}

	checkOutputPart(&partData)
}

// --------------------------------------------------------------------------------------------
// checkOutputPart Updated a part data from the match data and then outputs it
func checkOutputPart(partData *PartData) {
	checkMatch(partData)
	outputPartData(partData)
}

// --------------------------------------------------------------------------------------------
// outputPartData generates the product line for the output file and also prints a status message on stdout
func outputPartData(partData *PartData) {

	checkMatch(partData)
	fmt.Printf("%s |SKU: '%v' Product: '%v' Model:'%v' on page '%v'\n", partData.SpiderStatus, partData.SKU, partData.Name, partData.ModelURL, partData.URL)

	fmt.Fprintf(outfile, "%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v\n",
		partData.Order,
		partData.Section,
		partData.Name,
		partData.SKU,
		strings.TrimSpace(partData.Name+" "+partData.SKU),
		partData.URL,
		partData.ModelURL,
		"",
		partData.Extra[0], partData.Extra[1], partData.Extra[2], partData.Extra[3], partData.Extra[4], partData.Extra[5], partData.Extra[6],
		partData.OnshapeURL,
		partData.Status,
		partData.SpiderStatus,
		partData.Notes)
	linenum++
}

// --------------------------------------------------------------------------------------------
// outputError generates an error line in the output file (typically a missing download) and
// also prints the status message on stdout
func outputError(message string, args ...interface{}) {
	fmt.Printf("***"+message, args...)
	outmsg := fmt.Sprintf("%d`***", linenum) + message
	fmt.Fprint(outfile, fmt.Sprintf(outmsg, args...))
	linenum++
}

// --------------------------------------------------------------------------------------------
// findAllDownloads processes all of the content in the DOM looking for the signature download URLS
func findAllDownloads(ctx *fetchbot.Context, url string, root *goquery.Selection) downloadentmap {
	result := downloadentmap{}
	// fmt.Printf("findAllDownloads parent='%v'\n", root.Parent().Text())
	root.Parent().Find("a.product-downloadsList-listItem-link").Each(func(i int, elem *goquery.Selection) {
		// <a class="product-downloadsList-listItem-link ext-zip" title="1309-0016-2005.zip" href="/content/step_files/1309-0016-2005.zip" target="_blank">
		title, hastitle := elem.Attr("title")
		dlurl, foundurl := elem.Attr("href")
		// fmt.Printf("Found a on '%v' href=%v\n", elem.Text(), dlurl)
		if hastitle && foundurl {
			// The title often has a string like " STEP" at the end, so we can throw it away
			title = strings.Replace(title, " STEP", "", -1)
			title = strings.Replace(title, " File", "", -1)
			title = strings.Replace(title, " file", "", -1)
			title = strings.Replace(title, " assembly", "", -1)
			title = strings.Replace(title, ".zip", "", -1)
			title = strings.TrimSpace(title)
			result[title] = downloadent{dlurl, false}
			fmt.Printf("Save Download '%s'='%s'\n", title, dlurl)
		} else {
			if title == "" {
				outputError("No URL found associated with %s on %s\n", title, url)
			} else if foundurl {
				outputError("No Title found for url %s on %s\n", dlurl, url)
			} else {
				outputError("No URL or Title found with:%s on %s\n", elem.Text(), url)
			}
		}
	})
	return result
}

// --------------------------------------------------------------------------------------------
// getDownloadURL looks in the download map for a matching entry and returns the corresponding URL, marking it as used
// from the list of downloads so that we know what is left over
func getDownloadURL(ctx *fetchbot.Context, sku string, downloadurls downloadentmap) (result string) {
	result = "<NOMODEL:" + sku + ">"
	ent, found := downloadurls[sku]
	if found {
		result = ent.url
		downloadurls[sku] = downloadent{ent.url, true}
		return
	}
	ent, found = downloadurls[strings.ToLower(sku)]
	if found {
		result = ent.url
		downloadurls[strings.ToLower(sku)] = downloadent{ent.url, true}
		return
	}
	// We didn't find the sku in the list, but it is possible that they misnamed it.
	// For example https://www.servocity.com/8mm-4-barrel  has a SKU of 545314
	// But the text for the URL is mistyped as '535314' but it links to 'https://www.servocity.com/media/attachment/file/5/4/545314.zip'
	// So we want to try to use it
	for key, element := range downloadurls {
		if !element.used && strings.Index(element.url, sku) >= 0 {
			result = element.url
			downloadurls[key] = downloadent{ent.url, true}
			return
		}
	}
	// We really didn't find it.
	// Try our list of renamed products to see if we can get a download for it that way
	renames := map[string]string{
		"1600-0722-0008": "535034_3",
		"545361":         "545360_1",
		"585756":         "585717",
		"585757":         "585718",
		"3103-0001-0002": "605632",
		"3103-0001-0001": "605634_1",
		"1804-0032-0001": "svm275-115",
		"638230":         "585076",
		"639010":         "585399",
		"33488":          "HS-488HB",
		"33788":          "HS-788HB",
	}
	ent, found = downloadurls[renames[sku]]
	if found {
		result = ent.url
		downloadurls[renames[sku]] = downloadent{ent.url, true}
		return
	}

	// Ok last try.. This is the case where we have a cases such as
	//      HDA8-30  vs   hda8_assembly_2
	// To match, we drop everything after the - and check the list again
	skupart := strings.Split(strings.ToLower(sku), "-")
	for key, element := range downloadurls {
		if !element.used && strings.Index(element.url, skupart[0]) >= 0 {
			result = element.url
			downloadurls[key] = downloadent{ent.url, true}
			return
		}

	}
	return
}

// --------------------------------------------------------------------------------------------
// showUnusedURLs displays all of the downloads which were on a page but not referenced by any
// of the products associated with the page.  Typically this happens because the product number
// doesn't match or because it is an out of date product which has been removed
func showUnusedURLS(ctx *fetchbot.Context, url string, downloadurls downloadentmap) {
	for key, element := range downloadurls {
		if !element.used {
			// If it says instructions or other nonesense, we can ignore it.
			if strings.Index(key, "Instructions") < 0 &&
				strings.Index(key, "Spec Sheet") < 0 &&
				strings.Index(key, "Specs") < 0 &&
				strings.Index(key, "Guide") < 0 &&
				strings.Index(key, "Diagram") < 0 &&
				strings.Index(key, "Charts") < 0 &&
				strings.Index(key, "Manual") < 0 &&
				strings.Index(key, ".pdf") < 0 &&
				strings.Index(key, ".docx") < 0 &&
				strings.Index(key, ".exe") < 0 &&
				strings.Index(key, "arduino") < 0 &&
				strings.Index(key, "oboclaw_") < 0 &&
				strings.Index(key, "RoboclawClassLib") < 0 &&
				strings.Index(key, "USBRoboclawVirtualComport") < 0 &&
				strings.Index(key, "Pattern Information") < 0 &&
				strings.Index(key, "bldc_hsr_") < 0 &&
				strings.Index(key, "595644_assembly") < 0 &&
				strings.Index(key, "Hardware Accessory Pack") < 0 &&
				strings.Index(key, "Use Parameter") < 0 {

				outputError("Unused download``%s``%s`%s\n", key, url, element.url)
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// processProductGrid takes a standard page which has a single product on it and outputs the information
func processProductGrid(ctx *fetchbot.Context, breadcrumbs string, url string, pg *goquery.Selection) (found bool) {
	found = false
	fmt.Printf("Parents found: %d\n", pg.ParentFiltered("div.tab-content").Length())
	if pg.ParentFiltered("div.tab-content").Length() == 0 {
		pg.Find("li.product a[data-card-type],li.product a.card").Each(func(i int, a *goquery.Selection) {
			urlloc, _ := a.Attr("href")
			product, _ := a.Attr("title")
			fmt.Printf("**ProductGrid Found item name=%v url=%v on %v\n", product, urlloc, url)
			found = true
			enqueURL(ctx, urlloc, makeBreadCrumb(breadcrumbs, product))
		})
	}
	return
}

// --------------------------------------------------------------------------------------------
// processProductTableList takes a standard page which has a single product on it and outputs the information
func processProductTableList(ctx *fetchbot.Context, breadcrumbs string, url string, table *goquery.Selection) (found bool) {
	found = false
	// fmt.Printf("Parents found: %d\n", pg.ParentFiltered("div.tab-content").Length())
	if table.ParentFiltered("div.tab-content").Length() == 0 {
		table.Find("td.productTable-cell a.tableSKU").Each(func(i int, a *goquery.Selection) {
			urlloc, _ := a.Attr("href")
			product := a.Text()
			product = strings.Trim(strings.ReplaceAll(product, "\n", ""), " ")
			fmt.Printf("**processProductTableList Found item name=%s url=%s\n", product, urlloc)
			found = true
			enqueURL(ctx, urlloc, makeBreadCrumb(breadcrumbs, product))
		})
	}
	return
}

// --------------------------------------------------------------------------------------------
// processLazyLoad finds all the lazy loaded sub pages
func processLazyLoad(ctx *fetchbot.Context, breadcrumbs string, url string, js *goquery.Selection) (found bool) {
	jstext := js.Text()
	pos := strings.Index(jstext, "window.stencilBootstrap(")
	if pos > 0 {
		// fmt.Printf("Found Bootstrap: %v", jstext)
		pos := strings.Index(jstext, "subcategories")
		if pos > 0 {
			jstext = jstext[pos:]
			pos = strings.Index(jstext, ":[")
			if pos > 0 {
				pos2 := strings.Index(jstext, "],")
				// fmt.Printf("Found Javascript pos=%d pos2=%d '%s'\n", pos, pos2, jstext[pos:pos2])
				if pos2 > 0 {
					// //         <script>
					// // Exported in app.js
					// window.stencilBootstrap("category", "{\"categoryProductsPerPage\":50,
					// 				   \"subcategoryURLs\":[\"https://www.gobilda.com/chain/\",
					// 				   \"https://www.gobilda.com/set-screw-sprockets/\",
					// 				   \"https://www.gobilda.com/14mm-bore-aluminum-hub-mount-sprockets/\",
					// 				   \"https://www.gobilda.com/14mm-bore-plastic-hub-mount-sprockets/\",
					// 				   \"https://www.gobilda.com/32mm-bore-aluminum-hub-mount-sprockets/\"],
					// 				   \"template\":\"pages/custom/category/category-stacked-shortest-name\",
					// 				   \"themeSettings\":{\"alert-backgroundColor\":\"#ffffff\",\"alert-color\":\"#333333\",\"alert-color-alt\":\"#ffffff\",\"applePay-button\":\"black\",\"blockquote-cite-font-color\":\"#999999\",\"blog_size\":\"190x250\",\"body-bg\":\"#ffffff\",\"body-font\":\"Google_Roboto_300\",\"brand_size\":\"190x250\",\"brandpage_products_per_page\":12,\"button--default-borderColor\":\"#cccccc\",\"button--default-borderColorActive\":\"#757575\",\"button--default-borderColorHover\":\"#999999\",\"button--default-color\":\"#666666\",\"button--default-colorActive\":\"#000000\",\"button--default-colorHover\":\"#333333\",\"button--disabled-backgroundColor\":\"#cccccc\",\"button--disabled-borderColor\":\"transparent\",\"button--disabled-color\":\"#ffffff\",\"button--icon-svg-color\":\"#757575\",\"button--primary-backgroundColor\":\"#444444\",\"button--primary-backgroundColorActive\":\"#000000\",\"button--primary-backgroundColorHover\":\"#666666\",\"button--primary-color\":\"#ffffff\",\"button--primary-colorActive\":\"#ffffff\",\"button--primary-colorHover\":\"#ffffff\",\"card--alternate-backgroundColor\":\"#ffffff\",\"card--alternate-borderColor\":\"#ffffff\",\"card--alternate-color--hover\":\"#ffffff\",\"card-figcaption-button-background\":\"#ffffff\",\"card-figcaption-button-color\":\"#333333\",\"card-title-color\":\"#333333\",\"card-title-color-hover\":\"#757575\",\"carousel-arrow-bgColor\":\"#ffffff\",\"carousel-arrow-borderColor\":\"#ffffff\",\"carousel-arrow-color\":\"#999999\",\"carousel-bgColor\":\"#ffffff\",\"carousel-description-color\":\"#333333\",\"carousel-dot-bgColor\":\"#ffffff\",\"carousel-dot-color\":\"#333333\",\"carousel-dot-color-active\":\"#757575\",\"carousel-title-color\":\"#444444\",\"categorypage_products_per_page\":50,\"checkRadio-backgroundColor\":\"#ffffff\",\"checkRadio-borderColor\":\"#cccccc\",\"checkRadio-color\":\"#333333\",\"color-black\":\"#ffffff\",\"color-error\":\"#cc4749\",\"color-errorLight\":\"#ffdddd\",\"color-grey\":\"#999999\",\"color-greyDark\":\"#666666\",\"color-greyDarker\":\"#333333\",\"color-greyDarkest\":\"#000000\",\"color-greyLight\":\"#999999\",\"color-greyLighter\":\"#cccccc\",\"color-greyLightest\":\"#e5e5e5\",\"color-greyMedium\":\"#757575\",\"color-info\":\"#666666\",\"color-infoLight\":\"#dfdfdf\",\"color-primary\":\"#757575\",\"color-primaryDark\":\"#666666\",\"color-primaryDarker\":\"#333333\",\"color-primaryLight\":\"#999999\",\"color-secondary\":\"#ffffff\",\"color-secondaryDark\":\"#e5e5e5\",\"color-secondaryDarker\":\"#cccccc\",\"color-success\":\"#008a06\",\"color-successLight\":\"#d5ffd8\",\"color-textBase\":\"#333333\",\"color-textBase--active\":\"#757575\",\"color-textBase--hover\":\"#757575\",\"color-textHeading\":\"#444444\",\"color-textLink\":\"#333333\",\"color-textLink--active\":\"#757575\",\"color-textLink--hover\":\"#757575\",\"color-textSecondary\":\"#757575\",\"color-textSecondary--active\":\"#333333\",\"color-textSecondary--hover\":\"#333333\",\"color-warning\":\"#f1a500\",\"color-warningLight\":\"#fffdea\",\"color-white\":\"#ffffff\",\"color-whitesBase\":\"#e5e5e5\",\"color_actobotics-green\":\"#43b02a\",\"color_badge_product_sale_badges\":\"#007dc6\",\"color_gobilda-grey\":\"#2c2c2c\",\"color_gobilda-yellow\":\"#fad000\",\"color_hover_product_sale_badges\":\"#000000\",\"color_robotzone-red\":\"#da291c\",\"color_servocity-blue\":\"#0077c8\",\"color_text_product_sale_badges\":\"#ffffff\",\"container-border-global-color-base\":\"#e5e5e5\",\"container-fill-base\":\"#ffffff\",\"container-fill-dark\":\"#e5e5e5\",\"default_image_brand\":\"/assets/img/BrandDefault.gif\",\"default_image_gift_certificate\":\"/assets/img/GiftCertificate.png\",\"default_image_product\":\"/assets/img/ProductDefault.gif\",\"dropdown--quickSearch-backgroundColor\":\"#e5e5e5\",\"fontSize-h1\":28,\"fontSize-h2\":25,\"fontSize-h3\":22,\"fontSize-h4\":20,\"fontSize-h5\":15,\"fontSize-h6\":13,\"fontSize-root\":14,\"footer-backgroundColor\":\"#ffffff\",\"form-label-font-color\":\"#666666\",\"gallery_size\":\"300x300\",\"geotrust_ssl_common_name\":\"\",\"geotrust_ssl_seal_size\":\"M\",\"header-backgroundColor\":\"#ffffff\",\"headings-font\":\"Google_Montserrat_400\",\"hide_content_navigation\":false,\"homepage_blog_posts_count\":3,\"homepage_featured_products_column_count\":6,\"homepage_featured_products_count\":8,\"homepage_new_products_column_count\":6,\"homepage_new_products_count\":12,\"homepage_show_carousel\":true,\"homepage_stretch_carousel_images\":false,\"homepage_top_products_column_count\":6,\"homepage_top_products_count\":8,\"icon-color\":\"#757575\",\"icon-color-hover\":\"#999999\",\"icon-ratingEmpty\":\"#cccccc\",\"icon-ratingFull\":\"#757575\",\"input-bg-color\":\"#ffffff\",\"input-border-color\":\"#cccccc\",\"input-border-color-active\":\"#999999\",\"input-disabled-bg\":\"#ffffff\",\"input-font-color\":\"#666666\",\"label-backgroundColor\":\"#cccccc\",\"label-color\":\"#ffffff\",\"loadingOverlay-backgroundColor\":\"#ffffff\",\"logo-font\":\"Google_Oswald_300\",\"logo-position\":\"center\",\"logo_fontSize\":28,\"logo_size\":\"250x100\",\"medium_size\":\"800x800\",\"navPages-color\":\"#333333\",\"navPages-color-hover\":\"#757575\",\"navPages-subMenu-backgroundColor\":\"#e5e5e5\",\"navPages-subMenu-separatorColor\":\"#cccccc\",\"navUser-color\":\"#333333\",\"navUser-color-hover\":\"#757575\",\"navUser-dropdown-backgroundColor\":\"#ffffff\",\"navUser-dropdown-borderColor\":\"#cccccc\",\"navUser-indicator-backgroundColor\":\"#333333\",\"navigation_design\":\"simple\",\"optimizedCheckout-backgroundImage\":\"\",\"optimizedCheckout-backgroundImage-size\":\"1000x400\",\"optimizedCheckout-body-backgroundColor\":\"#ffffff\",\"optimizedCheckout-buttonPrimary-backgroundColor\":\"#333333\",\"optimizedCheckout-buttonPrimary-backgroundColorActive\":\"#000000\",\"optimizedCheckout-buttonPrimary-backgroundColorDisabled\":\"#cccccc\",\"optimizedCheckout-buttonPrimary-backgroundColorHover\":\"#666666\",\"optimizedCheckout-buttonPrimary-borderColor\":\"#cccccc\",\"optimizedCheckout-buttonPrimary-borderColorActive\":\"transparent\",\"optimizedCheckout-buttonPrimary-borderColorDisabled\":\"transparent\",\"optimizedCheckout-buttonPrimary-borderColorHover\":\"transparent\",\"optimizedCheckout-buttonPrimary-color\":\"#ffffff\",\"optimizedCheckout-buttonPrimary-colorActive\":\"#ffffff\",\"optimizedCheckout-buttonPrimary-colorDisabled\":\"#ffffff\",\"optimizedCheckout-buttonPrimary-colorHover\":\"#ffffff\",\"optimizedCheckout-buttonPrimary-font\":\"Google_Roboto_300\",\"optimizedCheckout-buttonSecondary-backgroundColor\":\"#ffffff\",\"optimizedCheckout-buttonSecondary-backgroundColorActive\":\"#e5e5e5\",\"optimizedCheckout-buttonSecondary-backgroundColorHover\":\"#f5f5f5\",\"optimizedCheckout-buttonSecondary-borderColor\":\"#cccccc\",\"optimizedCheckout-buttonSecondary-borderColorActive\":\"#757575\",\"optimizedCheckout-buttonSecondary-borderColorHover\":\"#999999\",\"optimizedCheckout-buttonSecondary-color\":\"#333333\",\"optimizedCheckout-buttonSecondary-colorActive\":\"#000000\",\"optimizedCheckout-buttonSecondary-colorHover\":\"#333333\",\"optimizedCheckout-buttonSecondary-font\":\"Google_Karla_400\",\"optimizedCheckout-colorFocus\":\"#4496f6\",\"optimizedCheckout-contentPrimary-color\":\"#333333\",\"optimizedCheckout-contentPrimary-font\":\"Google_Roboto_300\",\"optimizedCheckout-contentSecondary-color\":\"#757575\",\"optimizedCheckout-contentSecondary-font\":\"Google_Roboto_300\",\"optimizedCheckout-discountBanner-backgroundColor\":\"#e5e5e5\",\"optimizedCheckout-discountBanner-iconColor\":\"#333333\",\"optimizedCheckout-discountBanner-textColor\":\"#333333\",\"optimizedCheckout-form-textColor\":\"#666666\",\"optimizedCheckout-formChecklist-backgroundColor\":\"#ffffff\",\"optimizedCheckout-formChecklist-backgroundColorSelected\":\"#f5f5f5\",\"optimizedCheckout-formChecklist-borderColor\":\"#cccccc\",\"optimizedCheckout-formChecklist-color\":\"#333333\",\"optimizedCheckout-formField-backgroundColor\":\"#ffffff\",\"optimizedCheckout-formField-borderColor\":\"#cccccc\",\"optimizedCheckout-formField-errorColor\":\"#d14343\",\"optimizedCheckout-formField-inputControlColor\":\"#476bef\",\"optimizedCheckout-formField-placeholderColor\":\"#999999\",\"optimizedCheckout-formField-shadowColor\":\"#e5e5e5\",\"optimizedCheckout-formField-textColor\":\"#333333\",\"optimizedCheckout-header-backgroundColor\":\"#f5f5f5\",\"optimizedCheckout-header-borderColor\":\"#dddddd\",\"optimizedCheckout-header-textColor\":\"#333333\",\"optimizedCheckout-headingPrimary-color\":\"#333333\",\"optimizedCheckout-headingPrimary-font\":\"Google_Montserrat_400\",\"optimizedCheckout-headingSecondary-color\":\"#333333\",\"optimizedCheckout-headingSecondary-font\":\"Google_Montserrat_400\",\"optimizedCheckout-link-color\":\"#476bef\",\"optimizedCheckout-link-font\":\"Google_Karla_400\",\"optimizedCheckout-link-hoverColor\":\"#002fe1\",\"optimizedCheckout-loadingToaster-backgroundColor\":\"#333333\",\"optimizedCheckout-loadingToaster-textColor\":\"#ffffff\",\"optimizedCheckout-logo\":\"\",\"optimizedCheckout-logo-position\":\"left\",\"optimizedCheckout-logo-size\":\"250x100\",\"optimizedCheckout-orderSummary-backgroundColor\":\"#ffffff\",\"optimizedCheckout-orderSummary-borderColor\":\"#dddddd\",\"optimizedCheckout-show-backgroundImage\":false,\"optimizedCheckout-show-logo\":\"none\",\"optimizedCheckout-step-backgroundColor\":\"#757575\",\"optimizedCheckout-step-borderColor\":\"#dddddd\",\"optimizedCheckout-step-textColor\":\"#ffffff\",\"overlay-backgroundColor\":\"#333333\",\"pace-progress-backgroundColor\":\"#999999\",\"price_ranges\":true,\"product_list_display_mode\":\"grid\",\"product_sale_badges\":\"none\",\"product_size\":\"500x659\",\"productgallery_size\":\"318x318\",\"productgallery_size_three_column\":\"712x712\",\"productpage_related_products_count\":100,\"productpage_reviews_count\":9,\"productpage_similar_by_views_count\":10,\"productpage_videos_count\":8,\"productthumb_size\":\"100x100\",\"productview_thumb_size\":\"50x50\",\"restrict_to_login\":false,\"searchpage_products_per_page\":12,\"select-arrow-color\":\"#757575\",\"select-bg-color\":\"#ffffff\",\"shop_by_brand_show_footer\":true,\"shop_by_price_visible\":true,\"show_accept_amex\":false,\"show_accept_discover\":false,\"show_accept_mastercard\":false,\"show_accept_paypal\":false,\"show_accept_visa\":false,\"show_copyright_footer\":true,\"show_powered_by\":true,\"show_product_details_tabs\":true,\"show_product_dimensions\":false,\"show_product_quick_view\":true,\"show_product_weight\":true,\"social_icon_placement_bottom\":\"bottom_none\",\"social_icon_placement_top\":false,\"spinner-borderColor-dark\":\"#999999\",\"spinner-borderColor-light\":\"#ffffff\",\"storeName-color\":\"#333333\",\"swatch_option_size\":\"22x22\",\"thumb_size\":\"100x100\",\"zoom_size\":\"1280x1280\"},\"genericError\":\"Oops! Something went wrong.\",\"maintenanceMode\":{\"header\":null,\"message\":null,\"notice\":null,\"password\":null,\"securePath\":\"https://www.gobilda.com\"},\"urls\":{\"account\":{\"add_address\":\"/account.php?action=add_shipping_address\",\"addresses\":\"/account.php?action=address_book\",\"details\":\"/account.php?action=account_details\",\"inbox\":\"/account.php?action=inbox\",\"index\":\"/account.php\",\"orders\":{\"all\":\"/account.php?action=order_status\",\"completed\":\"/account.php?action=view_orders\",\"save_new_return\":\"/account.php?action=save_new_return\"},\"recent_items\":\"/account.php?action=recent_items\",\"returns\":\"/account.php?action=view_returns\",\"send_message\":\"/account.php?action=send_message\",\"update_action\":\"/account.php?action=update_account\",\"wishlists\":{\"add\":\"/wishlist.php?action=addwishlist\",\"all\":\"/wishlist.php\",\"delete\":\"/wishlist.php?action=deletewishlist\",\"edit\":\"/wishlist.php?action=editwishlist\"}},\"auth\":{\"check_login\":\"/login.php?action=check_login\",\"create_account\":\"/login.php?action=create_account\",\"forgot_password\":\"/login.php?action=reset_password\",\"login\":\"/login.php\",\"logout\":\"/login.php?action=logout\",\"save_new_account\":\"/login.php?action=save_new_account\",\"save_new_password\":\"/login.php?action=save_new_password\",\"send_password_email\":\"/login.php?action=send_password_email\"},\"brands\":\"https://www.gobilda.com/brands/\",\"cart\":\"/cart.php\",\"checkout\":{\"multiple_address\":\"/checkout.php?action=multiple\",\"single_address\":\"/checkout\"},\"compare\":\"/compare\",\"contact_us_submit\":\"/pages.php?action=sendContactForm\",\"gift_certificate\":{\"balance\":\"/giftcertificates.php?action=balance\",\"purchase\":\"/giftcertificates.php\",\"redeem\":\"/giftcertificates.php?action=redeem\"},\"home\":\"https://www.gobilda.com/\",\"product\":{\"post_review\":\"/postreview.php\"},\"rss\":{\"blog\":\"/rss.php?action=newblogs&type=rss\",\"blog_atom\":\"/rss.php?action=newblogs&type=atom\",\"products\":[]},\"search\":\"/search.php\",\"sitemap\":\"/sitemap.php\",\"subscribe\":{\"action\":\"/subscribe.php\"}}}").load();
					// </script>
					jstext = strings.ReplaceAll(jstext[pos+2:pos2], "\\\"", "\"")
					urlset := strings.Split(jstext, ",")
					for _, url := range urlset {
						pos3 := strings.Index(url, "\"url\":\"")
						if pos3 >= 0 {
							urlpart := url[pos3+7:]
							pos4 := strings.Index(urlpart, "\"")
							if pos4 > 0 {
								urlpart = urlpart[:pos4]
							}
							urlpart = strings.Trim(urlpart, "\"")
							found = true
							enqueURL(ctx, urlpart, breadcrumbs)
						}
					}
				}
			}
		}
	}
	return
}

// --------------------------------------------------------------------------------------------
// processProduct takes a standard page which has a single product on it and outputs the information
func processProduct(ctx *fetchbot.Context, productname string, url string, product *goquery.Selection) (found bool) {
	found = false
	outputCategory(productname, false)
	localname := product.Find(".productView-header h1.productView-title").Text()
	if localname == "" {
		localname, _ = product.Find("meta[itemprop=\"name\"]").Attr("content")
	}
	sku, hassku := product.Find("span.productView-sku[data-product-sku]").Attr("data-product-sku")
	if !hassku {
		sku, hassku = product.Find("meta[itemprop=\"sku\"]").Attr("content")
	}
	fmt.Printf("Process Product\n")
	changeset := product.Find("[data-product-option-change]")
	//  <div data-product-option-change="" style="">
	//    <div class="form-field" data-product-attribute="set-radio">
	//      <label class="form-label form-label--alternate form-label--inlineSmall">
	//         Cable Length:<small>Required</small>
	//      </label>
	//      <input class="form-radio" type="radio" id="attribute_radio_114" name="attribute[53]" value="114" required="" data-state="false">
	//      <label data-product-attribute-value="114" class="form-label" for="attribute_radio_114">30cm</label>
	//      <input class="form-radio" type="radio" id="attribute_radio_115" name="attribute[53]" value="115" checked="" data-default="" required="" data-state="true">
	//      <label data-product-attribute-value="115" class="form-label" for="attribute_radio_115">50cm</label>
	//      <input class="form-radio" type="radio" id="attribute_radio_144" name="attribute[53]" value="144" required="" data-state="false">
	//      <label data-product-attribute-value="144" class="form-label" for="attribute_radio_144">100cm</label>
	//    </div>
	//  </div>

	downloadurls := findAllDownloads(ctx, url, product)
	if hassku {
		if changeset.Children().Length() > 0 {
			fmt.Printf("Has Changeset\n")
			changeset.Find("input").Each(func(i int, input *goquery.Selection) {
				itemname := localname
				itemsku := sku
				// We have a pair like this:
				//      <input class="form-radio" type="radio" id="attribute_radio_114" name="attribute[53]" value="114" required="" data-state="false">
				//      <label data-product-attribute-value="114" class="form-label" for="attribute_radio_114">30cm</label>
				_, ischecked := input.Attr("checked")
				// Unfortunately we don't know how to recover the item SKU.  it comes from some external file that we didn't load
				if !ischecked {
					itemsku = sku[:7] + "????" // Take the REV-nn- portion of the SKU
				}
				// But we do need to find the item name
				val, hasval := input.Attr("value")
				if hasval {
					tofind := "[data-product-attribute-value=\"" + val + "\"]"
					label := changeset.Find(tofind)
					if label.Length() > 0 {
						itemname += " " + label.Text()
					}
				}
				outputProduct(itemname, itemsku, url, getDownloadURL(ctx, sku, downloadurls), nil)

			})
		} else {
			fmt.Printf("No Changeset\n")
			outputProduct(localname, sku, url, getDownloadURL(ctx, sku, downloadurls), nil)
		}
		found = true
	}
	showUnusedURLS(ctx, url, downloadurls)
	return
}

// --------------------------------------------------------------------------------------------
// ProcessTable parses a table of parts and outputs all of the products in the table
// The first step is
func processTable(ctx *fetchbot.Context, productname string, url string, downloadurls downloadentmap, table *goquery.Selection) (result bool) {
	type colact int
	const (
		actSKU colact = iota
		actSkip
		actKeepName
		actKeepNameAfter
		actKeepNameBefore
		actKeepBore
		actOutput
		actKeepTo
	)
	var specialmap = map[string]colact{
		"Part #":       actSKU,
		"Part Number":  actSKU,
		"SKU":          actSKU,
		"Meta Title":   actSKU,
		"Wishlist":     actSkip,
		"Price":        actSkip,
		"Purchase":     actSkip,
		"Length":       actKeepName,
		"Bore":         actKeepName,
		"A":            actKeepBore,
		"Tooth":        actKeepNameAfter,
		"Spline Size":  actKeepName,
		"Thread Size":  actKeepName,
		"Screw Size":   actKeepName,
		"Thread":       actKeepName,
		"Servo Spline": actKeepName,
		"Thickness":    actKeepName,
		"Bore A":       actKeepTo,
		"Bore B":       actKeepName,
		"Hex Size":     actKeepName,
		"# of teeth":   actKeepName,
	}
	type colent struct {
		name   string
		action colact
	}
	var colnames [64]colent
	result = false

	// First we need to figure out the columns in the table.  We need to know the SKU and all columns
	// except the Wishlist, Price and Purchase columns.  The remainder of the columns are useful to us
	// Note that sometimes the table has a Bore column and an A column.  When there is only an A column,
	// it is actually the Bore and as it turns out is the same as the Bore column, so we want to skip the
	// A column if the Bore was already found
	//
	// Unfortunately we also have some bad actors like:
	//      https://www.servocity.com/locking-washers and
	//      https://www.servocity.com/zinc-plated-oversized-washers
	// which doesn't have a TH, so we instead have to find the first TD and pretend it was a TH
	// fmt.Printf("Processing Table\n")
	foundbore := false
	thset := table.Find("thead tr th")
	if thset.Length() < 1 {
		thset = table.Find("tr:first-child td")
		fmt.Printf("Secondary set length=%d\n", thset.Length())
	}
	thset.Each(func(i int, th *goquery.Selection) {
		p := th.Find("p")
		strong := th.Find("strong")
		colname := ""
		if p.Length() > 0 {
			colname = p.Text()
		} else if strong.Length() > 0 {
			colname = strong.Text()
		} else {
			colname = th.Text()
		}
		action, found := specialmap[colname]
		// fmt.Printf("Found column header '%s' action=%d\n", colname, action)
		if !found {
			action = actOutput

		} else {
			if action == actSKU {
				result = true
			}
			if colname == "Bore" {
				foundbore = true
			} else if action == actKeepBore && foundbore {
				action = actSkip
			}
		}
		colnames[i] = colent{colname, action}
	})
	if result {
		table.Find("tbody tr").Each(func(i int, tr *goquery.Selection) {
			sku := ""
			var outpad []string
			outname := productname
			tr.Find("td").Each(func(i int, td *goquery.Selection) {
				column := ""
				p := td.Find("p")
				if p.Length() > 0 {
					column = p.Text()
				} else {
					column = td.Text()
				}
				switch colnames[i].action {
				case actSKU:
					sku = column
				case actSkip:
				case actKeepName:
					outname += " " + column
				case actKeepNameBefore:
					outname += " " + colnames[i].name + " " + column
				case actKeepNameAfter:
					outname += " " + column + " " + colnames[i].name
				case actKeepBore:
					outname += " " + column + " Bore"
				case actKeepTo:
					outname += " " + column + " To"
				case actOutput:
					outpad = append(outpad, colnames[i].name+":"+column)
				default:
				}
			})
			outputProduct(outname, sku, url, getDownloadURL(ctx, sku, downloadurls), outpad)
		})
	}
	return
}

func processJavascriptSelector(ctx *fetchbot.Context, breadcrumbs string, url string, product *goquery.Selection) (found bool) {
	type SingleOption struct {
		ID       string
		Label    string
		Price    string
		OldPrice string
		Products []string
	}
	type AttributesConfig struct {
		ID      string
		Code    string
		Label   string
		Options []SingleOption
		// Options interface{} // []SingleOption
	}
	type ProductConfig struct {
		// Attributes map[string]interface{} `json:"attributes"`
		Attributes map[string]AttributesConfig `json:"attributes"`
		Template   string                      `json:"template"`
		BasePrice  string                      `json:"basePrice"`
		OldPrice   string                      `json:"oldPrice"`
		ProductID  string                      `json:"productId"`
		ChooseText string                      `json:"chooseText"`
		TaxConfig  string                      `json:"taxConfig"`
	}
	type ProductInfo struct {
		Label string
		SKU   string
	}
	found = false
	downloadurls := findAllDownloads(ctx, url, product)
	ProductMap := map[string]ProductInfo{}
	productname := ""
	pn := product.Find("div.product-name h1")
	if pn.Length() > 0 {
		productname = pn.Text()
	}
	product.Find("script").Each(func(i int, js *goquery.Selection) {
		jstext := js.Text()
		pos := strings.Index(jstext, "Product.Config(")
		if pos > 0 {
			pos2 := strings.Index(jstext, ");")
			// fmt.Printf("Found Javascript pos=%d pos2=%d '%s'\n", pos, pos2, jstext)
			if pos2 > 0 {

				//                  "371":{"id":"371",
				//                         "code":"length_configurable",
				//                         "label":"Length",
				//                         "options":[{"id":"244","label":"4mm","price":"0","oldPrice":"0","products":["6452"]},
				//                                    {"id":"245","label":"5mm","price":"0","oldPrice":"0","products":["6453"]}
				//                                   ]
				var result ProductConfig
				jsontext := jstext[pos+15 : pos2]
				fmt.Printf("JSON ='%s'\n", jsontext)
				json.Unmarshal([]byte(jsontext), &result)
				fmt.Printf("Parse Result:%s\n", result)
				fmt.Printf("Parse Result:%s\n", result.Attributes)
				for key, value := range result.Attributes {
					for key1, value1 := range value.Options {
						fmt.Printf("Key:%s[%d] ID:%s Products:%s\n", key, key1, value1.ID, value1.Products)
						for _, value2 := range value1.Products {
							label := ""
							oldmap, exists := ProductMap[value2]
							oldlabel := ""
							extra := ""
							if exists {
								oldlabel = oldmap.Label
								extra = " "
							}
							if value1.Label == "Yes" {
								label = value.Label
							} else if value1.Label == "No" {
								extra = ""
							} else {
								label = value.Label + " " + value1.Label
							}
							ProductMap[value2] = ProductInfo{oldlabel + extra + label, ""}
						}
					}
				}
			}
		} else {
			// We want to parse the ProductMap which should look something like
			// var productMap = {"6452":"92029A140","6453":"92029A141"};
			pos = strings.Index(jstext, "var productMap = ")
			if pos >= 0 {
				pos2 := strings.Index(jstext, "};")
				if pos2 >= 0 {
					// Extract just the value to assign
					pmtext := jstext[pos+17 : pos2+1]
					fmt.Printf("***pmtext:%s\n", pmtext)
					var result map[string]string
					// And pull it into a map
					json.Unmarshal([]byte(pmtext), &result)
					// Which we iterate through
					for key, value := range result {
						// And assign to the products
						Label := ProductMap[key].Label
						ProductMap[key] = ProductInfo{Label, value}
					}

				}
			}
			// fmt.Printf("Javascript: %s\n", jstext)
		}
	})
	// Ok we got the data.  Dump it out for now
	for _, value := range ProductMap {
		outputProduct(productname+" "+value.Label, value.SKU, url, getDownloadURL(ctx, value.SKU, downloadurls), nil)
		found = true
	}
	return
}

func processSimpleProductTable(ctx *fetchbot.Context, breadcrumbs string, url string, productname string, root *goquery.Selection, table *goquery.Selection) (found bool) {
	found = false
	downloadurls := findAllDownloads(ctx, url, root)
	outputCategory(breadcrumbs, false)
	table.Each(func(i int, subtable *goquery.Selection) {
		if processTable(ctx, productname, url, downloadurls, subtable) {
			found = true
		}
	})
	if found {
		showUnusedURLS(ctx, url, downloadurls)
	}
	return
}

// enqueueLinks parses a page and adds links to elements found within by the various processors
func enqueueLinks(ctx *fetchbot.Context, doc *goquery.Document) {
	mu.Lock()
	url := doc.Url.String()
	found := false
	breadcrumbs := getBreadCrumbName(ctx, url, doc.Find("ul.breadcrumbs"))
	fmt.Printf("Breadcrumb:%s\n", breadcrumbs)
	doc.Find("ul.navList").Each(func(i int, categoryproducts *goquery.Selection) {
		fmt.Printf("Found Navlist\n")
		if processSubCategory(ctx, breadcrumbs, categoryproducts) {
			found = true
		}
	})
	if !found {
		fmt.Printf("Looking for productGrid\n")
		doc.Find("ul.productGrid,ul.threeColumnProductGrid,div.productTableWrapper").Each(func(i int, product *goquery.Selection) {
			fmt.Printf("ProcessingProductGrid\n")
			if processProductGrid(ctx, breadcrumbs, url, product) {
				found = true
				// } else if processProductViewWithTable(ctx, breadcrumbs, url, product) {
				// 	found = true
			}
		})
	}
	if !found {
		doc.Find("div[itemtype=\"http://schema.org/Product\"]").Each(func(i int, product *goquery.Selection) {
			if processProduct(ctx, breadcrumbs, url, product) {
				found = true
			}
		})
	}
	doc.Find("script").Each(func(i int, product *goquery.Selection) {
		if processLazyLoad(ctx, breadcrumbs, url, product) {
			found = true
		}
	})
	if !found {
		doc.Find("table.productTable").Each(func(i int, product *goquery.Selection) {
			if processProductTableList(ctx, breadcrumbs, url, product) {
				found = true
			}
		})
	}

	// Title is div.page-title h1
	// Table is div.category-description div.table-widget-container table
	if !found {
		title := doc.Find("div.page-title h1")
		table := doc.Find("div.category-description div.table-widget-container table")
		if title.Length() > 0 && table.Length() > 0 &&
			processSimpleProductTable(ctx, breadcrumbs, url, title.Text(), doc.Children(), table) {
			found = true
		}
	}
	// Look for any related products to add to the list
	doc.Find("div.product-related a[data-card-type]").Each(func(i int, a *goquery.Selection) {
		urlloc, _ := a.Attr("href")
		product, _ := a.Attr("title")
		fmt.Printf("**Related Found item name=%s url=%s\n", product, urlloc)
		enqueURL(ctx, urlloc, makeBreadCrumb(breadcrumbs, product))
	})
	if !found {
		outputError("Unable to process: %s\n", url)
	}
	mu.Unlock()
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func checkError(err error) {
	if err != nil {
		panic(err.Error())
	}
}

// LoadStatusSpreadsheet -
// Get Part# and URL from gobilda ALL spreadsheet:
// https://docs.google.com/spreadsheets/d/15XT3v9O0VOmyxqXrgR8tWDyb_CRLQT5-xPfWPdbx4RM/edit
func LoadStatusSpreadsheet(spreadsheetID string) *ReferenceData {

	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := sheets.New(client)
	checkError(err)

	readRange := "All"
	response, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	checkError(err)

	if len(response.Values) == 0 {
		fmt.Println("No data in spreadsheet !")
	}

	var referenceData = new(ReferenceData)
	referenceData.partNumber = make(map[string]*PartData)
	referenceData.url = make(map[string]*PartData)
	referenceData.partdata = make([]*PartData, len(response.Values))

	for ii, cols := range response.Values {
		if ii == 0 {
			continue // header row
		}

		partdata := new(PartData)
		for jj, col := range cols {
			switch {
			case jj == orderColumnIndex:
				value, err := strconv.ParseUint(col.(string), 0, 32)
				checkError(err)
				partdata.Order = uint(value)
			case jj == sectionColumnIndex:
				partdata.Section = col.(string)
			case jj == nameColumnIndex:
				partdata.Name = col.(string)
			case jj == skuColumnIndex:
				partdata.SKU = col.(string)
			case jj == urlColumnIndex:
				partdata.URL = col.(string)
			case jj == modelURLColumnIndex:
				partdata.ModelURL = col.(string)
			case jj == onShapeURLColumnIndex:
				partdata.OnshapeURL = col.(string)
			case jj >= extraColumnIndex && jj <= extraColumnIndex+6:
				partdata.Extra[jj-extraColumnIndex] = col.(string)
			case jj == statusColumnIndex:
				partdata.Status = col.(string)
			case jj == notesColumnIndex:
				partdata.Notes = col.(string)
			default:
			}
			partdata.SpiderStatus = "Not Found by Spider"
			referenceData.partdata[ii] = partdata
		}
		if excludeFromMatch(partdata) {
			continue
		}
		dup, ok := referenceData.partNumber[partdata.SKU]
		if ok {
			fmt.Printf("row %d: duplicate part number '%s' found (original row %d)\n", ii, partdata.SKU, dup.Order)
		} else {
			referenceData.partNumber[partdata.SKU] = partdata
		}

		referenceData.url[partdata.URL] = partdata

	}

	return referenceData
}

func excludeFromMatch(partdata *PartData) bool {
	if strings.HasPrefix(partdata.Name, "--") {
		return true
	}
	if strings.HasPrefix(partdata.SKU, "(Configurable)") {
		return true
	}
	if strings.HasPrefix(partdata.SKU, "(??") {
		return true
	}
	return false
}
