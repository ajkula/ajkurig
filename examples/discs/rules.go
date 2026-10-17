package main

import (
	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/world"
)

// ===== Constants =====

const (
	MaterialGrass world.MaterialID = iota
	MaterialDirt
	MaterialAsh
	MaterialOil
	MaterialFire
	MaterialIce
	MaterialWater
)

const (
	StimulusHeat world.StimulusID = world.FirstGameStimulus + iota
	StimulusChill
	StimulusOil
	StimulusDouse
	stimulusEnd
)

const (
	StatusBurning world.StatusMask = 1 << iota
	StatusOiled
	StatusChilled
)

const (
	ElementNone world.ElementID = iota
	ElementFire
	ElementOil
	ElementFrost
)

const (
	ReactionNone world.ReactionID = iota
	ReactionInferno
	ReactionSteam
	ReactionFreeze
)

const (
	WeatherClear world.WeatherID = iota
	WeatherHeatwave
	WeatherBlizzard
	WeatherStorm
)

const (
	SignalAction  = "action"
	SignalTension = "tension"
	SignalCombo   = "combo"
	SignalDanger  = "danger"
)

var materials = []world.Material{
	MaterialGrass: {Name: "Grass", BaseColor: colorx.Bytes(colorx.Scale(colorx.Keppel, 0.32)), ColorVariation: 10},
	MaterialDirt:  {Name: "Dirt", BaseColor: colorx.Bytes(colorx.RaisinBlack), ColorVariation: 8},
	MaterialAsh:   {Name: "Ash", BaseColor: colorx.Bytes(colorx.Scale(colorx.PaleSilver, 0.25)), ColorVariation: 8, LifetimeMinTicks: 200, LifetimeMaxTicks: 400},
	MaterialOil:   {Name: "Oil", BaseColor: colorx.Bytes(colorx.Scale(colorx.BlushingPurple, 0.35)), ColorVariation: 14, LifetimeMinTicks: 600, LifetimeMaxTicks: 900, Tag: uint32(ElementOil)},
	MaterialFire:  {Name: "Fire", BaseColor: colorx.Bytes(colorx.Frostbite), ColorVariation: 60, LifetimeMinTicks: 25, LifetimeMaxTicks: 50, Emits: StimulusHeat, Glows: true, Tag: uint32(ElementFire)},
	MaterialIce:   {Name: "Ice", BaseColor: colorx.Bytes(colorx.Scale(colorx.ElectricBlue, 0.8)), ColorVariation: 14, LifetimeMinTicks: 150, LifetimeMaxTicks: 250, Tag: uint32(ElementFrost)},
	MaterialWater: {Name: "Water", BaseColor: colorx.Bytes(colorx.Scale(colorx.ElectricBlue, 0.4)), ColorVariation: 12},
}

var materialRules = []world.MaterialRule{
	world.Rule(StimulusHeat, MaterialGrass, MaterialFire, 36),
	world.Rule(StimulusHeat, MaterialOil, MaterialFire, 1024),
	world.Rule(StimulusHeat, MaterialIce, MaterialWater, 500),
	world.Rule(StimulusChill, MaterialWater, MaterialIce, 1024),
	world.Rule(StimulusChill, MaterialFire, MaterialAsh, 1024),
	world.Rule(StimulusOil, MaterialGrass, MaterialOil, 1024),
	world.Rule(StimulusOil, MaterialDirt, MaterialOil, 1024),
	world.Rule(StimulusOil, MaterialAsh, MaterialOil, 1024),
	world.Rule(StimulusDouse, MaterialFire, MaterialAsh, 1024),
	world.Rule(world.StimulusExpire, MaterialFire, MaterialAsh, 1024),
	world.Rule(world.StimulusExpire, MaterialAsh, MaterialGrass, 1024),
	world.Rule(world.StimulusExpire, MaterialIce, MaterialWater, 1024),
	world.Rule(world.StimulusExpire, MaterialOil, MaterialDirt, 1024),
}

