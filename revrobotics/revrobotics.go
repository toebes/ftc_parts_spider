package revrobotics

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/toebes/ftc_parts_spider/partcatalog"
	"github.com/toebes/ftc_parts_spider/spiderdata"

	"github.com/PuerkitoBio/goquery"
)

// RevRoboticsTarget is the configuration structure for spidering the Rev Robotics website
var RevRoboticsTarget = spiderdata.SpiderTarget{
	Outfile:        "rev_robotics.txt",
	SpreadsheetID:  "1Rs6HgY-WZzOyxMhI53NjcfcuxjO-hh3gsDmO4Jk6PFA", //"19Mc9Uj0zoaRr_KmPncf_svNOp9WqIgrzaD7fEiNlBr0",
	Presets:        []string{},
	StripSKU:       false,
	Seed:           "https://www.revrobotics.com/ftc/",
	ParsePageFunc:  ParseRevRoboticsPage,
	CheckMatchFunc: CheckRevRoboticsMatch,

	SectionNameDeletes: []string{},
	SectionAllowedMap:  map[string]string{},
	SectionEquivalents: [][]string{
		// {"MOTION > Bearings", "MOTION > Linear Bearings"},
		// {"KITS > FTC Kits", "KITS > Linear Motion Kits"},
		// {"MOTION > Couplers > Shop by Coupler Bore", "MOTION > Couplers > "},
		// {"ELECTRONICS > Wiring > Connector Style", "ELECTRONICS > Wiring > "},
		// {"MOTION > Hubs > Servo Hubs", "MOTION > Servos & Accessories > Servo Hubs"},
		// {"MOTION > Servos & Accessories > Servos", "MOTION > Servos & Accessories"},
		// {"STRUCTURE > Adaptors", "MOTION > Hubs"},
		// {"STRUCTURE > Brackets", "STRUCTURE > X-Rail® > X-Rail® Accessories"},
	},
}

// These products have a selector for color (or other attribute)
var EquivSKUs = map[string]map[string]string{
	"15mm Extrusion Slot Cover - 2m": {
		"Red":    "REV-41-1633",
		"White":  "REV-41-1634",
		"Blue":   "REV-41-1635",
		"Yellow": "REV-41-1636",
		"Green":  "REV-41-1637",
		"Black":  "REV-41-1638",
		"Orange": "REV-41-1639",
	}}

// These products have a selector for options, but are all the same SKU
var SingleSKUs = map[string]struct{}{
	"REV-11-1105": {},
	"REV-31-1389": {},
	"REV-31-1557": {},
	"REV-45-1507": {},
}

// CheckRevRoboticsMatch compares a partData to what has been captured from the spreadsheet
// Any differences are put into the notes
func CheckRevRoboticsMatch(ctx *spiderdata.Context, partData *partcatalog.PartData) {

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
			var re = regexp.MustCompile(`[\- \(]*[0-9]+ [pP]ack *\)*`)
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
			newURL, strippedNew := spiderdata.CleanURL(ctx, partData.URL)
			oldURL, strippedOld := spiderdata.CleanURL(ctx, entry.URL)
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
				// extra = separator
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
		// Prevent us from outputting the same entry more than once.
		entry.SpiderStatus = partData.SpiderStatus
	} else {
		partData.SpiderStatus = partcatalog.NewPart
		partData.Status = "Not Done"
	}

}

