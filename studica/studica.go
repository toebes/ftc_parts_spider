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
	StripSKU:           true,
	Seed:               "https://www.studica.com/sitemap.xml",
	ParsePageFunc:      ParseStudicaPage,
	CheckMatchFunc:     CheckStudicaMatch,
	SectionNameDeletes: []string{},
	SectionAllowedMap:  map[string]string{},
	SectionEquivalents: [][]string{},
	SkipPages: []string{
		"https://www.studica.com/cdn-cgi/l/email-protection",
		"https://www.studica.com/search",
		"https://blog.studica.com",
		"https://www.studica.com/about-us",
		"https://www.studica.com/academic-verfication",
		"https://www.studica.com/adobe-non-profit-value-incentive-plan",
		"https://www.studica.com/animation-cad-modeling",
		"https://www.studica.com/aps-ft-webinar",
		"https://www.studica.com/architecture",
		"https://www.studica.com/arduino",
		"https://www.studica.com/automation-controls",
		"https://www.studica.com/avid-2",
		"https://www.studica.com/avid-sibelius-comparison",
		"https://www.studica.com/babbel",
		"https://www.studica.com/blog",
		"https://www.studica.com/career-tech-education",
		"https://www.studica.com/classroom",
		"https://www.studica.com/clearance",
		"https://www.studica.com/cnc-machines",
		"https://www.studica.com/coding-learn-to-program",
		"https://www.studica.com/contactus",
		"https://www.studica.com/contactus-2",
		"https://www.studica.com/cookiepolicy",
		"https://www.studica.com/curriculum-solutions",
		"https://www.studica.com/digilent",
		"https://www.studica.com/dremel",
		"https://www.studica.com/drones-uav",
		"https://www.studica.com/education-pricing-babbel-for-classroom",
		"https://www.studica.com/education-webinars-for-teachers",
		"https://www.studica.com/elenco-electronics",
		"https://www.studica.com/engineering-education",
		"https://www.studica.com/first-legal-ftc-robot-parts",
		"https://www.studica.com/fischertechnik",
		"https://www.studica.com/homepagetext",
		"https://www.studica.com/ibm-spss-2",
		"https://www.studica.com/industry",
		"https://www.studica.com/kitting-services",
		"https://www.studica.com/lumion-2",
		"https://www.studica.com/manufacturer/all",
		"https://www.studica.com/maxon",
		"https://www.studica.com/national-instruments",
		"https://www.studica.com/press-releases",
		"https://www.studica.com/press-release-studica-announces-mystem-board",
		"https://www.studica.com/privacy-policy",
		"https://www.studica.com/ptc-for-schools",
		"https://www.studica.com/robert-mcneel",
		"https://www.studica.com/robotics-3",
		"https://www.studica.com/robotics-distributor-program",
		"https://www.studica.com/school-affiliates",
		"https://www.studica.com/science-education",
		"https://www.studica.com/search",
		"https://www.studica.com/siemens-stem-courses",
		"https://www.studica.com/smart-farming-challenge-robocup-germany-pr",
		"https://www.studica.com/stem-programs",
		"https://www.studica.com/students",
		"https://www.studica.com/student-software-discounts",
		"https://www.studica.com/studica-news",
		"https://www.studica.com/studica-resources",
		"https://www.studica.com/studica-robotics-resources",
		"https://www.studica.com/studica-robotics-team-discount",
		"https://www.studica.com/terms-conditions",
		"https://www.studica.com/v-ray-chaos-for-education",
		"https://www.studica.com/webinars",
		"https://www.studica.com/who-can-order",
		"https://www.studica.com/worldskills-2021-mobile-robotics-competition",

		// These are empty product tag pages
		"https://www.studica.com/1000mm",
		"https://www.studica.com/100mm-drive-wheel",
		"https://www.studica.com/100mm-flex-wheel",
		"https://www.studica.com/10mm-groove-pulley",
		"https://www.studica.com/110mm-tire",
		"https://www.studica.com/128-tooth-gear-2",
		"https://www.studica.com/135-degree",
		"https://www.studica.com/135-degree-bracket",
		"https://www.studica.com/13-tooth-bevel-gear",
		"https://www.studica.com/144mm",
		"https://www.studica.com/144mm-flat-bracket",
		"https://www.studica.com/160mm-channel",
		"https://www.studica.com/192mm",
		"https://www.studica.com/192mm-flat-bracket",
		"https://www.studica.com/192mm-u-channel-2",
		"https://www.studica.com/1mm-screw-spacer",
		"https://www.studica.com/20-tooth-timing-belt-pulley",
		"https://www.studica.com/240mm",
		"https://www.studica.com/240mm-flat-bracket",
		"https://www.studica.com/24-tooth-aluminum-sprocket",
		"https://www.studica.com/25mm-standoff",
		"https://www.studica.com/26-tooth-bevel-gear",
		"https://www.studica.com/288mm",
		"https://www.studica.com/288mm-flat-beam",
		"https://www.studica.com/288mm-flat-bracket",
		"https://www.studica.com/288mm-low-profile-u-channel",
		"https://www.studica.com/288mm-u-channel",
		"https://www.studica.com/2mm-screw-spacer",
		"https://www.studica.com/30mm-round-groove-pulley",
		"https://www.studica.com/30-tooth-bevel-gear",
		"https://www.studica.com/30-tooth-timing-belt-pulley",
		"https://www.studica.com/32mm-channel",
		"https://www.studica.com/32-toorh-gear",
		"https://www.studica.com/32-tooth-aluminum-sprocket",
		"https://www.studica.com/336mm",
		"https://www.studica.com/336mm-flat-bracket",
		"https://www.studica.com/384mm",
		"https://www.studica.com/384mm-flat-beam-2",
		"https://www.studica.com/384mm-flat-bracket",
		"https://www.studica.com/384mm-low-profile-u-channel-2",
		"https://www.studica.com/3d-robot-3-axis",
		"https://www.studica.com/3ds-max",
		"https://www.studica.com/40mm-round-groove-pulley",
		"https://www.studica.com/40-tooth-sprocket",
		"https://www.studica.com/40-tooth-timing-belt-pulley",
		"https://www.studica.com/42mm-hinge",
		"https://www.studica.com/42mm-standoff",
		"https://www.studica.com/432mm-flat-bracket",
		"https://www.studica.com/48mm-standoff",
		"https://www.studica.com/48mm-u-channel",
		"https://www.studica.com/48-tooth-sprocket",
		"https://www.studica.com/50mm-drive-wheel",
		"https://www.studica.com/50mm-round-groove-pulley",
		"https://www.studica.com/52mm-standoff",
		"https://www.studica.com/5mm-bore-pulley",
		"https://www.studica.com/5mm-hex-shaft",
		"https://www.studica.com/5mm-screw-spacer",
		"https://www.studica.com/5mm-shaft-hub",
		"https://www.studica.com/60mm-round-groove-pulley",
		"https://www.studica.com/60-tooth-timing-belt-pulley",
		"https://www.studica.com/64-tooth-gear",
		"https://www.studica.com/6mm-10-tooth-pulley",
		"https://www.studica.com/6mm-35mm-d-shaft",
		"https://www.studica.com/6mm-432mm-d-shaft",
		"https://www.studica.com/6mm-70mm-d-shaft",
		"https://www.studica.com/6mm-96mm-d-shaft",
		"https://www.studica.com/6mm-d-shaft",
		"https://www.studica.com/6mm-servo-hub",
		"https://www.studica.com/6mm-shaft-hub",
		"https://www.studica.com/75mm-drive-wheel",
		"https://www.studica.com/80-tooth-timing-belt-pulley",
		"https://www.studica.com/90-degree",
		"https://www.studica.com/90-degree-bracket",
		"https://www.studica.com/96mm-channel",
		"https://www.studica.com/96mm-flat-beam",
		"https://www.studica.com/96mm-flat-bracket",
		"https://www.studica.com/96mm-low-profile-u-channel-2",
		"https://www.studica.com/96mm-square-beam-2",
		"https://www.studica.com/96mm-t-slot-extrusion-2",
		"https://www.studica.com/96mm-u-channel",
		"https://www.studica.com/accu-set-accu-set-nimh-battery",
		"https://www.studica.com/analog-module-jst-sh-jst-gh",
		"https://www.studica.com/architectural-visualization-2",
		"https://www.studica.com/autocad",
		"https://www.studica.com/automated-systems",
		"https://www.studica.com/autonomous-driving-control-technology-analog-sensor-robotics",
		"https://www.studica.com/ball-bearing-flanged",
		"https://www.studica.com/base-plate-2",
		"https://www.studica.com/biohazard",
		"https://www.studica.com/blackhawk",
		"https://www.studica.com/bluetooh-remote-control",
		"https://www.studica.com/box-1000-sorting-components-storage",
		"https://www.studica.com/bracket-120-degree",
		"https://www.studica.com/bronze-bushing",
		"https://www.studica.com/camera",
		"https://www.studica.com/circuit",
		"https://www.studica.com/clamping-shaft-hub-2",
		"https://www.studica.com/class-sets-electrical",
		"https://www.studica.com/class-sets-gears",
		"https://www.studica.com/class-sets-optics",
		"https://www.studica.com/class-sets-solar-energy",
		"https://www.studica.com/cobra-line-follower",
		"https://www.studica.com/coding-elementary-motors-sensors",
		"https://www.studica.com/competition-robots-rgb-sensor-ultrasonic-motors",
		"https://www.studica.com/control-cylinder",
		"https://www.studica.com/conveyor-belt-training-model",
		"https://www.studica.com/creative-box-brackets-plates",
		"https://www.studica.com/creative-box-mechanics-worm-drive-chain-transmission",
		"https://www.studica.com/creative-box-storage-container-components-parts",
		"https://www.studica.com/crimp",
		"https://www.studica.com/designer-sogtware-animation",
		"https://www.studica.com/drive-base-kit",
		"https://www.studica.com/drive-systems",
		"https://www.studica.com/d-shaft-collar",
		"https://www.studica.com/dupont-cable",
		"https://www.studica.com/education-2",
		"https://www.studica.com/education-bluetooth-motors-remote-control",
		"https://www.studica.com/eld",
		"https://www.studica.com/electronic-mounting-plate",
		"https://www.studica.com/electronics-2",
		"https://www.studica.com/electronics-simple-circuits-series-parallel-connections",
		"https://www.studica.com/e-tronic-2",
		"https://www.studica.com/e-tronic-electronics-circuits",
		"https://www.studica.com/expansion-board",
		"https://www.studica.com/extrusion",
		"https://www.studica.com/factory-simulation-24v-plc-gripper-robot-high-bay-warehouse-multi-processing-sorting-line-color-detection",
		"https://www.studica.com/factory-simulation-9v-txt-gripper-robot-high-bay-warehouse-multi-processing-sorting-line-color-detection",
		"https://www.studica.com/first",
		"https://www.studica.com/flat-beam",
		"https://www.studica.com/fpv-first-person-view",
		"https://www.studica.com/frc-3",
		"https://www.studica.com/front-loader",
		"https://www.studica.com/ftc-drive-base-kit-2",
		"https://www.studica.com/ftc-starter-kit-2",
		"https://www.studica.com/fuel-cells-hydrogen-solar-renewable-energy",
		"https://www.studica.com/game-controller",
		"https://www.studica.com/gt2-330mm-timing-belt",
		"https://www.studica.com/gt2-630mm-timing-belt",
		"https://www.studica.com/gt2-810mm-timing-belt",
		"https://www.studica.com/gt2-smooth-idler-pulley",
		"https://www.studica.com/gt2-timing-belt-clamp",
		"https://www.studica.com/h2-fuel-cell-hydrogen-renewable-energy",
		"https://www.studica.com/hdmi-cable",
		"https://www.studica.com/hex-hub",
		"https://www.studica.com/hex-key-metric",
		"https://www.studica.com/high-bay-warehouse-automated",
		"https://www.studica.com/hinge",
		"https://www.studica.com/hoists",
		"https://www.studica.com/hydraulics-control-cylinder-instructional-materials-models",
		"https://www.studica.com/hydraulics-fundamentals-force-teaching-materials-control-cylinders",
		"https://www.studica.com/i2c-tjc8-cable",
		"https://www.studica.com/indexed-line-machining-stations-24v-conveyor-line-plc",
		"https://www.studica.com/indexed-line-machining-stations-9v-conveyor-line-txt-controller",
		"https://www.studica.com/inside-l-bracket",
		"https://www.studica.com/inside-u-bracket",
		"https://www.studica.com/instructional-materials",
		"https://www.studica.com/introduction-to-stem-programming-stem-computer-science",
		"https://www.studica.com/iot-internet-of-things-network-cloud",
		"https://www.studica.com/junior-collection",
		"https://www.studica.com/language-development",
		"https://www.studica.com/l-bracket",
		"https://www.studica.com/led-lights",
		"https://www.studica.com/light-weight-shaft-hub-2",
		"https://www.studica.com/low-profile-channel-pack-2",
		"https://www.studica.com/low-profile-u-channel-2",
		"https://www.studica.com/m3-10mm-button-head-cap-screw",
		"https://www.studica.com/m3-10mm-socket-head-cap-screw",
		"https://www.studica.com/m3-12mm-socket-head-cap-screw",
		"https://www.studica.com/m3-20mm-socket-head-cap-screw",
		"https://www.studica.com/m3-30mm-socket-head-cap-screw",
		"https://www.studica.com/m3-8mm-socket-head-cap-screw",
		"https://www.studica.com/m3-kep-nut",
		"https://www.studica.com/m3-nyloc-nut",
		"https://www.studica.com/m3-socket-head-cap-screw",
		"https://www.studica.com/m3-t-slot-nut",
		"https://www.studica.com/maverick",
		"https://www.studica.com/maverick-gear-motor",
		"https://www.studica.com/mechanical",
		"https://www.studica.com/mechanics-engineering-construction-dynamics-statics-gears-structure",
		"https://www.studica.com/mechanics-retro-bevel-gear-planetary-gear-scissor-lift",
		"https://www.studica.com/mechanics-static-technical-construction-set-shaft-drive-planetary-gear",
		"https://www.studica.com/mh-fc-cable",
		"https://www.studica.com/microbit-programming-sensors-actuators-instructional-activity",
		"https://www.studica.com/micro-servo-motor",
		"https://www.studica.com/mobile-robotics",
		"https://www.studica.com/motion-2",
		"https://www.studica.com/motor-driver-sensor-adapter-myrio",
		"https://www.studica.com/motor-mount-clamp-kit-2",
		"https://www.studica.com/motor-mount-plate-2",
		"https://www.studica.com/motor-mount-plate-leaf-2",
		"https://www.studica.com/motor-sensor-training-kit-myrio",
		"https://www.studica.com/motor-set-geared-motor-toothed-gears-axles-gearbox-parts",
		"https://www.studica.com/multi-mode",
		"https://www.studica.com/multi-processing-station-oven-24v-pneumatic-gripper-conveyor-plc",
		"https://www.studica.com/multi-processing-station-oven-9v-pneumatic-gripper-conveyor-txt-controller",
		"https://www.studica.com/mxp",
		"https://www.studica.com/mxp-extender-cable",
		"https://www.studica.com/mxp-extender-cable-2",
		"https://www.studica.com/mydev-protoboard",
		"https://www.studica.com/nimh-battery-pack",
		"https://www.studica.com/nimh-battery-pack-charger",
		"https://www.studica.com/omni-wheels-robotics-object-recognition",
		"https://www.studica.com/parallel-connections",
		"https://www.studica.com/phillips-hex-screwdriver",
		"https://www.studica.com/physics",
		"https://www.studica.com/pneumatics-valves-cylinders-excavator-parts",
		"https://www.studica.com/pneumatics-valves-cylinders-tree-grabber-front-loader",
		"https://www.studica.com/power-control-panel-2",
		"https://www.studica.com/powerpole-cable",
		"https://www.studica.com/powerpole-crimp-tool",
		"https://www.studica.com/power-set-plug-in-class-2-transformer",
		"https://www.studica.com/power-switch-plate-2",
		"https://www.studica.com/punching-machine-conveyor-belt-24v-plc-training-model",
		"https://www.studica.com/punching-machine-conveyor-belt-9v-txt-controller-training-model",
		"https://www.studica.com/pwm-cable",
		"https://www.studica.com/raspberry-pi-2",
		"https://www.studica.com/reen-energy-renewable-energy-solar",
		"https://www.studica.com/rhino",
		"https://www.studica.com/rhino-for-schools",
		"https://www.studica.com/rhino-upgrade",
		"https://www.studica.com/robo-pro-software",
		"https://www.studica.com/robo-pro-software-programming-coding",
		"https://www.studica.com/robot",
		"https://www.studica.com/robot-base-plate",
		"https://www.studica.com/robotics-beginner-bluetooth",
		"https://www.studica.com/robotics-industry-3-axis-grappler-activity-booklet-programming",
		"https://www.studica.com/robotics-sensor-motor-training",
		"https://www.studica.com/robotics-toolbox-2",
		"https://www.studica.com/robotics-txt-controller",
		"https://www.studica.com/series",
		"https://www.studica.com/servo-mount-flat-plate-2",
		"https://www.studica.com/servo-mount-offset-plate-2",
		"https://www.studica.com/shanghai",
		"https://www.studica.com/shock-absorber-110mm",
		"https://www.studica.com/shock-absorber-55mm",
		"https://www.studica.com/simple-circuits",
		"https://www.studica.com/simple-machines-2",
		"https://www.studica.com/simple-machines-mechanical-pulleys-hoists-gears-statics",
		"https://www.studica.com/small-thrust-ball-bearing",
		"https://www.studica.com/smart-robot",
		"https://www.studica.com/smart-robot-servo-programmer-3",
		"https://www.studica.com/smart-servo-multi-mode",
		"https://www.studica.com/smart-servo-programmer",
		"https://www.studica.com/socket-head",
		"https://www.studica.com/software-4",
		"https://www.studica.com/solar-power-renewable-energy-construction-set-instructional-activity",
		"https://www.studica.com/sorting-line-color-detection-24v-conveyor-plc",
		"https://www.studica.com/sorting-line-color-detection-9v-conveyor-txt-controller",
		"https://www.studica.com/sprocket-25-chain",
		"https://www.studica.com/spss-student-amos-grad-pack",
		"https://www.studica.com/spss-student-amos-grad-pack-v28",
		"https://www.studica.com/spss-student-faculty-pack",
		"https://www.studica.com/spss-student-faculty-pack-v28",
		"https://www.studica.com/spss-student-grad-pack-v28-base",
		"https://www.studica.com/spss-student-grad-pack-v28-premium",
		"https://www.studica.com/spss-student-grad-pack-v28-standard",
		"https://www.studica.com/spss-v29",
		"https://www.studica.com/square-beam-2",
		"https://www.studica.com/sreb-middle-school",
		"https://www.studica.com/starter-kit",
		"https://www.studica.com/starter-robot-kit",
		"https://www.studica.com/stem-electroncis-circuits-resistors-motors-teaching-material",
		"https://www.studica.com/stem-engineering-robotics-coding-automated-systems",
		"https://www.studica.com/stem-gears-lever-ratios-pulleys",
		"https://www.studica.com/stem-pneumatics-compressors-valves-cylinders",
		"https://www.studica.com/stem-prep",
		"https://www.studica.com/stem-prep-drive-systems-mechanics-physics-electronics-optics",
		"https://www.studica.com/stem-renewable-energies-power-solar-fuel-cell",
		"https://www.studica.com/storage-container",
		"https://www.studica.com/storage-travel-case",
		"https://www.studica.com/super-zoom",
		"https://www.studica.com/super-zoom-microscope",
		"https://www.studica.com/tamiya-femail-adapter-cable",
		"https://www.studica.com/t-bracket",
		"https://www.studica.com/terrain-pack",
		"https://www.studica.com/titan-quad-motor-controller-2",
		"https://www.studica.com/tool",
		"https://www.studica.com/training-bot",
		"https://www.studica.com/training-factory-industry-40-24v-plc-simulation-gripper-multi-processing-station-sorting-line-color-detection",
		"https://www.studica.com/training-factory-industry-40-9v-txt-controller-simulation-gripper-multi-processing-station-sorting-line-color-detection",
		"https://www.studica.com/training-factory-industry-40-plc-modular-training-simulation-research-teaching-production-process",
		"https://www.studica.com/training-kit",
		"https://www.studica.com/t-slot",
		"https://www.studica.com/t-slot-extrusion",
		"https://www.studica.com/txt-controller-coding-encoder-motor-ultrasonic-sensor-track-sensor",
		"https://www.studica.com/txt-control-unit-robotics-bluetooth-wifi-coding",
		"https://www.studica.com/u-channel-2",
		"https://www.studica.com/u-channel-3",
		"https://www.studica.com/ultrasonic-distance-sensor-2",
		"https://www.studica.com/ultrasonic-distance-sensor-bracket-2",
		"https://www.studica.com/unity-2",
		"https://www.studica.com/urethane-belt-joining",
		"https://www.studica.com/urethane-round-belt",
		"https://www.studica.com/usb-cable",
		"https://www.studica.com/vacuum-gripper-robot-24v-plc-3-axis-txt-controller-training-model",
		"https://www.studica.com/vacuum-gripper-robot-9v-3-axis-txt-controller-training-model",
		"https://www.studica.com/vectorworks",
		"https://www.studica.com/vision",
		"https://www.studica.com/vmx-cable",
		"https://www.studica.com/vmx-cable-pack",
		"https://www.studica.com/vmx-frc-training-bot",
		"https://www.studica.com/vmx-jst-breakout-board",
		"https://www.studica.com/vmx-robotics-controller",
		"https://www.studica.com/vmx-titan-upgrade-kit",
		"https://www.studica.com/vmx-vision-motion-frc-ftc",
		"https://www.studica.com/vmx-wallwart-cable",
		"https://www.studica.com/wheel",
		"https://www.studica.com/wire-pack",
		"https://www.studica.com/worldskills-2",
		"https://www.studica.com/worldskills-mobile-robotics-shanghai",
		"https://www.studica.com/worldskills-mobile-robotics-vmx-titan",
		"https://www.studica.com/x-bracket",
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
		if strings.HasSuffix(strings.ToUpper(dlurl), ".JPG") {
			// We are going to ignore JPG files
		} else if title == "" {
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

	bc.Find("li[itemprop]").Each(func(i int, li *goquery.Selection) {
		name := ""
		// See if we have an <a> under the section
		li.Find("a[itemprop='item']").Each(func(i int, a *goquery.Selection) {
			name = a.Text()
		})
		li.Find("[itemprop='name']").Each(func(i int, span *goquery.Selection) {
			name = span.Text()
		})
		name = strings.TrimSpace(name)

		// Don't bother gathering the Studica Robotics at the top
		if name != "Studica Robotics" {
			result = spiderdata.MakeBreadCrumb(ctx, result, name)
		}
	})
	return result
}

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
					if strings.HasSuffix(strings.ToUpper(url), ".STP") ||
						strings.Contains(strings.ToUpper(url), "CAD.ONSHAPE.COM") {
						// We just want to ignore them
					} else {
						spiderdata.EnqueURL(ctx, url, "")
					}
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