var chemistrySettings = world.ChemistrySettings{
	Statuses: []world.StatusDefinition{
		{Name: "Burning", DurationSeconds: 3, Tint: colorx.Frostbite, PeakTint: colorx.RichLemon, AuraColor: colorx.RichLemon, TintStrength: 0.85, SpeedFactor: 1, DamagePerSecond: 6, MaximumLevel: 3},
		{Name: "Oiled", DurationSeconds: 6, Tint: colorx.RaisinBlack, TintStrength: 0.55, SpeedFactor: 0.8, MaximumLevel: 1},
		{Name: "Chilled", DurationSeconds: 3, Tint: colorx.ElectricBlue, PeakTint: colorx.PaleSilver, AuraColor: colorx.ElectricBlue, TintStrength: 0.75, SpeedFactor: 0.7, MaximumLevel: 3},
	},
	ElementStatus: []world.StatusMask{ElementFire: StatusBurning, ElementOil: StatusOiled, ElementFrost: StatusChilled},
	Rules: []world.ReactionRule{
		{Element: ElementFire, Status: StatusOiled, Reaction: ReactionInferno},
		{Element: ElementOil, Status: StatusBurning, Reaction: ReactionInferno},
		{Element: ElementFrost, Status: StatusBurning, Reaction: ReactionSteam},
		{Element: ElementFire, Status: StatusChilled, Reaction: ReactionSteam},
		{Element: ElementFrost, Status: StatusChilled, Reaction: ReactionFreeze, MinimumLevel: 3},
	},
	Priority: []world.StatusMask{StatusOiled, StatusBurning, StatusChilled},
}

func weatherDefinitions(sounds Sounds) []world.WeatherDefinition {
	return []world.WeatherDefinition{
		WeatherClear: {Key: "clear"},
		WeatherHeatwave: {
			Key: "heatwave", LabelColor: colorx.RichLemon, Multiply: colorx.Mix(colorx.White, colorx.RichLemon, 0.25), Lift: colorx.Scale(colorx.Frostbite, 0.08),
			Temperature: 1, SpreadMultiplier: 3, GroundStimulus: StimulusHeat, GroundCellsPerTick: 30, GroundInnerCells: 5, StartSound: sounds.Hiss,
			Precipitation: world.Precipitation{Style: world.StyleEmber, Count: 90, VelocityX: 15, VelocityY: -45, Size: 1.5, SizeSpread: 1.2, Sway: 10, Color: colorx.RichLemon, Alpha: 0.8},
		},
		WeatherBlizzard: {
			Key: "blizzard", LabelColor: colorx.ElectricBlue, Multiply: colorx.Mix(colorx.White, colorx.ElectricBlue, 0.3), Lift: colorx.Scale(colorx.PaleSilver, 0.15),
			Temperature: -1, GroundStimulus: StimulusChill, GroundCellsPerTick: 30, StartSound: sounds.Freeze,
			Precipitation: world.Precipitation{Style: world.StyleFlake, Count: 320, VelocityX: -50, VelocityY: 90, Size: 1.6, SizeSpread: 1.6, Sway: 18, Color: colorx.PaleSilver, Alpha: 0.85},
		},
		WeatherStorm: {
			Key: "storm", LabelColor: colorx.BlushingPurple, Multiply: colorx.Mix(colorx.White, colorx.BlushingPurple, 0.35), Lift: colorx.Scale(colorx.ElectricBlue, 0.03),
			GroundStimulus: StimulusDouse, GroundCellsPerTick: 160, StrikeChancePerSecond: 0.3, StartSound: sounds.Rain,
			Precipitation: world.Precipitation{Style: world.StyleStreak, Count: 260, VelocityX: -160, VelocityY: 1100, Size: 22, SizeSpread: 10, Color: colorx.ElectricBlue, Alpha: 0.4},
		},
	}
}

func generateTerrain(column, row int, seed uint32) world.MaterialID {
	x, y := float32(column), float32(row)
	moisture := world.ValueNoise(seed, x/22, y/22)*0.7 + world.ValueNoise(seed+7, x/7, y/7)*0.3
	oiliness := world.ValueNoise(seed+13, x/9, y/9)
	isWater := moisture < 0.24
	isDirt := moisture < 0.32 && !isWater
	isOil := oiliness > 0.87 && !isDirt && !isWater
	kinds := [4]world.MaterialID{MaterialGrass, MaterialDirt, MaterialWater, MaterialOil}
	return kinds[max(boolIndex(isDirt), boolIndex(isWater)*2, boolIndex(isOil)*3)]
}

func boolIndex(condition bool) int {
	if condition {
		return 1
	}
	return 0
}