// getBreadCrumbName returns the breadcrumb associated with a document
// A typical one looks like this:
//
//	<div class="breadcrumbs">
//	<ul>
//	                <li class="home">
//	                        <a href="https://www.servocity.com/" title="Go to Home Page">Home</a>
//	                                    <span>&gt; </span>
//	                    </li>
//	                <li class="category30">
//	                        <a href="https://www.servocity.com/motion-components" title="">Motion Components</a>
//	                                    <span>&gt; </span>
//	                    </li>
//	                <li class="category44">
//	                        <a href="https://www.servocity.com/motion-components/linear-motion" title="">Linear Motion</a>
//	                                    <span>&gt; </span>
//	                    </li>
//	                <li class="category87">
//	                        <strong>Linear Bearings</strong>
//	                                </li>
//	        </ul>
//
// </div>
//
// What we want to get is the name (the sections in the <a> or the <strong>) while building up a database of matches to
// the category since their website seems to put a unique category for each
// However the biggest problem is that we don't actually trust their breadcrumbs so we have to rely on knowing what
// pages referenced us and use that location
func getBreadCrumbName(ctx *spiderdata.Context, url string, bc *goquery.Selection) string {
	result := ""
	bc.Find("li").Each(func(i int, li *goquery.Selection) {
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
			spiderdata.OutputError(ctx, "No Class for name: %s url: %s\n", name, url)
		}
		spiderdata.SaveCategory(ctx, name, catclass, url)

		result = spiderdata.MakeBreadCrumb(ctx, result, name)
	})
	savename, found := ctx.G.BreadcrumbMap[url]
	if found && savename != "" {
		fmt.Printf("== For %v extracted breadcrumb '%v' but instead using '%v'\n", url, result, savename)
		result = savename
	}
	return result
}

func processNavList(ctx *spiderdata.Context, breadcrumbs string, navlist *goquery.Selection) (found bool) {
	found = false
	// fmt.Printf("processNavList\n")
	navlist.ChildrenFiltered("li.navList-item").Each(func(i int, item *goquery.Selection) {
		// fmt.Printf("-Found Category product LI element under %v\n", breadcrumbs)
		item.ChildrenFiltered("a.navList-action").Each(func(i int, elem *goquery.Selection) {
			url, _ := elem.Attr("href")
			// elemtext := "<NOT FOUND>"
			elemtext, _ := elem.Attr("title")
			elem.Find("span").Each(func(i int, span *goquery.Selection) {
				elemtext = span.Text()
			})

			localcrumbs := spiderdata.MakeBreadCrumb(ctx, breadcrumbs, elemtext)
			// fmt.Printf("Found Nav item name=%s url=%s\n", localcrumbs, url)
			found = true
			if !ctx.G.SingleOnly {
				spiderdata.EnqueURL(ctx, url, localcrumbs)
			}
			item.ChildrenFiltered("ul.navList").Each(func(i int, subnav *goquery.Selection) {
				processNavList(ctx, localcrumbs, subnav)
			})
		})
	})
	return
}

// processPagination looks for the additional (page2, 3, etc) links which are extensions of the same page
func processPagination(ctx *spiderdata.Context, breadcrumbs string, categoryproducts *goquery.Selection) (found bool) {
	found = false
	// fmt.Printf("processPagination\n")
	categoryproducts.Find("li.pagination-item:not(.pagination-item--current)").Each(func(i int, item *goquery.Selection) {
		// fmt.Printf("-Found pagination element\n")
		item.Find("a").Each(func(i int, elem *goquery.Selection) {
			url, _ := elem.Attr("href")
			// elemtext := elem.Text()
			fmt.Printf("Found pagination item url=%s\n", url)
			found = true
			if !ctx.G.SingleOnly {
				spiderdata.EnqueURL(ctx, url, breadcrumbs)
			}
		})
	})
	return
}

func processSubCategories(ctx *spiderdata.Context, breadcrumbs string, categoryproducts *goquery.Selection) (found bool) {
	found = false
	// fmt.Printf("processSubCategories\n")
	categoryproducts.Find("div.subCategory-name").Each(func(i int, item *goquery.Selection) {
		// fmt.Printf("-Found subCategory name element\n")
		item.Find("a").Each(func(i int, elem *goquery.Selection) {
			url, _ := elem.Attr("href")
			elemtext := elem.Text()
			fmt.Printf("Found subCategory item name=%s url=%s\n", elemtext, url)
			found = true
			subcrumb := spiderdata.MakeBreadCrumb(ctx, breadcrumbs, elemtext)
			if !ctx.G.SingleOnly {
				spiderdata.EnqueURL(ctx, url, subcrumb)
			}
		})
	})
	return
}

