package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
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

var ()

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

type spiderTarget struct {
	outfile        string
	spreadsheetID  string
	presets        []string
	seed           string
	parsePageFunc  func(ctx *fetchbot.Context, doc *goquery.Document)
	checkMatchFunc func(partData *PartData)
}

var (
	targets = map[string]spiderTarget{
		"": {
			"file.txt",
			"",
			[]string{},
			"",
			nilParsePage,
			nilCheckMatch,
		},
		"Rev": {
			"rev_robotics.txt",
			"19Mc9Uj0zoaRr_KmPncf_svNOp9WqIgrzaD7fEiNlBr0",
			[]string{},
			"",
			nilParsePage,
			nilCheckMatch,
		},
		"servocity": servocityTarget,
		"gobilda":   gobildaTarget,
		"andymark": {
			"andymark.txt",
			"1x4SUwNaQ_X687yA6kxPELoe7ZpoCKnnCq1-OsgxUCOw",
			[]string{},
			"",
			nilParsePage,
			nilCheckMatch,
		},
		"pitsco": {
			"pitsco.txt",
			"1adykd3BVYUyXsb3vC2A-lNhFNj_Q8Yzd1oXThmSwPio",
			[]string{},
			"",
			nilParsePage,
			nilCheckMatch,
		},
	}
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
	targetConfig spiderTarget

	// Command-line flags
	target        = flag.String("target", "servocity", "Target vendor to spider")
	seed          = flag.String("seed", "", "seed URL")
	cancelAfter   = flag.Duration("cancelafter", 0, "automatically cancel the fetchbot after a given time")
	cancelAtURL   = flag.String("cancelat", "", "automatically cancel the fetchbot at a given URL")
	stopAfter     = flag.Duration("stopafter", 0, "automatically stop the fetchbot after a given time")
	stopAtURL     = flag.String("stopat", "", "automatically stop the fetchbot at a given URL")
	memStats      = flag.Duration("memstats", 0, "display memory statistics at a given interval")
	fileout       = flag.String("out", "", "Output File")
	spreadsheetID = flag.String("spreadsheet", "", "spider this spreadsheet")

	outfile *os.File
)

func main() {
	flag.Parse()

	present := false
	targetConfig, present = targets[*target]
	if !present {
		targetConfig = targets[""]
	}

	// See if we have to fill in any defaults
	if len(*seed) == 0 {
		*seed = targetConfig.seed
	}
	if len(*fileout) == 0 {
		*fileout = targetConfig.outfile
	}
	if len(*spreadsheetID) == 0 {
		*spreadsheetID = targetConfig.spreadsheetID
	}

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

	referenceData, err = LoadStatusSpreadsheet(spreadsheetID)
	if err != nil {
		fmt.Printf("%v\n", err)
	}

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
			targetConfig.parsePageFunc(ctx, doc)
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

	for _, val := range targetConfig.presets {
		preloadQueueURL(q, val, "Initial")
	}
	preloadQueueURL(q, *seed, "Home > Competition > FTC")

	// Pre-queue any of the URLs that we had already found
	for _, entry := range referenceData.partNumber {
		preloadQueueURL(q, entry.URL, entry.Section)
	}

	q.Block()

	for _, entry := range referenceData.partNumber {
		outputPartData(entry)
	}
}

func preloadQueueURL(q *fetchbot.Queue, URL string, breadcrumb string) {
	URL, _ = cleanURL(URL)
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
		toadd = strings.ReplaceAll(toadd, "\u00A0", " ")

		if result != "" {
			result += " > "
		}
		result += toadd
	}
	return
}

// strupURLSku removes any selector from a URL returning the cleaned string and an indication that it was removed
func cleanURL(url string) (result string, stripped bool) {
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

func enqueURL(ctx *fetchbot.Context, url string, breadcrumb string) {
	// Resolve address
	fmt.Printf("+++Enqueue:%s\n", url)
	u, err := ctx.Cmd.URL().Parse(url)
	if err != nil {
		fmt.Printf("error: resolve URL %s - %s\n", url, err)
		return
	}
	// Trim off any sku= on the URL
	urlString, _ := cleanURL(u.String())
	_, found := bcmap[urlString]
	if !found {
		if _, err := ctx.Q.SendStringHead(urlString); err != nil {
			fmt.Printf("error: enqueue head %s - %s\n", u, err)
		} else {
			bcmap[urlString] = breadcrumb
		}
	}
}

// markVisitedURL allows us to mark a page which has been received as part of a 301 redirect.
// It prevents us from visiting a page twice (in theory)
func markVisitedURL(ctx *fetchbot.Context, url string, breadcrumb string) {
	u, err := ctx.Cmd.URL().Parse(url)
	if err != nil {
		fmt.Printf("error: resolve URL %s - %s\n", url, err)
		return
	}
	_, found := bcmap[u.String()]
	if !found {
		bcmap[u.String()] = breadcrumb
	}
}

// The output routines write the messages in two places.
//  First it puts a status on stdout so that the you can see what is happening
//  It also puts in lines in the output file so that it can be pulled into a spreadsheet
//  Note that the lines are numbered with columns separated by a backtick because sometimes
//  we may see tabs in the names

// --------------------------------------------------------------------------------------------
// outputHeader generates the first line of the output file with the column headers
// Note that we use ` to separate columns because we sometimes see tabs in the names
func outputHeader() {
	fmt.Fprintf(outfile, "%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v\n",
		"Order", "Section", "Name", "Part #", "Combined Name", "URL", "Model URL", "Extra 1", "Extra 2", "Extra 3", "Extra 4", "Extra 5", "Extra 6", "Extra 7", "Onshape URL", "Model Status", "Spider Status", "Notes")
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
		lastcategory = category
	}
}

// outputProduct takes the spidered information and generates the output structure
func outputProduct(name string, sku string, url string, modelURL string, isDiscontinued bool, extra []string) {
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
	partData.Order = uint(linenum)
	linenum++

	targetConfig.checkMatchFunc(&partData)

	if isDiscontinued {
		partData.SpiderStatus = "Discontinued"
	}
	outputPartData(&partData)
}

// --------------------------------------------------------------------------------------------
// outputPartData generates the product line for the output file and also prints a status message on stdout
func outputPartData(partData *PartData) {

	fmt.Printf("%s |SKU: '%v' Product: '%v' Model:'%v' on page '%v'\n", partData.SpiderStatus, partData.SKU, partData.Name, partData.ModelURL, partData.URL)

	fmt.Fprintf(outfile, "%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v`%v\n",
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

// --------------------------------------------------------------------------------------------
// outputError generates an error line in the output file (typically a missing download) and
// also prints the status message on stdout
func outputError(message string, args ...interface{}) {
	fmt.Printf("***"+message, args...)
	outmsg := fmt.Sprintf("%d`***", linenum) + message
	fmt.Fprint(outfile, fmt.Sprintf(outmsg, args...))
	linenum++
}
func excludeFromMatch(partdata *PartData) bool {
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
		partdata.Order = uint(linenum)
		partdata.SpiderStatus = "Same"
		outputPartData(partdata)
		linenum++
	}
	return exclude
}

func nilParsePage(ctx *fetchbot.Context, doc *goquery.Document) {}
func nilCheckMatch(partData *PartData)                          {}
