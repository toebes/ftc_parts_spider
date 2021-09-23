package andymark

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/toebes/ftc_parts_spider/partcatalog"
	"github.com/toebes/ftc_parts_spider/spiderdata"
)

// AndyMarkTarget is the configuration structure for spidering the AndyMark website
var AndyMarkTarget = spiderdata.SpiderTarget{
	Outfile:            "andymark.txt",
	SpreadsheetID:      "15XT3v9O0VOmyxqXrgR8tWDyb_CRLQT5-xPfWPdbx4RM",
	Presets:            []string{},
	Seed:               "https://www.andymark.com/structure/",
	ParsePageFunc:      andymark.ParseAndyMarkPage,
	CheckMatchFunc:     andymark.CheckAndyMarkMatch,
	SectionNameDeletes: []string{},
	SectionAllowedMap:  map[string]string{},
	SectionEquivalents: [][]string{},
}

// CheckAndyMarkMatch compares a partData to what has been captured from the spreadsheet
// Any differences are put into the notes
func CheckAndyMarkMatch(ctx *spiderdata.Context, partData *partcatalog.PartData) {
	entry, found := ctx.G.ReferenceData.PartNumber[partData.SKU]
	if !found {
		entry, found = ctx.G.ReferenceData.URL[partData.URL]
	}
	if found {
		// We matched a previous entry

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
		partData.SpiderStatus = partcatalog.UnchangedPart
		// If it was in a different path (the part moved on the website) Then we want to
		// keep the old section and record a message for the new section
		// Note that it may not have moved, but we chose to organize it slightly different
		// A good example of this is hubs which are grouped by hub type
		if !strings.EqualFold(partData.Section, entry.Section) && partData.Section != "" {
			// See if this is really a section change. Some things need to be modified before we accept it
			// First all non-breaking spaces are turned to regular spaces
			newsection := strings.ReplaceAll(partData.Section, "\u00A0", " ")
			oldsection := strings.ReplaceAll(entry.Section, "\u00A0", " ")
			// Then we delete some known patterns
			for _, deleteStr := range ctx.G.TargetConfig.SectionNameDeletes {
				newsection = strings.ReplaceAll(newsection, deleteStr, "")
			}
			//  Then strip leading/trailing blanks
			newsection = strings.TrimSpace(newsection)

			// Look for any equivalent mappings so that we end up trusting what is already in the spreadsheet.
			propersection, matched := ctx.G.TargetConfig.SectionAllowedMap[entry.SKU]
			if len(oldsection) > len(newsection) && strings.EqualFold(newsection, oldsection[:len(newsection)]) {
				newsection = oldsection
			}
			// Lastly anything which is deemed to be equivalent we will let through
			for _, strset := range ctx.G.TargetConfig.SectionEquivalents {
				if len(strset) == 2 {
					if strings.HasPrefix(strings.ToUpper(newsection), strings.ToUpper(strset[0])) &&
						strings.HasPrefix(strings.ToUpper(oldsection), strings.ToUpper(strset[1])) {
						newsection = oldsection
						break
					}
				}
			}

			// if it now matches then we want to use the OLD section silently
			// Also if it is one of the known special cases we also let it use the old section
			if strings.EqualFold(newsection, oldsection) || (matched && strings.EqualFold(propersection, oldsection)) {
				partData.Section = entry.Section
			} else {
				partData.SpiderStatus = partcatalog.PartChanged
				partData.Notes += extra + "New Section:" + newsection
				partData.Section = entry.Section
				extra = separator
			}
		}
		// Likewise if the name changed, we want to still use the old one.  This is because
		// Often the website name has something like (2 pack) or a plural that we want to make singular
		if !strings.EqualFold(partData.Name, entry.Name) {
			newName := strings.ReplaceAll(partData.Name, "\u00A0", " ")
			newName = strings.ReplaceAll(newName, "  ", " ")
			oldName := strings.ReplaceAll(entry.Name, "  ", " ")
			// Name changes
			// Get rid of any <n> Pack in the name
			var re = regexp.MustCompile("[\\- \\(]*[0-9]+ [pP]ack *\\)*")
			newName = re.ReplaceAllString(newName, "")

			oldName = strings.ReplaceAll(oldName, "(Pair)", "")
			oldName = strings.ReplaceAll(oldName, "[DISCONTINUED]", "")
			oldName = strings.ReplaceAll(oldName, "[OBSOLETE]", "")

			oldName = strings.TrimSpace(oldName)
			newName = strings.TrimSpace(newName)

			if strings.EqualFold(oldName, newName) {
				partData.Name = newName
			} else {
				// Eliminate double spaces
				partData.SpiderStatus = partcatalog.PartChanged
				partData.Notes += extra + "New Name:" + newName
				partData.Name = oldName
				extra = separator
			}
		}
		// If the SKU changes then we really want to know it.  We should use the new SKU
		// and stash away the old SKU but it needs to be updated
		if !strings.EqualFold(partData.SKU, entry.SKU) {
			partData.SpiderStatus = partcatalog.PartChanged
			partData.Notes += extra + " Old SKU:" + entry.SKU
			extra = separator
		}
		// If the URL changes then we really want to use it.
		// Just stash away the old URL so we know what happened
		if !strings.EqualFold(partData.URL, entry.URL) {
			// In the case where there was a sku= on the URL we want to keep the one with it
			urlString := partData.URL
			newURL, strippedNew := spiderdata.CleanURL(partData.URL)
			oldURL, strippedOld := spiderdata.CleanURL(entry.URL)
			if !strippedNew && strippedOld {
				urlString = entry.URL
			}
			// If they matched without the URL on it, then we want to take the one that
			// had the URL silently.
			if strings.EqualFold(oldURL, newURL) {
				partData.URL = urlString
			} else {
				partData.SpiderStatus = partcatalog.PartChanged
				partData.Notes += extra + " Old URL:" + entry.URL
				extra = separator
			}
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
		partData.SpiderStatus = partcatalog.NewPart
		partData.Status = "Not Done"
	}

}

// ParseAndyMarkPage parses a page and adds links to elements found within by the various processors
func ParseAndyMarkPage(ctx *spiderdata.Context, doc *goquery.Document) {
	ctx.G.Mu.Lock()
	url := doc.Url.String()
	found := false
	breadcrumbs := getBreadCrumbName(ctx, url, doc.Find("ul.breadcrumbs"))
	spiderdata.MarkVisitedURL(ctx, url, breadcrumbs)

	// see if this has been discontinued
	isDiscontinued := (doc.Find("p.discontinued").Length() > 0)

	fmt.Printf("Breadcrumb:%s\n", breadcrumbs)
	doc.Find("ul.navPages-list").Each(func(i int, categoryproducts *goquery.Selection) {
		fmt.Printf("Found Navigation List\n")
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
		products := doc.Find("div[itemtype=\"http://schema.org/Product\"]")
		products.Each(func(i int, product *goquery.Selection) {
			if processProduct(ctx, breadcrumbs, url, product, isDiscontinued, products.Length() > 1) {
				found = true
			}
		})
	}
	if !found {
		hasOptions := doc.Find("div.available section.productView-children")
		if hasOptions.Length() > 0 {
			products := doc.Find("header.productView-header")

			products.Each(func(i int, product *goquery.Selection) {
				if processProduct(ctx, breadcrumbs, url, product.Parent(), isDiscontinued, products.Length() > 1) {
					found = true
				}
			})
		}
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
		spiderdata.EnqueURL(ctx, urlloc, spiderdata.MakeBreadCrumb(ctx, breadcrumbs, product))
	})
	if !found {
		// See if they have a meta refresh request for a page which is a redirect
		doc.Find("meta[http-equiv=refresh]").Each(func(i int, meta *goquery.Selection) {
			content, _ := meta.Attr("content")
			pos := strings.Index(content, ";url=")
			if pos >= 0 {
				redirectURL := content[pos+5:]
				spiderdata.EnqueURL(ctx, redirectURL, breadcrumbs)
				found = true
			}
		})
	}
	if !found {
		spiderdata.OutputError(ctx, "Unable to process: %s\n", url)
	}
	ctx.G.Mu.Unlock()
}