func processSubProducts(ctx *spiderdata.Context, breadcrumbs string, categoryproducts *goquery.Selection) (found bool) {
	found = false
	// fmt.Printf("processSubProducts\n")
	categoryproducts.Find("li.productCard").Each(func(i int, item *goquery.Selection) {
		// fmt.Printf("-Found productCard element\n")
		item.Find("h4.card-title a").Each(func(i int, elem *goquery.Selection) {
			url, _ := elem.Attr("href")
			elemtext := elem.Text()
			fmt.Printf("Found ProductCard item name=%s url=%s\n", elemtext, url)
			found = true
			if !ctx.G.SingleOnly {
				spiderdata.EnqueURL(ctx, url, breadcrumbs)
			}
		})
	})
	return
}

// --------------------------------------------------------------------------------------------
// findAllDownloads processes all of the content in the DOM looking for the signature download URLS
func findAllDownloads(ctx *spiderdata.Context, url string, root *goquery.Selection) spiderdata.DownloadEntMap {
	result := spiderdata.DownloadEntMap{}
	// fmt.Printf("findAllDownloads parent='%v'\n", root.Parent().Text())
	// Find h2 elements and filter those containing the text "CAD"
	root.Parent().Find("h2").Each(func(i int, h2elem *goquery.Selection) {
		// fmt.Printf("Checking against: %v\n", h2elem.Text())
		if strings.Contains(h2elem.Text(), "CAD") {
			// fmt.Printf("Processing potential Download\n")
			h2elem.Next().Find("a").Each(func(i int, elem *goquery.Selection) {

				// <a href="https://revrobotics.com/content/cad/REV-41-1562.STEP" target="_blank" rel="noopener">REV-41-1562 STEP File</a>
				// <a href="https://cad.onshape.com/documents/06cfe5484b1c923156af9761/w/ba20e3e567a200c0a057448d/e/23c453dec8707802b812c02a">REV-41-1562 Onshape</a>
				// <a href="https://revrobotics.com/content/docs/REV-41-1562-DR.pdf" target="_blank" rel="noopener">REV-41-1562 Drawing</a>
				title := elem.Text()

				// fmt.Printf("Found one title=%v\n", title)
				dlurl, foundurl := elem.Attr("href")

				// fmt.Printf("Found a on '%v' href=%v\n", elem.Text(), dlurl)
				if title != "" && foundurl {
					// The title often has a string like " STEP" at the end, so we can throw it away
					title = strings.Replace(title, " STEP", "", -1)
					title = strings.Replace(title, " File", "", -1)
					title = strings.Replace(title, " file", "", -1)
					title = strings.Replace(title, " Onshape", "", -1)
					title = strings.Replace(title, " Drawing", "", -1)
					title = strings.Replace(title, " assembly", "", -1)
					title = fixSku(strings.TrimSpace(title))
					// We need to do something special to separate out Step, Onshape and Drawings
					if strings.Contains(dlurl, "cad.onshape") {
						title = "ONSHAPE:" + title
					} else if strings.Contains(dlurl, ".STEP") {
						title = "STEP:" + title
					} else if strings.Contains(dlurl, ".pdf") {
						title = "DRAWING:" + title
					}
					result[title] = spiderdata.DownloadEnt{URL: dlurl, Used: false}
					fmt.Printf("Save Download '%s'='%s'\n", title, dlurl)
					// } else {
					// We don't actually care to complain about these bad URLs as they aren't problematic
					//     if title != "" {
					//         spiderdata.OutputError(ctx, "No URL found associated with %s on %s\n", title, url)
					//     } else if foundurl {
					//         spiderdata.OutputError(ctx, "No Title found for url %s on %s\n", dlurl, url)
					//     } else {
					//         spiderdata.OutputError(ctx, "No URL or Title found with:%s on %s\n", elem.Text(), url)
					//     }
				}
			})
		} else if strings.Contains(h2elem.Text(), "Product Options") {

			// We need to look for buttons in a table
			h2elem.Next().Find("a[name]").Each(func(i int, aelem *goquery.Selection) {
				title, _ := aelem.Attr("name")
				title = fixSku(title)
				// We need to go to the parent tr and find all the A elements underneath
				aelem.ParentsFiltered("tr").First().Find("a").Each(func(i int, elem *goquery.Selection) {
					dlurl, foundurl := elem.Attr("href")
					_, hasname := elem.Attr("name")
					// We need to throw away <a href="https://www.revrobotics.com/15mm-metal-brackets/#REV-41-1303">REV-41-1303-PK8</a>
					if hasname || strings.Contains(dlurl, "/#") {
						return
						// foundurl = false
						// hasname = true
					}
					atitle := title
					if title != "" && foundurl {
						// We need to do something special to separate out Step, Onshape and Drawings
						if strings.Contains(dlurl, "cad.onshape") {
							atitle = "ONSHAPE:" + atitle
						} else if strings.Contains(dlurl, ".STEP") {
							atitle = "STEP:" + atitle
						} else if strings.Contains(dlurl, ".pdf") {
							atitle = "DRAWING:" + atitle
						}
						result[atitle] = spiderdata.DownloadEnt{URL: dlurl, Used: false}
						fmt.Printf("Save Download '%s'='%s'\n", atitle, dlurl)
					} else {
						if title == "" {
							spiderdata.OutputError(ctx, "No URL found associated with %s on %s\n", title, url)
						} else if foundurl {
							spiderdata.OutputError(ctx, "No Title found for url %s on %s\n", dlurl, url)
						} else {
							spiderdata.OutputError(ctx, "No URL or Title found with:%s on %s\n", elem.Text(), url)
						}
					}
				})
			})
		}
	})
	return result
}

