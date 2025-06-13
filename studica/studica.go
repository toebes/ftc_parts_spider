package studica

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/toebes/ftc_parts_spider/partcatalog"
	"github.com/toebes/ftc_parts_spider/spiderdata"
)

// StudicaTarget is the configuration structure for spidering the Studica website
var StudicaTarget = spiderdata.SpiderTarget{
	Outfile:            "studica.txt",
	SpreadsheetID:      "1xomFgFZ3Ie79XHOMbAX76sSRYDzkkj3VywsakY3DCjA",
	Presets:            []string{},
	StripSKU:           false,
	Seed:               "https://www.studica.com/sitemap.xml",
	ParsePageFunc:      ParseStudicaPage,
	CheckMatchFunc:     CheckStudicaMatch,
	SectionNameDeletes: []string{},
	SectionAllowedMap:  map[string]string{},
	SectionEquivalents: [][]string{},
	SkipPages: []string{
		"https://www.studica.com/blog",
		"https://www.studica.com/search",
		"https://www.studica.com/studica-resources",
		"https://www.studica.com/education-webinars-for-teachers",
		"https://www.studica.com/contactus",
		"https://www.studica.com/industry",
		"https://www.studica.com/manufacturer/all",
		"https://blog.studica.com",
		"https://www.studica.com/webinars",
		"https://www.studica.com/contactus",
		"https://www.studica.com/drones-uav",
		"https://www.studica.com/classroom",
		"https://www.studica.com/animation-cad-modeling",
		"https://www.studica.com/career-tech-education",
		"https://www.studica.com/coding-learn-to-program",
		"https://www.studica.com/curriculum-solutions",
		"https://www.studica.com/engineering-education",
		"https://www.studica.com/kitting-services",
		"https://www.studica.com/education-pricing-babbel-for-classroom",
		"https://www.studica.com/science-education",
		"https://www.studica.com/stem-programs",
		"https://www.studica.com/students",
		"https://www.studica.com/student-software-discounts",
		"https://www.studica.com/school-affiliates",
		"https://www.studica.com/industry",
		"https://www.studica.com/architecture",
		"https://www.studica.com/automation-controls",
		"https://www.studica.com/robotics-3",
		"https://www.studica.com/cnc-machines",
		"https://www.studica.com/clearance",
	},
}

// const menuPrefix = "/menus/"

