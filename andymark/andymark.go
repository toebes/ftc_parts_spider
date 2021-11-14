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
	SpreadsheetID:      "1x4SUwNaQ_X687yA6kxPELoe7ZpoCKnnCq1-OsgxUCOw",
	Presets:            []string{},
	Seed:               "https://www.andymark.com/structure/",
	ParsePageFunc:      ParseAndyMarkPage,
	CheckMatchFunc:     CheckAndyMarkMatch,
	SectionNameDeletes: []string{},
	SectionAllowedMap:  map[string]string{},
	SectionEquivalents: [][]string{},
}

const menuPrefix = "/menus/"

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

//
// <li class="primary-nav__item primary-nav__item--parent" data-primary-nav-content="5bbf4f7d61a10d68088ef08a">
// <a class="primary-nav__link" data-analytics="{&quot;event&quot;:&quot;primaryNavigationClick&quot;,&quot;domEvent&quot;:&quot;click&quot;,&quot;payload&quot;:{&quot;name&quot;:&quot;New Products&quot;,&quot;url&quot;:&quot;/categories/new&quot;}}" href="/categories/new"><span class="primary-nav__link-text">New &amp; Deals</span>
// <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" class="primary-nav__link-icon svg-icon svg-icon--tiny"><title>Link with drop down menu</title><path d="M12 18l1-.8 10.4-8.9L21.5 6 12 14.1 2.5 6 .6 8.3 11 17.2"></path></svg>

