package servocity

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/toebes/spider_gobilda/spiderdata"

	"github.com/PuerkitoBio/goquery"
)

// ServocityTarget is the configuration structure for spidering the ServoCity website
var ServocityTarget = spiderdata.SpiderTarget{
	Outfile:       "servocity.txt",
	SpreadsheetID: "15Mm-Thdcpl5fVPs3vnyFUXWthuaV1tacXPJ7xQuoB8A",
	Presets: []string{"https://www.servocity.com/structure/",
		"https://www.servocity.com/motion/",
		"https://www.servocity.com/electronics/",
		"https://www.servocity.com/hardware/",
		"https://www.servocity.com/kits/",
	},
	Seed:           "https://www.servocity.com/electronics/",
	ParsePageFunc:  parseServocityPage,
	CheckMatchFunc: checkServocityMatch,
}

// checkServocityMatch compares a partData to what has been captured from the spreadsheet
// Any differences are put into the notes
func checkServocityMatch(ctx *spiderdata.Context, partData *spiderdata.PartData) {
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
			deleteParts := []string{
				"Shop by Electrical Connector Style > ",
				"Shop by Hub Style > ",
				" Aluminum REX Shafting >",
				" Stainless Steel D-Shafting >",
				" > Motor Mounts for AndyMark NeveRest Motors > Motor Mounts for NeveRest Orbital Gear Motors",
				" > Motor Mounts for REV Robotics Motors > Motor Mounts for REV Core Hex Motor",
				" > Motor Mounts for REV Robotics Motors > Motor Mounts for REV UltraPlanetary Gearbox",
				" > XL Series, 3/8\" Width Timing Belts",
				" > XL Series, 3/8\" Width, Cut Length Timing Belts",
			}
			for _, deleteStr := range deleteParts {
				newsection = strings.ReplaceAll(newsection, deleteStr, "")
			}
			//  Then strip leading/trailing blanks
			newsection = strings.TrimSpace(newsection)
			// Special cases
			allowedMap := map[string]string{
				// Go Bilda
				"1310-0016-4012": "MOTION > Hubs > Hyper Hubs (16mm Pattern)",
				"1311-0016-1006": "MOTION > Hubs > Sonic Hubs > Thru-Hole Sonic Hubs (16mm Pattern)",
				"1309-0016-1006": "MOTION > Hubs > Sonic Hubs > Sonic Hubs (16mm Pattern)",
				"1310-0016-1006": "MOTION > Hubs > Hyper Hubs (16mm Pattern)",
				"1312-0016-1006": "MOTION > Hubs > Sonic Hubs > Double Sonic Hubs (16mm Pattern)",
				"1309-0016-0006": "MOTION > Hubs > Sonic Hubs > Sonic Hubs (16mm Pattern)",
				"1310-0016-0008": "MOTION > Hubs > Hyper Hubs (16mm Pattern)",
				"1310-0016-5008": "MOTION > Hubs > Hyper Hubs (16mm Pattern)",
				"1123-0048-0048": "STRUCTURE > Pattern Plates",
				// Servocity
				"637213":   "KITS > Linear Motion Kits",
				"ASCC8074": "HARDWARE > Lubricants",
				"555192":   "STRUCTURE > Motor Mounts > Motor Mounts for NeveRest Classic Gear Motors",
				"555104":   "STRUCTURE > Motor Mounts > Motor Mounts for Econ Spur Gear Motors",
				"585074":   "STRUCTURE > X-Rail® > X-Rail® Accessories",
				"585073":   "STRUCTURE > X-Rail® > X-Rail® Accessories",
				"605638":   "STRUCTURE > X-Rail® > X-Rail® Accessories",
			}
			// Other mappings allowed
			equivalentMaps := [][]string{
				{"MOTION > Bearings", "MOTION > Linear Bearings"},
				{"KITS > FTC Kits", "KITS > Linear Motion Kits"},
				{"MOTION > Couplers > Shop by Coupler Bore", "MOTION > Couplers > "},
				{"ELECTRONICS > Wiring > Connector Style", "ELECTRONICS > Wiring > "},
				{"MOTION > Hubs > Servo Hubs", "MOTION > Servos & Accessories > Servo Hubs"},
				{"MOTION > Servos & Accessories > Servos", "MOTION > Servos & Accessories"},
				{"STRUCTURE > Adaptors", "MOTION > Hubs"},
				{"STRUCTURE > Brackets", "STRUCTURE > X-Rail® > X-Rail® Accessories"},
			}

			// Look for any equivalent mappings so that we end up trusting what is already in the spreadsheet.
			propersection, matched := allowedMap[entry.SKU]
			if len(oldsection) > len(newsection) && strings.EqualFold(newsection, oldsection[:len(newsection)]) {
				newsection = oldsection
			}

			for _, strset := range equivalentMaps {
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

// --------------------------------------------------------------------------------------------
// findAllDownloads processes all of the content in the DOM looking for the signature download URLS
func findAllDownloads(ctx *spiderdata.Context, url string, root *goquery.Selection) spiderdata.DownloadEntMap {
	result := spiderdata.DownloadEntMap{}
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
		return
	}
	ent, found = downloadurls[strings.ToLower(sku)]
	if found {
		result = ent.URL
		downloadurls[strings.ToLower(sku)] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
		return
	}
	// We didn't find the sku in the list, but it is possible that they misnamed it.
	// For example https://www.servocity.com/8mm-4-barrel  has a SKU of 545314
	// But the text for the URL is mistyped as '535314' but it links to 'https://www.servocity.com/media/attachment/file/5/4/545314.zip'
	// So we want to try to use it
	for key, element := range downloadurls {
		if !element.Used && strings.Index(element.URL, sku) >= 0 {
			result = element.URL
			downloadurls[key] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
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
		result = ent.URL
		downloadurls[renames[sku]] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
		return
	}

	// Ok last try.. This is the case where we have a cases such as
	//      HDA8-30  vs   hda8_assembly_2
	// To match, we drop everything after the - and check the list again
	skupart := strings.Split(strings.ToLower(sku), "-")
	for key, element := range downloadurls {
		if !element.Used && strings.Index(element.URL, skupart[0]) >= 0 {
			result = element.URL
			downloadurls[key] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
			return
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

				spiderdata.OutputError(ctx, "Unused download``%s``%s`%s\n", key, url, element.URL)
			}
		}
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
// processProductGrid takes a standard page which has a single product on it and outputs the information
func processProductGrid(ctx *spiderdata.Context, breadcrumbs string, url string, pg *goquery.Selection) (found bool) {
	found = false
	fmt.Printf("Parents found: %d\n", pg.ParentFiltered("div.tab-content").Length())
	if pg.ParentFiltered("div.tab-content").Length() == 0 {
		pg.Find("li.product a[data-card-type],li.product a.card").Each(func(i int, a *goquery.Selection) {
			urlloc, _ := a.Attr("href")
			product, _ := a.Attr("title")
			fmt.Printf("**ProductGrid Found item name=%v url=%v on %v\n", product, urlloc, url)
			found = true
			spiderdata.EnqueURL(ctx, urlloc, spiderdata.MakeBreadCrumb(ctx, breadcrumbs, product))
		})
	}
	return
}

// --------------------------------------------------------------------------------------------
// processProductTableList takes a standard page which has a single product on it and outputs the information
func processProductTableList(ctx *spiderdata.Context, breadcrumbs string, url string, table *goquery.Selection) (found bool) {
	found = false
	// fmt.Printf("Parents found: %d\n", pg.ParentFiltered("div.tab-content").Length())
	if table.ParentFiltered("div.tab-content").Length() == 0 {
		table.Find("td.productTable-cell a.tableSKU").Each(func(i int, a *goquery.Selection) {
			urlloc, _ := a.Attr("href")
			product := a.Text()
			product = strings.Trim(strings.ReplaceAll(product, "\n", ""), " ")
			fmt.Printf("**processProductTableList Found item name=%s url=%s\n", product, urlloc)
			found = true
			spiderdata.EnqueURL(ctx, urlloc, spiderdata.MakeBreadCrumb(ctx, breadcrumbs, product))
		})
	}
	return
}

// --------------------------------------------------------------------------------------------
// processLazyLoad finds all the lazy loaded sub pages
func processLazyLoad(ctx *spiderdata.Context, breadcrumbs string, url string, js *goquery.Selection) (found bool) {
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
							spiderdata.EnqueURL(ctx, urlpart, breadcrumbs)
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
func processProduct(ctx *spiderdata.Context, productname string, url string, product *goquery.Selection, isDiscontinued bool, addSKU bool) (found bool) {
	found = false
	spiderdata.OutputCategory(ctx, productname, false)
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
				spiderdata.OutputProduct(ctx, itemname, itemsku, url, getDownloadURL(ctx, sku, downloadurls), isDiscontinued, nil)
			})
		} else {
			if addSKU {
				url, _ = spiderdata.CleanURL(url)
				url += "?sku=" + sku
			}
			spiderdata.OutputProduct(ctx, localname, sku, url, getDownloadURL(ctx, sku, downloadurls), isDiscontinued, nil)
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

// parseServocityPage parses a page and adds links to elements found within by the various processors
func parseServocityPage(ctx *spiderdata.Context, doc *goquery.Document) {
	ctx.G.Mu.Lock()
	url := doc.Url.String()
	found := false
	breadcrumbs := getBreadCrumbName(ctx, url, doc.Find("ul.breadcrumbs"))
	spiderdata.MarkVisitedURL(ctx, url, breadcrumbs)

	// see if this has been discontinued
	isDiscontinued := (doc.Find("p.discontinued").Length() > 0)

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
		products := doc.Find("div[itemtype=\"http://schema.org/Product\"]")
		products.Each(func(i int, product *goquery.Selection) {
			if processProduct(ctx, breadcrumbs, url, product, isDiscontinued, products.Length() > 1) {
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