// CheckStudicaMatch compares a partData to what has been captured from the spreadsheet
// Any differences are put into the notes
func CheckStudicaMatch(ctx *spiderdata.Context, partData *partcatalog.PartData) {
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

// --------------------------------------------------------------------------------------------
// findAllDownloads processes all of the content in the DOM looking for the signature download URLS
func findAllDownloads(ctx *spiderdata.Context, url string, root *goquery.Selection) spiderdata.DownloadEntMap {
	result := spiderdata.DownloadEntMap{}
	// fmt.Printf("findAllDownloads parent='%v'\n", root.Parent().Text())
	root.Parent().Find("div.full-description a").Each(func(i int, elem *goquery.Selection) {
		//<a target="_blank" class="product-documents__link" href="https://andymark-weblinc.netdna-ssl.com/media/W1siZiIsIjIwMTgvMTEvMDYvMTUvMDIvMTQvNTMwZjE4YmMtMmM5NS00Yzk3LTg3OWMtZjNmYzI1MTllMzJiL2FtLTMyODQgMzJ0IE5pbmphIFN0YXIgU3Byb2NrZXQuU1RFUCJdXQ/am-3284%2032t%20Ninja%20Star%20Sprocket.STEP?sha=9834a1285a141ddc">am-3284 32t Ninja Star Sprocket.STEP</a>
		title := strings.TrimSpace(elem.Text())
		dlurl, foundurl := elem.Attr("href")
		fmt.Printf("Found a on '%v' href=%v\n", elem.Text(), dlurl)

		if title == "" {
			spiderdata.OutputError(ctx, "No Title found for url %s on %s\n", dlurl, url)
		} else if !foundurl {
			spiderdata.OutputError(ctx, "No URL found associated with %s on %s\n", title, url)
		} else if strings.ToUpper(title) == "STEP FILE" || strings.ToUpper(title) == "ONSHAPE MODEL LINK" {
			result[strings.ToUpper((title))] = spiderdata.DownloadEnt{URL: dlurl, Used: false}
		} else if strings.HasSuffix(strings.ToUpper(title), ".STEP") ||
			strings.HasSuffix(strings.ToUpper(title), ".STEP.ZIP") ||
			strings.HasSuffix(strings.ToUpper(title), ".STP") ||
			strings.HasSuffix(strings.ToUpper(title), ".STL") ||
			strings.HasSuffix(strings.ToUpper(title), ".SLDDRW") {
			m := regexp.MustCompile(`[ \.].*$`)
			title = m.ReplaceAllString(title, "")
			result[title] = spiderdata.DownloadEnt{URL: dlurl, Used: false}
			// fmt.Printf("Save Download '%s'='%s'\n", title, dlurl)
		}
	})
	return result
}

// --------------------------------------------------------------------------------------------
// getDownloadURL looks in the download map for a matching entry and returns the corresponding URL, marking it as used
// from the list of downloads so that we know what is left over
func getDownloadURL(_ /*ctx*/ *spiderdata.Context, sku string, downloadurls spiderdata.DownloadEntMap) (result string) {
	result = "<NOMODEL:" + sku + ">"
	ent, found := downloadurls[sku]
	if found {
		result = ent.URL
		downloadurls[sku] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
		return
	}

	ent, found = downloadurls["ONSHAPE MODEL LINK"]
	if found {
		result = ent.URL
		downloadurls[sku] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
		return

	}
	ent, found = downloadurls["STEP FILE"]
	if found {
		result = ent.URL
		downloadurls[sku] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
		return

	}

	// We didn't find the sku in the list, but it is possible that they misnamed it.
	// For example https://www.servocity.com/8mm-4-barrel  has a SKU of 545314
	// But the text for the URL is mistyped as '535314' but it links to 'https://www.servocity.com/media/attachment/file/5/4/545314.zip'
	// So we want to try to use it
	for key, element := range downloadurls {
		if !element.Used && strings.Contains(element.URL, sku) {
			result = element.URL
			downloadurls[key] = spiderdata.DownloadEnt{URL: ent.URL, Used: true}
		}
	}
	return
}

func processProductBrowse(ctx *spiderdata.Context, productname string, _ /*url*/ string, product *goquery.Selection) (found bool) {
	found = false
	spiderdata.OutputCategory(ctx, productname, true)
	product.Find("div.product-summary a.product-summary__media-link").Each(func(i int, linked *goquery.Selection) {
		impression, hassimpression := linked.Attr("data-analytics-product-impression")
		itemurl, hasurl := linked.Attr("href")
		if hassimpression && hasurl {
			var keys map[string]interface{}
			json.Unmarshal([]byte(impression), &keys)
			name := keys["name"]
			sku := keys["sku"]
			fmt.Printf(" Browse: name '%v' sku '%v' url '%v'\n", name, sku, itemurl)
			if !ctx.G.SingleOnly {
				spiderdata.EnqueURL(ctx, itemurl, productname)
			}
			found = true
		}
	})
	return
}

func processProductDetail(ctx *spiderdata.Context, breadcrumbs string, url string, product *goquery.Selection) (found bool) {
	found = false

	spiderdata.OutputCategory(ctx, breadcrumbs, true)
	downloadurls := findAllDownloads(ctx, url, product)

	localproductname := product.Find("div.product-name").Text()

	// See if this has a set of options
	productVariants := product.Find("div.product-variant-list div.product-variant-line")
	if productVariants.Length() > 0 {
		productVariants.Each(func(i int, variant *goquery.Selection) {
			variantName := variant.Find("div.variant-name").Text()
			variantSKU := variant.Find("div.manufacturer-part-number span.value").Text()
			spiderdata.OutputProduct(ctx, localproductname+" - "+variantName, variantSKU, url, getDownloadURL(ctx, variantSKU, downloadurls), false, nil)
		})
		found = true
		return
	}
	productForm := product.Find("#product-details-form")
	productForm.Each(func(i int, formElem *goquery.Selection) {
		sku := formElem.Find("div.manufacturer-part-number span.value").Text()
		spiderdata.OutputProduct(ctx, localproductname, sku, url, getDownloadURL(ctx, sku, downloadurls), false, nil)
		found = true
	})
	return
}

func processProductSelection(ctx *spiderdata.Context, productname string, url string, product *goquery.Selection) (found bool) {
	found = false
	spiderdata.OutputCategory(ctx, productname, true)
	localname := product.Find("h1.product-details__heading").Text()
	// fmt.Printf("ProcessProductSelection\n")
	changeset := product.Find("div.select-menu select")

	downloadurls := findAllDownloads(ctx, url, product)
	if changeset.Children().Length() > 0 {
		changeset.Find("option").Each(func(i int, option *goquery.Selection) {
			value, hasval := option.Attr("value")
			if hasval {
				/// The value generally is of the form:  descr (sku)
				// So we want to split on the left paren
				pos := strings.Index(value, " (")
				if pos >= 0 {
					namepart := value[:pos-1]
					sku := value[pos+2 : len(value)-1]
					spiderdata.OutputProduct(ctx, localname+" "+namepart, sku, url, getDownloadURL(ctx, sku, downloadurls), false, nil)
					found = true
				}
			}
		})
	}
	return
}

// Recursive function to print the HTML tree
func printHTMLTree(selection *goquery.Selection, indent int) {
	// Print the current node
	selection.Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)

		// Indentation for visualizing hierarchy
		fmt.Println(strings.Repeat("  ", indent), "<", node.Data, ">")

		// Print attributes
		for _, attr := range node.Attr {
			fmt.Printf("%s  - %s=\"%s\"\n", strings.Repeat("  ", indent+1), attr.Key, attr.Val)
		}

		// Recursively print children
		s.Children().Each(func(j int, child *goquery.Selection) {
			printHTMLTree(child, indent+1)
		})
	})
}