func fixSku(sku string) (result string) {
	// See if our SKU ended with a -PK<n> and try again
	// Define a regular expression that matches "-PK" followed by one or more digits at the end of the string
	re := regexp.MustCompile(`-PK\d+$`)
	result = re.ReplaceAllString(sku, "")
	return
}

// --------------------------------------------------------------------------------------------
// getDownloadURL looks in the download map for a matching entry and returns the corresponding URL, marking it as used
// from the list of downloads so that we know what is left over
func getKeyDownloadURL(sku string, downloadurls spiderdata.DownloadEntMap, key string) (result string, found bool) {
	result = ""
	found = false

	keylook := sku
	if key != "" {
		keylook = key + ":" + sku
	}

	ent, found1 := downloadurls[keylook]
	if found1 {
		result = ent.URL
		found = true
		downloadurls[keylook] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
	} else {
		// See if our SKU ended with a -PK<n> and try again
		// Replace the matched pattern with an empty string (remove it)
		keylook = fixSku(keylook)
		ent, found = downloadurls[keylook]
		if found {
			result = ent.URL
			downloadurls[keylook] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
		}
	}
	return
}

// --------------------------------------------------------------------------------------------
// getDownloadURL looks in the download map for a matching entry and returns the corresponding URL, marking it as used
// from the list of downloads so that we know what is left over
func getDownloadURL(_ /*ctx*/ *spiderdata.Context, sku string, downloadurls spiderdata.DownloadEntMap) (result string) {
	result = "<NOMODEL:" + sku + ">"

	// We will first look for a ONSHAPE: version and use it if found,
	// Then we look for the STEP:
	// otherwise we go for the DRAWING: version
	// title = "ONSHAPE:" + title
	// title = "STEP:" + title
	// title = "DRAWING:" + title

	onshapeurl, foundonshape := getKeyDownloadURL(sku, downloadurls, "ONSHAPE")
	stepurl, foundstep := getKeyDownloadURL(sku, downloadurls, "STEP")
	drawingurl, founddrawing := getKeyDownloadURL(sku, downloadurls, "DRAWING")
	if foundonshape {
		// Best choice, use the Onshape version
		result = onshapeurl
	} else if foundstep {
		// Yes, we use the step model
		result = stepurl
	} else if founddrawing {
		// Perfect, we use the drawing
		result = drawingurl
	} else {
		// No STEP or DRAWING, check for just a plain link
		url, found := getKeyDownloadURL(sku, downloadurls, "")
		if found {
			// We got the plain link.
			result = url
		} else {
			// We didn't find the sku in the list, but it is possible that they misnamed it.
			// For example https://www.servocity.com/8mm-4-barrel  has a SKU of 545314
			// But the text for the URL is mistyped as '535314' but it links to 'https://www.servocity.com/media/attachment/file/5/4/545314.zip'
			// So we want to try to use it
			for key, element := range downloadurls {
				if !element.Used && strings.Contains(element.URL, sku) {
					result = element.URL
					downloadurls[key] = spiderdata.DownloadEnt{URL: element.URL, Used: true}
				}
			}

		}
	}
	return
}

