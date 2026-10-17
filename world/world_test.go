package world

import (
	"testing"

	"github.com/ajkula/ajkurig/rng"
)

// ===== Constants =====

const (
	testGrass MaterialID = iota
	testFire
	testAsh
	testWater
	testIce
)

const (
	testHeat StimulusID = FirstGameStimulus + iota
	testChill
	testStimulusEnd
)

const (
	testStatusBurning StatusMask = 1 << iota
	testStatusWet
	testStatusChilled
)

const (
	testElementNone ElementID = iota
	testElementFire
	testElementFrost
	testElementWater
)

const (
	testReactionNone ReactionID = iota
	testReactionSteam
	testReactionFreeze
)

var testMaterials = []Material{
	testGrass: {Name: "Grass", BaseColor: [3]uint8{46, 86, 44}, ColorVariation: 16},
	testFire:  {Name: "Fire", BaseColor: [3]uint8{250, 110, 25}, ColorVariation: 70, LifetimeMinTicks: 25, LifetimeMaxTicks: 50, Emits: testHeat, Glows: true},
	testAsh:   {Name: "Ash", BaseColor: [3]uint8{54, 52, 52}, ColorVariation: 12, LifetimeMinTicks: 200, LifetimeMaxTicks: 400},
	testWater: {Name: "Water", BaseColor: [3]uint8{36, 84, 165}, ColorVariation: 16},
	testIce:   {Name: "Ice", BaseColor: [3]uint8{165, 210, 240}, ColorVariation: 18, LifetimeMinTicks: 150, LifetimeMaxTicks: 250},
}

var testRules = []MaterialRule{
	{testHeat, testGrass, testFire, 36},
	{testHeat, testIce, testWater, 500},
	{testChill, testWater, testIce, 1024},
	{testChill, testFire, testAsh, 1024},
	{StimulusExpire, testFire, testAsh, 1024},
	{StimulusExpire, testAsh, testGrass, 1024},
	{StimulusExpire, testIce, testWater, 1024},
}

// ===== Public API =====

func TestFireSpreadsBurnsToAshAndGrassGrowsBack(t *testing.T) {
	ground := newTestGround(1)
	ground.PaintCircle(32, 32, 2, testFire)
	burned, peakAsh := 0, 0
	for range 1200 {
		ground.Tick()
		peakAsh = max(peakAsh, ground.CountMaterial(testAsh))
		burned = max(burned, ground.CountMaterial(testFire)+ground.CountMaterial(testAsh))
	}
	if burned <= 13 || peakAsh == 0 {
		t.Fatalf("fire did not spread: %d burning or burnt cells, peak ash %d", burned, peakAsh)
	}
	if ground.CountMaterial(testFire) != 0 || ground.CountMaterial(testAsh) != 0 {
		t.Fatalf("fire or ash left after 1200 ticks: %d fire, %d ash", ground.CountMaterial(testFire), ground.CountMaterial(testAsh))
	}
}

func TestHeatMakesFireSpreadFurther(t *testing.T) {
	normal, hot := 0, 0
	for seed := range uint32(8) {
		normal += burnedCells(newTestGround(seed+1), 1)
		hot += burnedCells(newTestGround(seed+1), 3)
	}
	if hot <= normal*3/2 {
		t.Fatalf("a hot world burned %d cells, a normal one %d: heat should spread fire much further", hot, normal)
	}
	t.Logf("burned cells over 8 seeds: normal %d, hot %d", normal, hot)
}

func TestOptimizedTickAndPixelsMatchTheFullScan(t *testing.T) {
	optimized, reference := newTestGround(7), newTestGround(7)
	for _, ground := range []*Ground{optimized, reference} {
		ground.SpreadMultiplier = 2.5
		ground.PaintCircle(20, 20, 3, testFire)
		ground.PaintCircle(40, 40, 4, testWater)
	}
	for frame := range uint32(900) {
		optimized.RefreshPixels(frame)
		fullRefreshPixels(reference, frame)
		if string(optimized.Pixels) != string(reference.Pixels) {
			t.Fatalf("pixels differ at frame %d", frame)
		}
		if frame%6 != 0 {
			continue
		}
		optimized.Tick()
		fullScanTick(reference)
		optimized.StimulateArea(320, 320, 2, testChill)
		reference.StimulateArea(320, 320, 2, testChill)
	}
	if string(optimized.cells) != string(reference.cells) {
		t.Fatal("cells differ after 150 ticks")
	}
}

func TestChillFreezesWater(t *testing.T) {
	ground := newTestGround(3)
	ground.PaintCircle(10, 10, 3, testWater)
	ground.StimulateArea(10*8+4, 10*8+4, 3, testChill)
	if ground.MaterialAtCell(10, 10) != testIce {
		t.Fatalf("water at the center did not freeze")
	}
}

func TestReactionsFollowRulesAndLevels(t *testing.T) {
	chemistry := newTestChemistry()
	store := NewStatusStore(chemistry, 4)
	store.Apply(0, testStatusBurning, 0)
	if reaction := store.FindReaction(0, testElementFrost); reaction != testReactionSteam {
		t.Fatalf("frost on burning gave reaction %d, want steam", reaction)
	}
	store.Apply(1, testStatusChilled, 0)
	if reaction := store.FindReaction(1, testElementFrost); reaction != testReactionNone {
		t.Fatalf("freeze triggered at chill level 1, want level 2")
	}
	store.Apply(1, testStatusChilled, 0)
	if reaction := store.FindReaction(1, testElementFrost); reaction != testReactionFreeze {
		t.Fatalf("freeze did not trigger at chill level 2")
	}
	store.Apply(2, testStatusBurning, testStatusBurning)
	if store.Status[2] != 0 {
		t.Fatalf("an immune entity caught a status")
	}
}

