package revrobotics

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/toebes/ftc_parts_spider/spiderdata"

	"github.com/PuerkitoBio/goquery"
)

// RevRoboticsTarget is the configuration structure for spidering the Rev Robotics website
var RevRoboticsTarget = spiderdata.SpiderTarget{
	Outfile:        "rev_robotics.txt",
	SpreadsheetID:  "19Mc9Uj0zoaRr_KmPncf_svNOp9WqIgrzaD7fEiNlBr0",
	Presets:        []string{},
	Seed:           "https://www.revrobotics.com/ftc/",
	ParsePageFunc:  ParseRevRoboticsPage,
	CheckMatchFunc: CheckRevRoboticsMatch,

	SectionNameDeletes: []string{
		"Shop by Electrical Connector Style > ",
		"Shop by Hub Style > ",
		" Aluminum REX Shafting >",
		" Stainless Steel D-Shafting >",
		" > Motor Mounts for AndyMark NeveRest Motors > Motor Mounts for NeveRest Orbital Gear Motors",
		" > Motor Mounts for REV Robotics Motors > Motor Mounts for REV Core Hex Motor",
		" > Motor Mounts for REV Robotics Motors > Motor Mounts for REV UltraPlanetary Gearbox",
		" > XL Series, 3/8\" Width Timing Belts",
		" > XL Series, 3/8\" Width, Cut Length Timing Belts",
	},
	SectionAllowedMap: map[string]string{
		"637213":   "KITS > Linear Motion Kits",
		"ASCC8074": "HARDWARE > Lubricants",
		"555192":   "STRUCTURE > Motor Mounts > Motor Mounts for NeveRest Classic Gear Motors",
		"555104":   "STRUCTURE > Motor Mounts > Motor Mounts for Econ Spur Gear Motors",
		"585074":   "STRUCTURE > X-Rail® > X-Rail® Accessories",
		"585073":   "STRUCTURE > X-Rail® > X-Rail® Accessories",
		"605638":   "STRUCTURE > X-Rail® > X-Rail® Accessories",
	},
	SectionEquivalents: [][]string{
		{"MOTION > Bearings", "MOTION > Linear Bearings"},
		{"KITS > FTC Kits", "KITS > Linear Motion Kits"},
		{"MOTION > Couplers > Shop by Coupler Bore", "MOTION > Couplers > "},
		{"ELECTRONICS > Wiring > Connector Style", "ELECTRONICS > Wiring > "},
		{"MOTION > Hubs > Servo Hubs", "MOTION > Servos & Accessories > Servo Hubs"},
		{"MOTION > Servos & Accessories > Servos", "MOTION > Servos & Accessories"},
		{"STRUCTURE > Adaptors", "MOTION > Hubs"},
		{"STRUCTURE > Brackets", "STRUCTURE > X-Rail® > X-Rail® Accessories"},
	},
}

// CheckRevRoboticsMatch compares a partData to what has been captured from the spreadsheet
// Any differences are put into the notes
func CheckRevRoboticsMatch(ctx *spiderdata.Context, partData *spiderdata.PartData) {
	entry, found := ctx.G.ReferenceData.PartNumber[partData.SKU]
	if !found {
		entry, found = ctx.G.ReferenceData.URL[partData.URL]
	}
	if found {
		// We matched a previous entry
		ctx.G.ReferenceData.Mu.Lock()
		delete(ctx.G.ReferenceData.URL, entry.URL)
		delete(ctx.G.ReferenceData.PartNumber, entry.SKU)
		ctx.G.ReferenceData.Mu.Unlock()

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
		partData.SpiderStatus = "Same"
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
				partData.SpiderStatus = "Changed"
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
				partData.SpiderStatus = "Changed"
				partData.Notes += extra + "New Name:" + newName
				partData.Name = oldName
				extra = separator
			}
		}
		// If the SKU changes then we really want to know it.  We should use the new SKU
		// and stash away the old SKU but it needs to be updated
		if !strings.EqualFold(partData.SKU, entry.SKU) {
			partData.SpiderStatus = "Changed"
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
				partData.SpiderStatus = "Changed"
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
		partData.SpiderStatus = "Not Found"
		partData.Status = "Not Done"
	}

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
func getBreadCrumbName(ctx *spiderdata.Context, url string, bc *goquery.Selection) string {
	result := ""
	prevresult := ""
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

		prevresult = result
		result = spiderdata.MakeBreadCrumb(ctx, result, name)
	})
	// fmt.Printf("+++Extracted breadcrumb was '%v' lastname='%v' prevresult='%v'\n", result, lastname, prevresult)
	// Now see if the breadcrumb was Home > Shop All (without the last name)
	if strings.EqualFold(prevresult, "Home > Shop All") {
		// It was, so we need to extract the proper name
		savename, found := ctx.G.BreadcrumbMap[url]
		// fmt.Printf("+++Checking savename='%v' found=%v for url='%v'\n", savename, found, url)
		if found {
			result = savename
		}
	}
	return result
}