// --------------------------------------------------------------------------------------------
// showUnusedURLs displays all of the downloads which were on a page but not referenced by any
// of the products associated with the page.  Typically this happens because the product number
// doesn't match or because it is an out of date product which has been removed
func showUnusedURLS(ctx *spiderdata.Context, url string, downloadurls spiderdata.DownloadEntMap) {
	for key, element := range downloadurls {
		if !element.Used {
			// If it says instructions or other nonesense, we can ignore it.
			if strings.Contains(key, "Instructions") &&
				strings.Contains(key, "Spec Sheet") &&
				strings.Contains(key, "Specs") &&
				strings.Contains(key, "Guide") &&
				strings.Contains(key, "Diagram") &&
				strings.Contains(key, "Charts") &&
				strings.Contains(key, "Manual") &&
				strings.Contains(key, "Pattern Information") &&
				strings.Contains(key, "Use Parameter") {

				spiderdata.OutputError(ctx, "Unused download '%s': %s on %s\n", key, element.URL, url)
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// processProductGrid takes a standard page which has a single product on it and outputs the information
func processProductGrid(ctx *spiderdata.Context, breadcrumbs string, _ /*url*/ string, pg *goquery.Selection) (found bool) {
	found = false
	// fmt.Printf("Parents found: %d\n", pg.ParentFiltered("div.tab-content").Length())
	if pg.ParentFiltered("div.tab-content").Length() == 0 {
		pg.Find("li.product h4.card-title a").Each(func(i int, a *goquery.Selection) {
			urlloc, _ := a.Attr("href")
			product := a.Text()
			fmt.Printf("**ProductGrid Found item name=%s url=%s\n", product, urlloc)
			found = true
			if !ctx.G.SingleOnly {

				spiderdata.EnqueURL(ctx, urlloc, spiderdata.MakeBreadCrumb(ctx, breadcrumbs, product))
			}
		})
	}
	return
}

// --------------------------------------------------------------------------------------------
// processProductGrid takes a standard page which has a single product on it and outputs the information
func processQaatcList(ctx *spiderdata.Context, breadcrumbs string, _ /*url*/ string, pg *goquery.Selection) (found bool) {
	found = false

	// fmt.Printf("Parents found: %d\n", pg.ParentFiltered("div.tab-content").Length())
	if pg.ParentFiltered("div.tab-content").Length() == 0 {
		pg.Find("li.qaatc__item  a.qaatc__name").Each(func(i int, a *goquery.Selection) {
			urlloc, _ := a.Attr("href")
			product := a.Text()
			fmt.Printf("**processQaatcList Found item name=%s url=%s\n", product, urlloc)
			found = true
			if !ctx.G.SingleOnly {
				spiderdata.EnqueURL(ctx, urlloc, breadcrumbs)
			}
		})
	}
	return
}

// --------------------------------------------------------------------------------------------
// processProduct takes a standard page which has a single product on it and outputs the information
func processProduct(ctx *spiderdata.Context, productname string, url string, product *goquery.Selection) (found bool) {
	outpad := make([]string, 7)
	found = false
	spiderdata.OutputCategory(ctx, productname, false)
	localname := product.Find("div.productView-product h1.productView-title").Text()
	sku := product.Find("div.productView-product .productView-info-value").Text()
	// We need to strip off the -PK<n>
	sku = fixSku(sku)

	// fmt.Printf("Process Product\n")
	changesetInputs := product.Find("[data-product-option-change]").Find("input")
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
	_, isSingle := SingleSKUs[sku]

	if changesetInputs.Length() > 0 && !isSingle {
		//fmt.Printf("Has Changeset\n")
		changesetInputs.Each(func(i int, input *goquery.Selection) {
			id, hasid := input.Attr("id")
			datalabel, hasdatalabel := input.Attr("data-option-label")
			if hasid {
				label := input.Parent().Find(fmt.Sprintf("label[for='%s']", id))

				if label.Length() > 0 {
					labelText := label.Text()
					// Regular expression to capture both the SKU and the description
					//					re := regexp.MustCompile(`\((REV-[\d-]+)(?:-PK\d+)?\)[\s ]+(.+?)[\s ]*-\s*\d+[\s ]*Pack`)
					re := regexp.MustCompile(`\((REV-[\d-]+(?:-PK\d+)?)\)[\s ]+(.+?)(?:\s*-\s*\d+\s*Pack)?$`)

					matches := re.FindStringSubmatch(labelText)
					if len(matches) < 2 && hasdatalabel {
						codemap, foundcodemap := EquivSKUs[localname]
						if foundcodemap {
							code, foundcode := codemap[datalabel]
							if foundcode {
								matches = []string{"", code, localname + " - " + datalabel}
							}
						}
					}
					if len(matches) > 2 {
						itemsku := fixSku(matches[1])
						itemname := matches[2]
						// We have a pair like this:
						//      <input class="form-radio" type="radio" id="attribute_radio_114" name="attribute[53]" value="114" required="" data-state="false">
						//      <label data-product-attribute-value="114" class="form-label" for="attribute_radio_114">30cm</label>
						outpad[6], _ = getKeyDownloadURL(itemsku, downloadurls, "STEP")
						spiderdata.OutputProduct(ctx, itemname, itemsku, url, getDownloadURL(ctx, itemsku, downloadurls), false, outpad)
					}
				}
			}
		})
		found = true
	} else if sku != "" {
		// fmt.Printf("No Changeset\n")
		outpad[6], _ = getKeyDownloadURL(sku, downloadurls, "STEP")
		spiderdata.OutputProduct(ctx, localname, sku, url, getDownloadURL(ctx, sku, downloadurls), false, outpad)
		found = true
	}

	showUnusedURLS(ctx, url, downloadurls)
	return
}

// ParseRevRoboticsPage parses a page and adds links to elements found within by the various processors
func ParseRevRoboticsPage(ctx *spiderdata.Context, doc *goquery.Document) {
	ctx.G.Mu.Lock()
	url := ctx.Url

	found := false
	breadcrumbs := getBreadCrumbName(ctx, url, doc.Find("ul.breadcrumbs"))
	fmt.Printf("Breadcrumb:%s\n", breadcrumbs)
	doc.Find("ul.navList").Each(func(i int, navItems *goquery.Selection) {
		// fmt.Printf("Found Navlist\n")
		if navItems.ParentsFiltered("ul.navList").Length() == 0 {
			if processNavList(ctx, "", navItems) {
				found = true
			}
		}
	})
	// Find all the pagination links from this page
	doc.Find("div.pagination ul.pagination-list").Each(func(i int, pagination *goquery.Selection) {
		// fmt.Printf("Found Subcategory\n")
		_ = processPagination(ctx, breadcrumbs, pagination)
	})
	doc.Find("div.subCategories").Each(func(i int, subCategories *goquery.Selection) {
		// fmt.Printf("Found Subcategory\n")
		_ = processSubCategories(ctx, breadcrumbs, subCategories)
	})
	doc.Find("div.productCategoryCompare").Each(func(i int, products *goquery.Selection) {
		_ = processSubProducts(ctx, breadcrumbs, products)
	})
	if !found {
		doc.Find("ul.productGrid").Each(func(i int, product *goquery.Selection) {
			if processProductGrid(ctx, breadcrumbs, url, product) {
				found = true
				// } else if processProductViewWithTable(ctx, breadcrumbs, url, product) {
				// 	found = true
			}
		})
	}
	doc.Find("ul.qaatc__list").Each(func(i int, product *goquery.Selection) {
		if processQaatcList(ctx, breadcrumbs, url, product) {
			found = true
		}
	})
	if !found {
		doc.Find("div.productView").Each(func(i int, product *goquery.Selection) {
			if processProduct(ctx, breadcrumbs, url, product) {
				found = true
			}
		})
	}
	if !found {
		spiderdata.OutputError(ctx, "Unable to process: %s\n", url)
	}
	ctx.G.Mu.Unlock()
}