func TestStatusesStackDamageSlowAndExpire(t *testing.T) {
	store := NewStatusStore(newTestChemistry(), 1)
	for range 3 {
		store.Apply(0, testStatusBurning|testStatusChilled, 0)
	}
	if store.Level(0, StatusIndex(testStatusBurning)) != 3 || store.SpeedFactor(0) > 0.35 {
		t.Fatalf("level %d speed %.2f after three hits", store.Level(0, StatusIndex(testStatusBurning)), store.SpeedFactor(0))
	}
	damage := float32(0)
	for range 240 {
		damage += store.Tick(0, 1.0/60)
	}
	if damage < 50 || store.Status[0] != 0 {
		t.Fatalf("damage %.1f status %b after 4 s, want about 54 damage and no status", damage, store.Status[0])
	}
}

func TestWeatherStartsFadesAndEnds(t *testing.T) {
	random := rng.New(5)
	definitions := []WeatherDefinition{{Key: "clear"}, {Key: "heatwave", Temperature: 1, SpreadMultiplier: 3, GroundStimulus: testHeat, GroundCellsPerTick: 30}}
	weather := NewWeather(definitions, WeatherTiming{CalmMinimumSeconds: 1, CalmMaximumSeconds: 1, ActiveSeconds: 10, FadeSeconds: 2, GroundRadiusCells: 20, FlashSeconds: 0.3}, &random)
	started, ended, peakTemperature := false, false, float32(0)
	for range 60 * 15 {
		events := weather.Update(1.0/60, &random)
		started = started || events.HasStart
		ended = ended || events.Ended
		peakTemperature = max(peakTemperature, weather.Temperature())
	}
	if !started || !ended || peakTemperature < 0.99 {
		t.Fatalf("started %v ended %v peak temperature %.2f", started, ended, peakTemperature)
	}
}

func BenchmarkGroundTick(b *testing.B) {
	ground := NewGround(GroundSettings{
		Materials: testMaterials, Rules: testRules, StimulusCount: int(testStimulusEnd),
		CellSize: 16, Columns: 256, Rows: 256, Seed: 9,
		Generate: func(column, row int, seed uint32) MaterialID { return testGrass },
	})
	ground.PaintCircle(128, 128, 20, testFire)
	for b.Loop() {
		ground.Tick()
	}
}

func BenchmarkGroundRefreshPixels(b *testing.B) {
	ground := NewGround(GroundSettings{
		Materials: testMaterials, Rules: testRules, StimulusCount: int(testStimulusEnd),
		CellSize: 16, Columns: 256, Rows: 256, Seed: 9,
		Generate: func(column, row int, seed uint32) MaterialID { return testGrass },
	})
	ground.PaintCircle(128, 128, 20, testFire)
	ground.RefreshPixels(0)
	frame := uint32(0)
	for b.Loop() {
		frame++
		ground.RefreshPixels(frame)
	}
}

// ===== Internal =====

func newTestGround(seed uint32) *Ground {
	return NewGround(GroundSettings{
		Materials: testMaterials, Rules: testRules, StimulusCount: int(testStimulusEnd),
		CellSize: 8, Columns: 64, Rows: 64, Seed: seed,
		Generate: func(column, row int, seed uint32) MaterialID { return testGrass },
	})
}

func fullScanTick(ground *Ground) {
	ground.tick++
	for index := range ground.cells {
		ground.updateCell(index)
	}
}

func fullRefreshPixels(ground *Ground, clock uint32) {
	for index, material := range ground.cells {
		ground.writePixel(index, material, clock)
	}
}

func burnedCells(ground *Ground, spread float32) int {
	ground.SpreadMultiplier = spread
	ground.PaintCircle(32, 32, 1, testFire)
	burned := 0
	for range 120 {
		ground.Tick()
		burned = max(burned, ground.CountMaterial(testFire)+ground.CountMaterial(testAsh))
	}
	return burned
}

func newTestChemistry() *Chemistry {
	return NewChemistry(ChemistrySettings{
		Statuses: []StatusDefinition{
			{Name: "Burning", DurationSeconds: 3, DamagePerSecond: 6, MaximumLevel: 3, SpeedFactor: 1, TintStrength: 0.85},
			{Name: "Wet", DurationSeconds: 5, MaximumLevel: 1, SpeedFactor: 0.9, TintStrength: 0.45},
			{Name: "Chilled", DurationSeconds: 3, MaximumLevel: 3, SpeedFactor: 0.7, TintStrength: 0.75},
		},
		ElementStatus: []StatusMask{testElementFire: testStatusBurning, testElementFrost: testStatusChilled, testElementWater: testStatusWet},
		Rules: []ReactionRule{
			{testElementFrost, testStatusBurning, testReactionSteam, 0},
			{testElementFrost, testStatusWet, testReactionFreeze, 0},
			{testElementFrost, testStatusChilled, testReactionFreeze, 2},
		},
	})
}
