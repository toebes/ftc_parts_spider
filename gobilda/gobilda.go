package gobilda

import (
	"github.com/toebes/spider_gobilda/spiderdata"
)

// GobildaTarget is the configuration structure for spidering the Gobilda website
var GobildaTarget = spiderdata.SpiderTarget{
	Outfile:       "gobilda.txt",
	SpreadsheetID: "15XT3v9O0VOmyxqXrgR8tWDyb_CRLQT5-xPfWPdbx4RM",
	Presets: []string{
		"https://www.gobilda.com/structure/",
		"https://www.gobilda.com/motion/",
		"https://www.gobilda.com/electronics/",
		"https://www.gobilda.com/hardware/",
		"https://www.gobilda.com/kits/"},
	Seed:           "https://www.gobilda.com/structure/",
	ParsePageFunc:  servocity.parseServocityPage,
	CheckMatchFunc: servocity.checkServocityMatch,
}