// </a><div class="primary-nav__content">
// <div class="content-block--hidden-for-small content-block content-block--html" id="content_block_5bd34acf61a10d293f964439" data-analytics="{&quot;event&quot;:&quot;contentBlockDisplay&quot;,&quot;payload&quot;:{&quot;id&quot;:&quot;5bd34acf61a10d293f964439&quot;,&quot;type&quot;:&quot;html&quot;,&quot;position&quot;:0,&quot;data&quot;:{&quot;html&quot;:&quot;\u003cdiv class=\&quot;taxonomy-content-block\&quot;\u003e\r\n\t\u003cdiv class=\&quot;taxonomy-content-block--one-column\&quot;\u003e\r\n\t\t\u003cspan class=\&quot;taxonomy-content-block__menu-heading-custom taxonomy-content-block__centered\&quot;\u003e\r\n\t\t\t\u003ca href=\&quot;/categories/new\&quot;\u003eNew Products\u003c/a\u003e\r\n\t\t\u003c/span\u003e\r\n\t\t\u003cul class=\&quot;taxonomy-content-block__menu-custom\&quot;\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/merchandise\&quot;\u003eAndyMark Merchandise\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/clearance\&quot;\u003eClearance \u0026 Discontinued\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/on-sale-now\&quot;\u003eAll On Sale Now!\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/new-products-new-for-2021\&quot;\u003eNew for 2021\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/sporting-playground-equipment\&quot;\u003eSporting \u0026 Playground Equipment\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\u003c/ul\u003e\r\n\t\u003c/div\u003e\r\n\u003c/div\u003e&quot;}}}" data-hidden-block-css-content="Block hidden at this breakpoint"><div class="html-content-block"><div class="taxonomy-content-block">
// 	<div class="taxonomy-content-block--one-column">
// 		<span class="taxonomy-content-block__menu-heading-custom taxonomy-content-block__centered">
// 			<a href="/categories/new">New Products</a>
// 		</span>
// 		<ul class="taxonomy-content-block__menu-custom">
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/merchandise">AndyMark Merchandise</a>
// 			</li>
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/clearance">Clearance &amp; Discontinued</a>
// 			</li>
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/on-sale-now">All On Sale Now!</a>
// 			</li>
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/new-products-new-for-2021">New for 2021</a>
// 			</li>
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/sporting-playground-equipment">Sporting &amp; Playground Equipment</a>
// 			</li>
// 		</ul>
// 	</div>
// </div></div>
// </div><div class="content-block--hidden-for-medium content-block--hidden-for-wide content-block--hidden-for-x-wide content-block--hidden-for-xx-wide content-block content-block--html" id="content_block_5be8e4b9fe93c67bb3df62f3" data-analytics="{&quot;event&quot;:&quot;contentBlockDisplay&quot;,&quot;payload&quot;:{&quot;id&quot;:&quot;5be8e4b9fe93c67bb3df62f3&quot;,&quot;type&quot;:&quot;html&quot;,&quot;position&quot;:1,&quot;data&quot;:{&quot;html&quot;:&quot;\u003cdiv class=\&quot;taxonomy-content-block\&quot;\u003e\r\n\t\u003cdiv class=\&quot;taxonomy-content-block--five-column\&quot;\u003e\r\n\t\t\u003cspan class=\&quot;taxonomy-content-block__menu-heading-custom taxonomy-content-block__centered\&quot;\u003e\r\n\t\t\t\u003ca href=\&quot;/categories/new\&quot;\u003eNew Products\u003c/a\u003e\r\n\t\t\u003c/span\u003e\r\n\t\t\u003cul class=\&quot;taxonomy-content-block__menu-custom\&quot;\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/merchandise\&quot;\u003eAndyMark Merchandise\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/clearance\&quot;\u003eClearance \u0026 Discontinued\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/on-sale-now\&quot;\u003eAll On Sale Now!\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/new-products-new-for-2021\&quot;\u003eNew for 2021\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/sporting-playground-equipment\&quot;\u003eSporting \u0026 Playground Equipment\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\u003c/ul\u003e\r\n\t\u003c/div\u003e\r\n\u003c/div\u003e&quot;}}}" data-hidden-block-css-content="Block hidden at this breakpoint"><div class="html-content-block"><div class="taxonomy-content-block">
// 	<div class="taxonomy-content-block--five-column">
// 		<span class="taxonomy-content-block__menu-heading-custom taxonomy-content-block__centered">
// 			<a href="/categories/new">New Products</a>
// 		</span>
// 		<ul class="taxonomy-content-block__menu-custom">
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/merchandise">AndyMark Merchandise</a>
// 			</li>
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/clearance">Clearance &amp; Discontinued</a>
// 			</li>
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/on-sale-now">All On Sale Now!</a>
// 			</li>
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/new-products-new-for-2021">New for 2021</a>
// 			</li>
// 			<li class="taxonomy-content-block__menu-item-custom">
// 				<a href="/categories/sporting-playground-equipment">Sporting &amp; Playground Equipment</a>
// 			</li>
// 		</ul>
// 	</div>
// </div></div>
// </div>
// </div></li>
//

var skipMenus = map[string]bool{"New & Deals": true, "View All": true, "Gift Card": true}

func CacheNav(ctx *spiderdata.Context, nav *goquery.Selection) {
	toplevel := nav.Find("ul li.primary-nav__item")
	toplevel.Each(func(i int, l1 *goquery.Selection) {
		navcontent, hasnavcontent := l1.Attr("data-primary-nav-content")
		alink := l1.Find("a.primary-nav__link")
		navtitle := ""
		if alink.Length() != 1 {
			fmt.Printf("+++Looking at top level link expected 1 entry found %v\n", alink.Length())
		}
		href, hashref := alink.Attr(("href"))
		if hashref {
			title := alink.Find("span.primary-nav__link-text")
			if title.Length() != 1 {
				fmt.Printf("+++Looking at title for '%v' expected 1 link text found %v\n", href, title.Length())
			}
			navtitle = title.Text()
			// We have a title and a URL, so output it and then find the children
			////TEMP		spiderdata.EnqueURL(ctx, href, navtitle)
		}
		_, skipItem := skipMenus[navtitle]
		if skipItem {
			fmt.Printf("Skipping %v\n", navtitle)
		} else {
			fmt.Printf("Caching '%v'\n", navtitle)

			if hasnavcontent {
				ctx.G.BreadcrumbMap[menuPrefix+navcontent] = navtitle
				spiderdata.EnqueURL(ctx, menuPrefix+navcontent, navtitle)
			}
		}
	})
}

