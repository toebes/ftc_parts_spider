package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"

	"github.com/toebes/ftc_parts_spider/andymark"
	"github.com/toebes/ftc_parts_spider/gobilda"
	"github.com/toebes/ftc_parts_spider/partcatalog"
	"github.com/toebes/ftc_parts_spider/revrobotics"
	"github.com/toebes/ftc_parts_spider/servocity"
	"github.com/toebes/ftc_parts_spider/spiderdata"
	"github.com/toebes/ftc_parts_spider/studica"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
)

var ()

var (
	targets = map[string]*spiderdata.SpiderTarget{
		"": {
			Outfile:        "file.txt",
			SpreadsheetID:  "",
			Presets:        []string{},
			Seed:           "",
			StripSKU:       false,
			ParsePageFunc:  spiderdata.NilParsePage,
			CheckMatchFunc: spiderdata.NilCheckMatch,
		},
		"rev":       &revrobotics.RevRoboticsTarget,
		"servocity": &servocity.ServocityTarget,
		"gobilda":   &gobilda.GobildaTarget,
		"andymark":  &andymark.AndyMarkTarget,
		"studica":   &studica.StudicaTarget,
		"pitsco": {
			Outfile:        "pitsco.txt",
			SpreadsheetID:  "1adykd3BVYUyXsb3vC2A-lNhFNj_Q8Yzd1oXThmSwPio",
			Presets:        []string{},
			Seed:           "",
			StripSKU:       false,
			ParsePageFunc:  spiderdata.NilParsePage,
			CheckMatchFunc: spiderdata.NilCheckMatch,
		},
	}

	// Command-line flags
	target        = flag.String("target", "rev", "Target vendor to spider")
	seed          = flag.String("seed", "", "seed URL")
	cancelAfter   = flag.Duration("cancelafter", 0, "automatically cancel the fetchbot after a given time")
	cancelAtURL   = flag.String("cancelat", "", "automatically cancel the fetchbot at a given URL")
	stopAfter     = flag.Duration("stopafter", 0, "automatically stop the fetchbot after a given time")
	stopAtURL     = flag.String("stopat", "", "automatically stop the fetchbot at a given URL")
	memStats      = flag.Duration("memstats", 0, "display memory statistics at a given interval")
	fileout       = flag.String("out", "", "Output File")
	spreadsheetID = flag.String("spreadsheet", "", "spider this spreadsheet")
	singleOnly    = flag.Bool("single", false, "Only process the seed and don't follow any additional links")
	StripSKU      = flag.Bool("stripsku", false, "Strip the SKU and other parameters from URLs")
	SkipCatalog   = flag.Bool("skipcatalog", false, "Skip loading the catalog")
)

type userAgentTransport struct {
}

func (uat *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "FTCPartsSpider/1.0.0")
	req.Header.Set("Host", "ftconshape.com")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept-Encoding", "")
	req.Header.Set("Connection", "keep-alive")
	return http.DefaultTransport.RoundTrip(req)
}

// ExcludeFromMatch checks to see whether something should be spidered
func ExcludeFromMatch(partdata *partcatalog.PartData) bool {
	exclude := false
	if strings.HasPrefix(partdata.Name, "--") {
		exclude = true
	}
	if strings.HasPrefix(partdata.SKU, "(Configurable)") {
		exclude = true
	}
	if strings.HasPrefix(partdata.SKU, "(No Part Number)") {
		exclude = true
	}
	return exclude
}

