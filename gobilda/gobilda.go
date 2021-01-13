package gobilda

import (
	"github.com/toebes/ftc_parts_spider/servocity"
	"github.com/toebes/ftc_parts_spider/spiderdata"
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
	ParsePageFunc:  servocity.ParseServocityPage,
	CheckMatchFunc: servocity.CheckServocityMatch,
	SectionNameDeletes: []string{
		"Shop by Electrical Connector Style > ",
		"Shop by Hub Style > ",
		" Aluminum REX Shafting >",
		" Stainless Steel D-Shafting >",
		" > Motor Mounts for AndyMark NeveRest Motors > Motor Mounts for NeveRest Orbital Gear Motors",
		" > Motor Mounts for REV Robotics Motors > Motor Mounts for REV Core Hex Motor",
		" > Motor Mounts for REV Robotics Motors > Motor Mounts for REV UltraPlanetary Gearbox",
	},
	SectionAllowedMap: map[string]string{
		"1310-0016-4012": "MOTION > Hubs > Hyper Hubs (16mm Pattern)",
		"1311-0016-1006": "MOTION > Hubs > Sonic Hubs > Thru-Hole Sonic Hubs (16mm Pattern)",
		"1309-0016-1006": "MOTION > Hubs > Sonic Hubs > Sonic Hubs (16mm Pattern)",
		"1310-0016-1006": "MOTION > Hubs > Hyper Hubs (16mm Pattern)",
		"1312-0016-1006": "MOTION > Hubs > Sonic Hubs > Double Sonic Hubs (16mm Pattern)",
		"1309-0016-0006": "MOTION > Hubs > Sonic Hubs > Sonic Hubs (16mm Pattern)",
		"1310-0016-0008": "MOTION > Hubs > Hyper Hubs (16mm Pattern)",
		"1310-0016-5008": "MOTION > Hubs > Hyper Hubs (16mm Pattern)",
		"1123-0048-0048": "STRUCTURE > Pattern Plates",
	},
	SectionEquivalents: [][]string{},
}