///
// New & Deals  Skip
// Bundles
// Mechanical
// Electrical
// View All  - Skip
// FIRST

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
			fmt.Printf("L2 Found %v at %v\n", l2titletext, l2href)
			if l2href != "" {
				spiderdata.EnqueURL(ctx, l2href, l2titletext)
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
							fmt.Printf("L3 Found %v at %v\n", l3titletext, l3href)
							spiderdata.EnqueURL(ctx, l3href, l3titletext)
						}
					})
				}
			}
		}
	})
}

////
// <div id='navigation'>
//     <div class='primary-nav__content'>
// 	       <div class="content-block--hidden-for-small content-block content-block--html" id="content_block_5bd34acf61a10d293f964439" data-analytics="{&quot;event&quot;:&quot;contentBlockDisplay&quot;,&quot;payload&quot;:{&quot;id&quot;:&quot;5bd34acf61a10d293f964439&quot;,&quot;type&quot;:&quot;html&quot;,&quot;position&quot;:0,&quot;data&quot;:{&quot;html&quot;:&quot;\u003cdiv class=\&quot;taxonomy-content-block\&quot;\u003e\r\n\t\u003cdiv class=\&quot;taxonomy-content-block--one-column\&quot;\u003e\r\n\t\t\u003cspan class=\&quot;taxonomy-content-block__menu-heading-custom taxonomy-content-block__centered\&quot;\u003e\r\n\t\t\t\u003ca href=\&quot;/categories/new\&quot;\u003eNew Products\u003c/a\u003e\r\n\t\t\u003c/span\u003e\r\n\t\t\u003cul class=\&quot;taxonomy-content-block__menu-custom\&quot;\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/merchandise\&quot;\u003eAndyMark Merchandise\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/clearance\&quot;\u003eClearance \u0026 Discontinued\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/on-sale-now\&quot;\u003eAll On Sale Now!\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/new-products-new-for-2021\&quot;\u003eNew for 2021\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/sporting-playground-equipment\&quot;\u003eSporting \u0026 Playground Equipment\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\u003c/ul\u003e\r\n\t\u003c/div\u003e\r\n\u003c/div\u003e&quot;}}}" data-hidden-block-css-content="Block hidden at this breakpoint">
// 		       <div class='html-content-block'>
// 		        	<div class="taxonomy-content-block">
// 		        		<div class="taxonomy-content-block--one-column">
// 		        			<span class="taxonomy-content-block__menu-heading-custom taxonomy-content-block__centered">
// 		        				<a href="/categories/new">New Products</a>
// 		        			</span>
// 		        			<ul class="taxonomy-content-block__menu-custom">
// 		        				<li class="taxonomy-content-block__menu-item-custom">
// 		        					<a href="/categories/merchandise">AndyMark Merchandise</a>
// 		        				</li>
// 		        				<li class="taxonomy-content-block__menu-item-custom">
// 		        					<a href="/categories/clearance">Clearance & Discontinued</a>
// 		        				</li>
// 		        				<li class="taxonomy-content-block__menu-item-custom">
// 		        					<a href="/categories/on-sale-now">All On Sale Now!</a>
// 		        				</li>
// 		        				<li class="taxonomy-content-block__menu-item-custom">
// 		        					<a href="/categories/new-products-new-for-2021">New for 2021</a>
// 		        				</li>
// 		        				<li class="taxonomy-content-block__menu-item-custom">
// 		        					<a href="/categories/sporting-playground-equipment">Sporting & Playground Equipment</a>
// 		        				</li>
// 		        			</ul>
// 		        		</div>
// 		        	</div>
// 		        </div>
// 	</div>
// 	<div class="content-block--hidden-for-medium content-block--hidden-for-wide content-block--hidden-for-x-wide content-block--hidden-for-xx-wide content-block content-block--html" id="content_block_5be8e4b9fe93c67bb3df62f3" data-analytics="{&quot;event&quot;:&quot;contentBlockDisplay&quot;,&quot;payload&quot;:{&quot;id&quot;:&quot;5be8e4b9fe93c67bb3df62f3&quot;,&quot;type&quot;:&quot;html&quot;,&quot;position&quot;:1,&quot;data&quot;:{&quot;html&quot;:&quot;\u003cdiv class=\&quot;taxonomy-content-block\&quot;\u003e\r\n\t\u003cdiv class=\&quot;taxonomy-content-block--five-column\&quot;\u003e\r\n\t\t\u003cspan class=\&quot;taxonomy-content-block__menu-heading-custom taxonomy-content-block__centered\&quot;\u003e\r\n\t\t\t\u003ca href=\&quot;/categories/new\&quot;\u003eNew Products\u003c/a\u003e\r\n\t\t\u003c/span\u003e\r\n\t\t\u003cul class=\&quot;taxonomy-content-block__menu-custom\&quot;\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/merchandise\&quot;\u003eAndyMark Merchandise\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/clearance\&quot;\u003eClearance \u0026 Discontinued\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/on-sale-now\&quot;\u003eAll On Sale Now!\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/new-products-new-for-2021\&quot;\u003eNew for 2021\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\t\u003cli class=\&quot;taxonomy-content-block__menu-item-custom\&quot;\u003e\r\n\t\t\t\t\u003ca href=\&quot;/categories/sporting-playground-equipment\&quot;\u003eSporting \u0026 Playground Equipment\u003c/a\u003e\r\n\t\t\t\u003c/li\u003e\r\n\t\t\u003c/ul\u003e\r\n\t\u003c/div\u003e\r\n\u003c/div\u003e&quot;}}}" data-hidden-block-css-content="Block hidden at this breakpoint">
// 		<div class='html-content-block'>
// 			<div class="taxonomy-content-block">
// 				<div class="taxonomy-content-block--five-column">
// 					<span class="taxonomy-content-block__menu-heading-custom taxonomy-content-block__centered">
// 						<a href="/categories/new">New Products</a>
// 					</span>
// 					<ul class="taxonomy-content-block__menu-custom">
// 						<li class="taxonomy-content-block__menu-item-custom">
// 							<a href="/categories/merchandise">AndyMark Merchandise</a>
// 						</li>
// 						<li class="taxonomy-content-block__menu-item-custom">
// 							<a href="/categories/clearance">Clearance & Discontinued</a>
// 						</li>
// 						<li class="taxonomy-content-block__menu-item-custom">
// 							<a href="/categories/on-sale-now">All On Sale Now!</a>
// 						</li>
// 						<li class="taxonomy-content-block__menu-item-custom">
// 							<a href="/categories/new-products-new-for-2021">New for 2021</a>
// 						</li>
// 						<li class="taxonomy-content-block__menu-item-custom">
// 							<a href="/categories/sporting-playground-equipment">Sporting & Playground Equipment</a>
// 						</li>
// 					</ul>
// 				</div>
// 			</div>
// 		</div>
// 	</div>
// </div>
// ////