func main() {
	flag.Parse()

	context := spiderdata.Context{}
	context.G = &spiderdata.Globals{}
	context.G.BreadcrumbMap = make(map[string]string)
	context.G.CatMap = make(spiderdata.CategoryMap)
	context.G.DownloadMap = make(spiderdata.DownloadEntMap)
	context.G.SingleOnly = *singleOnly
	context.G.StripSKU = *StripSKU

	present := false
	context.G.TargetConfig, present = targets[*target]
	if !present {
		context.G.TargetConfig = targets[""]
	}

	// See if we have to fill in any defaults
	if len(*seed) == 0 {
		*seed = context.G.TargetConfig.Seed
	}
	if len(*fileout) == 0 {
		*fileout = context.G.TargetConfig.Outfile
	}
	if len(*spreadsheetID) == 0 {
		*spreadsheetID = context.G.TargetConfig.SpreadsheetID
	}

	if context.G.TargetConfig.StripSKU {
		context.G.StripSKU = context.G.TargetConfig.StripSKU
	}

	// Parse the provided seed
	u, err := url.Parse(*seed)
	if err != nil {
		log.Fatal(err)
	}

	// Mark all the pages that we want to automatically skip
	for _, url := range context.G.TargetConfig.SkipPages {
		spiderdata.MarkVisitedURL(&context, url, "")
	}

	// Start our log file to import into Excel
	context.G.Outfile, err = os.Create(*fileout)
	if err != nil {
		log.Fatal(err)
	}
	spiderdata.OutputHeader(&context)

	///
	if *SkipCatalog {
		context.G.ReferenceData = partcatalog.NewPartCatalogData()
		context.G.ReferenceData.Partdata = make([]*partcatalog.PartData, 0)
	} else {
		context.G.ReferenceData, err = partcatalog.LoadPartCatalog(spreadsheetID, ExcludeFromMatch)
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}

	if context.G.ReferenceData != nil {
		for _, partdata := range context.G.ReferenceData.ExcludeFromSearch {
			partdata.SpiderStatus = partcatalog.UnchangedPart
			spiderdata.OutputPartData(&context, partdata)
		}
	}
	context.Qc = &spiderdata.QueueCounter{}

	// Initialize a custom HTTP client with a User-Agent
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	client := &http.Client{
		Transport: &userAgentTransport{},
		Jar:       jar}

	// Create the muxer
	mux := fetchbot.NewMux()

	// Handle all errors the same
	mux.HandleErrors(fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
	}))

	// Handle GET requests for html responses, to parse the body and enqueue all links as HEAD
	// requests.
	getHandler := fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			// Display cookies received for the current URL
			// cookies := client.Jar.Cookies(res.Request.URL)
			// for _, cookie := range cookies {
			// 	log.Printf("Cookie: %s = %s\n", cookie.Name, cookie.Value)
			// }
			context.Qc.Decrement()
			if err != nil {
				fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
				return
			}
			// Process the body to find the links
			defer res.Body.Close()
			if res.StatusCode == 404 {

			} else {
				doc, err := goquery.NewDocumentFromReader(res.Body)
				if err != nil {
					fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
					return
				}
				url := res.Request.URL.String()
				wasseen := false
				original := ctx.Cmd.URL().String()
				if url != original {
					// We got a redirect.  See if the finalURL was also on the list
					_, wasseen = context.G.BreadcrumbMap[url]
				}

				muxcontext := spiderdata.Context{Cmd: ctx.Cmd, Q: ctx.Q, G: context.G, Url: url, Qc: context.Qc}
				// Enqueue all links as HEAD requests
				if !wasseen {
					context.G.TargetConfig.ParsePageFunc(&muxcontext, doc)
				}
				// See how many are remaining
				remain := context.Qc.GetPendingCount()
				fmt.Printf("#### After Processing %v remain\n", remain)
				// Note that when we get down to processing the last one, we want to
				// queue in all the entries which were in the loaded list
				if remain == 1 && !context.G.SingleOnly {
					for _, entry := range context.G.ReferenceData.PartNumber {
						spiderdata.EnqueURL(&context, entry.URL, entry.Section)
					}
				}
			}
		})
	mux.Response().Method("GET").ContentType("text/html").Handler(getHandler)
	mux.Response().Method("GET").ContentType("application/xml").Handler(getHandler)

	// Handle HEAD requests for html responses coming from the source host - we don't want
	// to crawl links from other hosts.
	headhandler := fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			if _, err := ctx.Q.SendStringGet(ctx.Cmd.URL().String()); err != nil {
				fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
			}
		})

	mux.Response().Method("HEAD").Host(u.Host).ContentType("application/xml").Handler(headhandler)
	mux.Response().Method("HEAD").Host(u.Host).ContentType("text/html").Handler(headhandler)

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
	f.HttpClient = client

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
	context.Q = q

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
	spiderdata.EnqueURL(&context, *seed, "Home > Competition > FTC")

	if !context.G.SingleOnly {
		for _, val := range context.G.TargetConfig.Presets {
			spiderdata.EnqueURL(&context, val, "Initial")
		}
	} else {
		fmt.Print("*** -single option selected, no additional URLs will be spidered")
	}

	q.Block()

	for _, entry := range context.G.ReferenceData.PartNumber {
		if entry.SpiderStatus == partcatalog.PartNotFoundBySpider {
			spiderdata.OutputPartData(&context, entry)
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