func processSubCategory(ctx *spiderdata.Context, breadcrumbs string, categoryproducts *goquery.Selection) (found bool) {
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
			spiderdata.EnqueURL(ctx, url, breadcrumbs)
		})
	})
	return
}

// --------------------------------------------------------------------------------------------
// findAllDownloads processes all of the content in the DOM looking for the signature download URLS
func findAllDownloads(ctx *spiderdata.Context, url string, root *goquery.Selection) spiderdata.DownloadEntMap {
	result := spiderdata.DownloadEntMap{}
	// fmt.Printf("findAllDownloads parent='%v'\n", root.Parent().Text())
	root.Parent().Find("a").Each(func(i int, elem *goquery.Selection) {
		//<a title="REV-31-1425 STEP File" href="/content/cad/REV-31-1425.STEP" download="REV-31-1425.STEP">REV-31-1425 STEP File</a>
		title := elem.Text()
		download, hasdownload := elem.Attr("download")
		j1, _ := elem.Attr("href")
		fmt.Printf("Found a on '%v' href=%v download=%v hasdownload=%v\n", elem.Text(), j1, download, hasdownload)
		if hasdownload {
			fmt.Printf("Found one title=%v\n", title)
			dlurl, foundurl := elem.Attr("href")

			if title != "" && foundurl {
				// The title often has a string like " STEP" at the end, so we can throw it away
				title = strings.Replace(title, " STEP", "", -1)
				title = strings.Replace(title, " File", "", -1)
				title = strings.Replace(title, " file", "", -1)
				title = strings.Replace(title, " assembly", "", -1)
				title = strings.TrimSpace(title)
				result[title] = spiderdata.DownloadEnt{URL: dlurl, Used: false}
				fmt.Printf("Save Download '%s'='%s'\n", title, dlurl)
			} else {
				if title == "" {
					spiderdata.OutputError(ctx, "No URL found associated with %s on %s\n", title, url)
				} else if foundurl {
					spiderdata.OutputError(ctx, "No Title found for url %s on %s\n", dlurl, url)
				} else {
					spiderdata.OutputError(ctx, "No URL or Title found with:%s on %s\n", elem.Text(), url)
				}
			}
		}
	})
	return result
}