// ParseAndyMarkPage parses a page and adds links to elements found within by the various processors
func ParseAndyMarkPage(ctx *spiderdata.Context, doc *goquery.Document) {
	ctx.G.Mu.Lock()
	url := ctx.Cmd.URL().RawPath
	found := false
	breadcrumbs := getBreadCrumbName(ctx, url, doc.Find("ul.breadcrumbs"))
	spiderdata.MarkVisitedURL(ctx, url, breadcrumbs)

	// see if this has been discontinued
	// isDiscontinued := (doc.Find("p.discontinued").Length() > 0)

	fmt.Printf("Breadcrumb:%s\n", breadcrumbs)

	primaryNav := doc.Find("nav.primary-nav")
	primaryNav.Each(func(i int, nav *goquery.Selection) {
		CacheNav(ctx, nav)
	})

	if !found {
		// See if this is a menu navigation page
		if strings.Contains(url, menuPrefix) {
			navtitle, foundbc := ctx.G.BreadcrumbMap[url]
			if !foundbc {
				navtitle = "XXX-" + url + "-XXX"
			}
			l2menus := doc.Find("div.taxonomy-content-block")
			l2menus.Each(func(i int, nav *goquery.Selection) {
				CacheNavMenu(ctx, navtitle, nav)
				found = true
			})
		}
	}
	// doc.Find("ul.navPages-list").Each(func(i int, categoryproducts *goquery.Selection) {
	// 	fmt.Printf("Found Navigation List\n")
	// 	if processSubCategory(ctx, breadcrumbs, categoryproducts) {
	// 		found = true
	// 	}
	// })
	// if !found {
	// 	fmt.Printf("Looking for productGrid\n")
	// 	doc.Find("ul.productGrid,ul.threeColumnProductGrid,div.productTableWrapper").Each(func(i int, product *goquery.Selection) {
	// 		fmt.Printf("ProcessingProductGrid\n")
	// 		if processProductGrid(ctx, breadcrumbs, url, product) {
	// 			found = true
	// 			// } else if processProductViewWithTable(ctx, breadcrumbs, url, product) {
	// 			// 	found = true
	// 		}
	// 	})
	// }
	// if !found {
	// 	products := doc.Find("div[itemtype=\"http://schema.org/Product\"]")
	// 	products.Each(func(i int, product *goquery.Selection) {
	// 		if processProduct(ctx, breadcrumbs, url, product, isDiscontinued, products.Length() > 1) {
	// 			found = true
	// 		}
	// 	})
	// }
	// if !found {
	// 	hasOptions := doc.Find("div.available section.productView-children")
	// 	if hasOptions.Length() > 0 {
	// 		products := doc.Find("header.productView-header")

	// 		products.Each(func(i int, product *goquery.Selection) {
	// 			if processProduct(ctx, breadcrumbs, url, product.Parent(), isDiscontinued, products.Length() > 1) {
	// 				found = true
	// 			}
	// 		})
	// 	}
	// }
	// doc.Find("script").Each(func(i int, product *goquery.Selection) {
	// 	if processLazyLoad(ctx, breadcrumbs, url, product) {
	// 		found = true
	// 	}
	// })
	// if !found {
	// 	doc.Find("table.productTable").Each(func(i int, product *goquery.Selection) {
	// 		if processProductTableList(ctx, breadcrumbs, url, product) {
	// 			found = true
	// 		}
	// 	})
	// }

	// // Title is div.page-title h1
	// // Table is div.category-description div.table-widget-container table
	// if !found {
	// 	title := doc.Find("div.page-title h1")
	// 	table := doc.Find("div.category-description div.table-widget-container table")
	// 	if title.Length() > 0 && table.Length() > 0 &&
	// 		processSimpleProductTable(ctx, breadcrumbs, url, title.Text(), doc.Children(), table) {
	// 		found = true
	// 	}
	// }
	// // Look for any related products to add to the list
	// doc.Find("div.product-related a[data-card-type]").Each(func(i int, a *goquery.Selection) {
	// 	urlloc, _ := a.Attr("href")
	// 	product, _ := a.Attr("title")
	// 	fmt.Printf("**Related Found item name=%s url=%s\n", product, urlloc)
	// 	spiderdata.EnqueURL(ctx, urlloc, spiderdata.MakeBreadCrumb(ctx, breadcrumbs, product))
	// })
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
		if url != "https://www.andymark.com/" {
			spiderdata.OutputError(ctx, "Unable to process: %s\n", url)
		}
	}
	ctx.G.Mu.Unlock()
}