// getBreadCrumbName returns the breadcrumb associated with a document
// A typical one looks like this:
//
// <div class="breadcrumb">
// <ul itemscope itemtype="http://schema.org/BreadcrumbList">
//
//	<li>
//		<span>
//			<a href="/">
//				<span>Home</span>
//			</a>
//		</span>
//		<span class="delimiter">/</span>
//	<li itemprop="itemListElement" itemscope itemtype="http://schema.org/ListItem">
//		<a href="/first-robotics" itemprop="item">
//			<span itemprop="name">FIRST Robotics</span>
//		</a>
//		<span class="delimiter">/</span>
//		<meta itemprop="position" content="1">
//		<li itemprop="itemListElement" itemscope itemtype="http://schema.org/ListItem">
//			<a href="/first-tech-challenge" itemprop="item">
//				<span itemprop="name">FTC</span>
//			</a>
//			<span class="delimiter">/</span>
//			<meta itemprop="position" content="2">
//			<li itemprop="itemListElement" itemscope itemtype="http://schema.org/ListItem">
//				<strong class="current-item" itemprop="name">12V 3000mAh NiMH Battery Pack PP45 ARES</strong>
//				<span itemprop="item" itemscope itemtype="http://schema.org/Thing" id="/studica-robotics-brand/12v-3000mah-nimh-battery-pack-pp45-ares"></span>
//				<meta itemprop="position" content="3">
//
// </ul>
// </div>
//
// What we want to get is the name (the sections in the <a> or the <strong>) while building up a database of matches to
// the category since their website seems to put a unique category for each
func getBreadCrumbName(ctx *spiderdata.Context, url string, bc *goquery.Selection) string {
	result := ""
	prevresult := ""
	fmt.Printf("%v children\n", bc.Children().Length())
	printHTMLTree(bc, 0)
	// bc.Find("li[itemprop]").Each(func(i int, li *goquery.Selection) {
	bc.Find("li[itemprop]").Each(func(i int, li *goquery.Selection) {
		name := ""
		url := ""
		// See if we have an <a> under the section
		li.Find("a[itemprop='item']").Each(func(i int, a *goquery.Selection) {
			name = a.Text()
			urlloc, hasurl := a.Attr("href")
			if hasurl {
				url = urlloc
			}
		})
		li.Find("span[itemprop='name]").Each(func(i int, span *goquery.Selection) {
			name = span.Text()
		})
		li.Find("strong[itemprop='name]").Each(func(i int, strong *goquery.Selection) {
			name = strong.Text()
		})
		name = strings.TrimSpace(name)
		spiderdata.SaveCategory(ctx, name, name, url)

		if name != "Studica Robotics" {
			prevresult = result
			result = spiderdata.MakeBreadCrumb(ctx, result, name)
		}
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

// func CacheNav(ctx *spiderdata.Context, nav *goquery.Selection) {
// 	toplevel := nav.Find("ul.mega-menu-responsive li")
// 	toplevel.Each(func(i int, li *goquery.Selection) {
// 		alink := li.Find("a")
// 		navtitle := ""
// 		if alink.Length() != 1 {
// 			fmt.Printf("+++Looking at top level link expected 1 entry found %v\n", alink.Length())
// 		}
// 		href, hashref := alink.Attr(("href"))
// 		if hashref {
// 			title := alink.Find("span")
// 			if title.Length() != 1 {
// 				fmt.Printf("+++Looking at title for '%v' expected 1 link text found %v\n", href, title.Length())
// 			}
// 			navtitle = title.Text()
// 			// We have a title and a URL, so output it and then find the children
// 			////TEMP		spiderdata.EnqueURL(ctx, href, navtitle)
// 		}
// 		_, skipItem := skipMenus[navtitle]
// 		if skipItem {
// 			// fmt.Printf("Skipping %v\n", navtitle)
// 		} else {
// 			// fmt.Printf("Caching '%v'\n", navtitle)
// 			if !ctx.G.SingleOnly {
// 				spiderdata.EnqueURL(ctx, href, navtitle)
// 			}
// 		}
// 	})
// }

func cacheTagLinks(ctx *spiderdata.Context, ptpDiv *goquery.Selection) (found bool) {
	found = false
	if !ctx.G.SingleOnly {
		ptpDiv.Find(".product-title a").Each(func(i int, url *goquery.Selection) {
			urlhref, hashref := url.Attr("href")
			if hashref {
				found = true
				spiderdata.EnqueURL(ctx, urlhref, "")
			}
		})
	}
	return
}

func CacheSiteURLs(ctx *spiderdata.Context, urlset *goquery.Selection) {
	urllocs := urlset.Find("url loc")
	urllocs.Each(func(i int, loc *goquery.Selection) {
		if !ctx.G.SingleOnly {
			spiderdata.EnqueURL(ctx, loc.Text(), "")
		}
	})
}

var fixMenus = map[string]string{"Bundles > All Bundles": "Bundles"}

func CacheNavMenu(ctx *spiderdata.Context, navtitle string, l2menu *goquery.Selection) {
	// Find the span a in the block
	l2titles := l2menu.Find("span a")
	l2titles.Each(func(i int, l2title *goquery.Selection) {
		l2href, hashref := l2title.Attr("href")
		processl3 := true
		l2text := strings.TrimSpace(l2title.Text())
		if l2text == "" {
			alttext, hasalt := l2title.Find("img").Attr("alt")
			if hasalt {
				processl3 = false
				l2text = strings.TrimSpace(alttext)
			}
		}

		l2titletext := navtitle + " > " + l2text
		if hashref {
			// fmt.Printf("L2 Found %v at %v\n", l2titletext, l2href)
			if l2href != "" {
				if !ctx.G.SingleOnly {
					spiderdata.EnqueURL(ctx, l2href, l2titletext)
				}
				// If we are doing the FIRST stuff, we only need to get the top level
				if processl3 {
					// We have the second level title, so we need to iterate over all the children
					l2title.Parent().Parent().Find("ul li a").Each(func(i int, l3child *goquery.Selection) {
						l3href, hashref := l3child.Attr("href")
						l3text := strings.TrimSpace(l3child.Text())
						l3titletext := l2titletext + " > " + l3text
						fixtitle, dofix := fixMenus[l3titletext]
						if dofix {
							l3titletext = fixtitle
						}
						if hashref {
							// fmt.Printf("L3 Found %v at %v\n", l3titletext, l3href)
							if !ctx.G.SingleOnly {
								spiderdata.EnqueURL(ctx, l3href, l3titletext)
							}
						}
					})
				}
			}
		}
	})
}

// ParseStudicaPage parses a page and adds links to elements found within by the various processors
func ParseStudicaPage(ctx *spiderdata.Context, doc *goquery.Document) {
	ctx.G.Mu.Lock()
	url := ctx.Url
	found := false

	// When debugging the single page case, dump the DOM to make it easier to figure out what we are missing
	if ctx.G.SingleOnly {
		printHTMLTree(doc.Children(), 0)
	}
	// Find the breadcrumbs so we know the catagory of the product(s)
	bcloc := doc.Find("ul[itemtype=\"http://schema.org/BreadcrumbList\"]")
	breadcrumbs := getBreadCrumbName(ctx, url, bcloc)

	// Remember that we have been here so that we can mark it as complete
	spiderdata.MarkVisitedURL(ctx, url, breadcrumbs)

	// TODO: see if this has been discontinued
	// isDiscontinued := (doc.Find("p.discontinued").Length() > 0)

	// See if this is the main site map page
	sitemap := doc.Find("urlset")
	sitemap.Each(func(i int, urlset *goquery.Selection) {
		CacheSiteURLs(ctx, urlset)
		found = true
	})

	// TODO: See if this is actually needed
	// // Cache any menu navigation links
	// primaryNav := doc.Find("div.header-menu")
	// primaryNav.Each(func(i int, nav *goquery.Selection) {
	// 	CacheNav(ctx, nav)
	// })

	// TODO: See if this is needed
	// if !found {
	// 	// See if this is a menu navigation page
	// 	if strings.Contains(url, menuPrefix) {
	// 		navtitle, foundbc := ctx.G.BreadcrumbMap[url]
	// 		if !foundbc {
	// 			navtitle = "XXX-" + url + "-XXX"
	// 		}
	// 		l2menus := doc.Find("div.taxonomy-content-block")
	// 		l2menus.Each(func(i int, nav *goquery.Selection) {
	// 			CacheNavMenu(ctx, navtitle, nav)
	// 			found = true
	// 		})
	// 	}
	// }
	if !found {
		// Enqueue any related products
		if !ctx.G.SingleOnly {
			doc.Find("div.related-products-grid a").Each(func(i int, a *goquery.Selection) {
				url, foundurl := a.Attr("href")
				if foundurl {
					spiderdata.EnqueURL(ctx, url, "")
				}
			})
		}
	}

	if !found {
		// see if we have a product-tag-page which is just a list of links to other pages
		ptp := doc.Find("div.product-tag-page")
		if ptp.Length() > 0 {
			tagpageEmpty := true
			ptp.Each(func(i int, ptpDiv *goquery.Selection) {
				if cacheTagLinks(ctx, ptpDiv) {
					tagpageEmpty = false
				}
			})
			if tagpageEmpty {
				spiderdata.OutputError(ctx, "***Product Tag Page Empty: %s\n", url)
			}
			found = true
		}

	}

	if !found {
		doc.Find("div.product-details--option_selects").Each(func(i int, productselect *goquery.Selection) {
			// fmt.Printf("Found Product Details Selection\n")
			if processProductSelection(ctx, breadcrumbs, url, productselect) {
				found = true
			}
		})
	}

	if !found {
		doc.Find("div.product-browse").Each(func(i int, productbrowse *goquery.Selection) {
			// fmt.Printf("Found Product Browse\n")
			if processProductBrowse(ctx, breadcrumbs, url, productbrowse) {
				found = true
			}
		})
	}

	if !found {
		doc.Find("div.product-details-page").Each(func(i int, product *goquery.Selection) {
			// fmt.Printf("Found Product Detail Container")
			if processProductDetail(ctx, breadcrumbs, url, product) {
				found = true
			}
		})
	}

	if !found {
		doc.Find("div.category-page").Each(func(i int, category *goquery.Selection) {
			if !ctx.G.SingleOnly {
				category.Find("div.product-item .product-title a").Each(func(i int, a *goquery.Selection) {
					url, foundurl := a.Attr("href")
					if foundurl {
						spiderdata.EnqueURL(ctx, url, "")
					}
				})
			}
			found = true
		})
	}

	if !found {
		doc.Find("a.category-summary-content-block__content").Each(func(i int, a *goquery.Selection) {
			url, foundurl := a.Attr("href")
			if foundurl {
				// Now we need to compute better breadcrumbs for the link
				a.Find("span.category-summary-content-block__heading").Each(func(i int, catname *goquery.Selection) {
					catcrumb := spiderdata.MakeBreadCrumb(ctx, breadcrumbs, catname.Text())
					if !ctx.G.SingleOnly {
						spiderdata.EnqueURL(ctx, url, catcrumb)
					}
					found = true
				})
			}
		})
	}

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
		if url != "https://www.studica.com/" {
			spiderdata.OutputError(ctx, "Unable to process: %s\n", url)
		}
	}
	ctx.G.Mu.Unlock()
}