// --------------------------------------------------------------------------------------------
// getDownloadURL looks in the download map for a matching entry and returns the corresponding URL, marking it as used
// from the list of downloads so that we know what is left over
func getDownloadURL(ctx *spiderdata.Context, sku string, downloadurls spiderdata.DownloadEntMap) (result string) {
	result = "<NOMODEL:" + sku + ">"
	ent, found := downloadurls[sku]
	if found {
		result = ent.URL
		downloadurls[sku] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
	} else {
		// We didn't find the sku in the list, but it is possible that they misnamed it.
		// For example https://www.servocity.com/8mm-4-barrel  has a SKU of 545314
		// But the text for the URL is mistyped as '535314' but it links to 'https://www.servocity.com/media/attachment/file/5/4/545314.zip'
		// So we want to try to use it
		for key, element := range downloadurls {
			if !element.Used && strings.Index(element.URL, sku) >= 0 {
				result = element.URL
				downloadurls[key] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
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
			if strings.Index(key, "Instructions") < 0 &&
				strings.Index(key, "Spec Sheet") < 0 &&
				strings.Index(key, "Specs") < 0 &&
				strings.Index(key, "Guide") < 0 &&
				strings.Index(key, "Diagram") < 0 &&
				strings.Index(key, "Charts") < 0 &&
				strings.Index(key, "Manual") < 0 &&
				strings.Index(key, "Pattern Information") < 0 &&
				strings.Index(key, "Use Parameter") < 0 {

				spiderdata.OutputError(ctx, "Unused download '%s': %s on %s\n", key, element.URL, url)
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// processProductGrid takes a standard page which has a single product on it and outputs the information
func processProductGrid(ctx *spiderdata.Context, breadcrumbs string, url string, pg *goquery.Selection) (found bool) {
	found = false
	// fmt.Printf("Parents found: %d\n", pg.ParentFiltered("div.tab-content").Length())
	if pg.ParentFiltered("div.tab-content").Length() == 0 {
		pg.Find("li.product h4.card-title a").Each(func(i int, a *goquery.Selection) {
			urlloc, _ := a.Attr("href")
			product := a.Text()
			fmt.Printf("**ProductGrid Found item name=%s url=%s\n", product, urlloc)
			found = true
			spiderdata.EnqueURL(ctx, urlloc, spiderdata.MakeBreadCrumb(ctx, breadcrumbs, product))
		})
	}
	return
}

// --------------------------------------------------------------------------------------------
// processProductGrid takes a standard page which has a single product on it and outputs the information
func processQaatcList(ctx *spiderdata.Context, breadcrumbs string, url string, pg *goquery.Selection) (found bool) {
	found = false
	// fmt.Printf("Parents found: %d\n", pg.ParentFiltered("div.tab-content").Length())
	if pg.ParentFiltered("div.tab-content").Length() == 0 {
		pg.Find("li.qaatc__item  a.qaatc__name").Each(func(i int, a *goquery.Selection) {
			urlloc, _ := a.Attr("href")
			product := a.Text()
			fmt.Printf("**processQaatcList Found item name=%s url=%s\n", product, urlloc)
			found = true
			spiderdata.EnqueURL(ctx, urlloc, spiderdata.MakeBreadCrumb(ctx, breadcrumbs, product))
		})
	}
	return
}

// --------------------------------------------------------------------------------------------
// processProduct takes a standard page which has a single product on it and outputs the information
func processProduct(ctx *spiderdata.Context, productname string, url string, product *goquery.Selection) (found bool) {
	found = false
	spiderdata.OutputCategory(ctx, productname, true)
	localname := product.Find("div.productView-product h1.productView-title").Text()
	sku := product.Find("div.productSKU .productView-info-value").Text()
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
	if sku != "" {
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
				spiderdata.OutputProduct(ctx, itemname, itemsku, url, getDownloadURL(ctx, sku, downloadurls), false, nil)

			})
		} else {
			fmt.Printf("No Changeset\n")
			spiderdata.OutputProduct(ctx, localname, sku, url, getDownloadURL(ctx, sku, downloadurls), false, nil)
		}
		found = true
	}
	showUnusedURLS(ctx, url, downloadurls)
	return
}

// --------------------------------------------------------------------------------------------
// ProcessTable parses a table of parts and outputs all of the products in the table
// The first step is
func processTable(ctx *spiderdata.Context, productname string, url string, downloadurls spiderdata.DownloadEntMap, table *goquery.Selection) (result bool) {
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
			spiderdata.OutputProduct(ctx, outname, sku, url, getDownloadURL(ctx, sku, downloadurls), false, outpad)
		})
	}
	return
}

func processJavascriptSelector(ctx *spiderdata.Context, breadcrumbs string, url string, product *goquery.Selection) (found bool) {
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
		spiderdata.OutputProduct(ctx, productname+" "+value.Label, value.SKU, url, getDownloadURL(ctx, value.SKU, downloadurls), false, nil)
		found = true
	}
	return
}

func processSimpleProductTable(ctx *spiderdata.Context, breadcrumbs string, url string, productname string, root *goquery.Selection, table *goquery.Selection) (found bool) {
	found = false
	downloadurls := findAllDownloads(ctx, url, root)
	spiderdata.OutputCategory(ctx, breadcrumbs, false)
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

// ParseRevRoboticsPage parses a page and adds links to elements found within by the various processors
func ParseRevRoboticsPage(ctx *spiderdata.Context, doc *goquery.Document) {
	ctx.G.Mu.Lock()
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
	// qaatc__list
	if !found {
		doc.Find("ul.productGrid").Each(func(i int, product *goquery.Selection) {
			if processProductGrid(ctx, breadcrumbs, url, product) {
				found = true
				// } else if processProductViewWithTable(ctx, breadcrumbs, url, product) {
				// 	found = true
			}
		})
	}
	if !found {
		doc.Find("div.productView").Each(func(i int, product *goquery.Selection) {
			if processProduct(ctx, breadcrumbs, url, product) {
				found = true
			}
		})
	}
	if !found {
		doc.Find("ul.qaatc__list").Each(func(i int, product *goquery.Selection) {
			if processQaatcList(ctx, breadcrumbs, url, product) {
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
	if !found {
		spiderdata.OutputError(ctx, "Unable to process: %s\n", url)
	}
	ctx.G.Mu.Unlock()
}
