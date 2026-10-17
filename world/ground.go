package world

import (
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rng"
)

// ===== Types =====

type MaterialID uint8

type StimulusID uint8

type Material struct {
	Name             string
	BaseColor        [3]uint8
	ColorVariation   uint8
	LifetimeMinTicks uint16
	LifetimeMaxTicks uint16
	Emits            StimulusID
	Glows            bool
	Tag              uint32
}

type MaterialRule struct {
	Stimulus      StimulusID
	From          MaterialID
	Into          MaterialID
	ChancePer1024 uint32
}

type GroundSettings struct {
	Materials     []Material
	Rules         []MaterialRule
	StimulusCount int
	CellSize      int
	Columns       int
	Rows          int
	Seed          uint32
	Generate      func(column, row int, seed uint32) MaterialID
}

type transition struct {
	into          MaterialID
	chancePer1024 uint32
}

type Ground struct {
	Columns          int
	Rows             int
	CellSize         int
	SpreadMultiplier float32
	materials        []Material
	isActive         []bool
	isGlowing        []bool
	transitions      [][]transition
	cells            []MaterialID
	lifetimes        []uint16
	shades           []uint8
	bornOnTick       []uint32
	isPending        []bool
	pendingCells     []int32
	isListedGlowing  []bool
	glowingCells     []int32
	tick             uint32
	random           rng.Random
	palette          [][ShadeLevels][4]byte
	Pixels           []byte
}

// ===== Constants =====

const (
	StimulusNone      StimulusID = 0
	StimulusExpire    StimulusID = 1
	FirstGameStimulus            = 2
	ShadeLevels                  = 16
)

var cardinalOffsets = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// ===== Public API =====

func NewGround(settings GroundSettings) *Ground {
	cellCount := settings.Columns * settings.Rows
	ground := &Ground{
		Columns:          settings.Columns,
		Rows:             settings.Rows,
		CellSize:         settings.CellSize,
		SpreadMultiplier: 1,
		materials:        settings.Materials,
		isActive:         make([]bool, len(settings.Materials)),
		isGlowing:        make([]bool, len(settings.Materials)),
		transitions:      buildTransitions(settings),
		cells:            make([]MaterialID, cellCount),
		lifetimes:        make([]uint16, cellCount),
		shades:           make([]uint8, cellCount),
		bornOnTick:       make([]uint32, cellCount),
		isPending:        make([]bool, cellCount),
		pendingCells:     make([]int32, 0, cellCount),
		isListedGlowing:  make([]bool, cellCount),
		glowingCells:     make([]int32, 0, cellCount/8),
		random:           rng.New(settings.Seed),
		palette:          buildPalette(settings.Materials),
		Pixels:           make([]byte, cellCount*4),
	}
	for material, definition := range settings.Materials {
		ground.isActive[material] = definition.Emits != StimulusNone || definition.LifetimeMaxTicks > 0
		ground.isGlowing[material] = definition.Glows
	}
	ground.Regenerate(settings.Seed, settings.Generate)
	return ground
}

func Rule(stimulus StimulusID, from, into MaterialID, chancePer1024 uint32) MaterialRule {
	return MaterialRule{Stimulus: stimulus, From: from, Into: into, ChancePer1024: chancePer1024}
}

func (g *Ground) Regenerate(seed uint32, generate func(column, row int, seed uint32) MaterialID) {
	g.random = rng.New(seed)
	g.tick = 0
	for index := range g.cells {
		g.setCell(index, generate(index%g.Columns, index/g.Columns, seed))
	}
}

func (g *Ground) Tick() {
	g.tick++
	for index, material := range g.cells {
		if !g.isActive[material] {
			continue
		}
		g.updateCell(index)
	}
}

func (g *Ground) Material(id MaterialID) *Material {
	return &g.materials[id]
}

func (g *Ground) MaterialAt(x, y float32) MaterialID {
	return g.cells[g.indexAt(x, y)]
}

func (g *Ground) MaterialAtCell(column, row int) MaterialID {
	return g.cells[row*g.Columns+column]
}

func (g *Ground) StimulateAt(x, y float32, stimulus StimulusID) {
	g.stimulate(g.indexAt(x, y), stimulus)
}

func (g *Ground) StimulateArea(x, y float32, radiusCells int, stimulus StimulusID) {
	centerColumn, centerRow := int(x)/g.CellSize, int(y)/g.CellSize
	for row := centerRow - radiusCells; row <= centerRow+radiusCells; row++ {
		for column := centerColumn - radiusCells; column <= centerColumn+radiusCells; column++ {
			isInsideCircle := (row-centerRow)*(row-centerRow)+(column-centerColumn)*(column-centerColumn) <= radiusCells*radiusCells
			g.stimulateCellIf(column, row, stimulus, isInsideCircle)
		}
	}
}

func (g *Ground) PaintCircle(centerColumn, centerRow, radiusCells int, material MaterialID) {
	for row := centerRow - radiusCells; row <= centerRow+radiusCells; row++ {
		for column := centerColumn - radiusCells; column <= centerColumn+radiusCells; column++ {
			isInside := (row-centerRow)*(row-centerRow)+(column-centerColumn)*(column-centerColumn) <= radiusCells*radiusCells
			g.paintCellIf(column, row, material, isInside)
		}
	}
}

func (g *Ground) CountMaterial(material MaterialID) int {
	count := 0
	for _, cell := range g.cells {
		count += mathx.BoolToIndex(cell == material)
	}
	return count
}

func (g *Ground) RefreshPixels(clock uint32) {
	for _, index := range g.pendingCells {
		g.isPending[index] = false
		g.writePixel(int(index), g.cells[index], clock)
	}
	g.pendingCells = g.pendingCells[:0]
	kept := g.glowingCells[:0]
	for _, index := range g.glowingCells {
		kept = mathx.AppendIf(kept, index, g.refreshGlowingCell(int(index), clock))
	}
	g.glowingCells = kept
}

func ValueNoise(seed uint32, x, y float32) float32 {
	column, row := int(mathx.Floor(x)), int(mathx.Floor(y))
	fractionX, fractionY := mathx.Smoothstep(x-float32(column)), mathx.Smoothstep(y-float32(row))
	top := mathx.Lerp(latticeValue(seed, column, row), latticeValue(seed, column+1, row), fractionX)
	bottom := mathx.Lerp(latticeValue(seed, column, row+1), latticeValue(seed, column+1, row+1), fractionX)
	return mathx.Lerp(top, bottom, fractionY)
}

// ===== Internal =====

func buildTransitions(settings GroundSettings) [][]transition {
	table := make([][]transition, max(settings.StimulusCount, FirstGameStimulus))
	for stimulus := range table {
		table[stimulus] = make([]transition, len(settings.Materials))
		for material := range table[stimulus] {
			table[stimulus][material] = transition{into: MaterialID(material)}
		}
	}
	for _, rule := range settings.Rules {
		table[rule.Stimulus][rule.From] = transition{into: rule.Into, chancePer1024: rule.ChancePer1024}
	}
	return table
}

func buildPalette(materials []Material) [][ShadeLevels][4]byte {
	palette := make([][ShadeLevels][4]byte, len(materials))
	for material := range materials {
		definition := &materials[material]
		for shade := range ShadeLevels {
			offset := (shade - ShadeLevels/2) * int(definition.ColorVariation) / (ShadeLevels / 2)
			palette[material][shade] = [4]byte{
				byte(mathx.ClampInt(int(definition.BaseColor[0])+offset, 0, 255)),
				byte(mathx.ClampInt(int(definition.BaseColor[1])+offset, 0, 255)),
				byte(mathx.ClampInt(int(definition.BaseColor[2])+offset, 0, 255)),
				255,
			}
		}
	}
	return palette
}

func (g *Ground) writePixel(index int, material MaterialID, clock uint32) {
	flicker := ((uint32(index)*2654435761 + clock*40503) >> 28) * uint32(mathx.BoolToIndex(g.isGlowing[material]))
	shade := (uint32(g.shades[index]) + flicker) % ShadeLevels
	copy(g.Pixels[index*4:], g.palette[material][shade][:])
}

func (g *Ground) refreshGlowingCell(index int, clock uint32) bool {
	material := g.cells[index]
	isStillGlowing := g.isGlowing[material]
	g.isListedGlowing[index] = isStillGlowing
	if isStillGlowing {
		g.writePixel(index, material, clock)
	}
	return isStillGlowing
}

func (g *Ground) markChanged(index int, material MaterialID) {
	isNewPending := !g.isPending[index]
	g.pendingCells = mathx.AppendIf(g.pendingCells, int32(index), isNewPending)
	g.isPending[index] = true
	isNewGlowing := g.isGlowing[material] && !g.isListedGlowing[index]
	g.glowingCells = mathx.AppendIf(g.glowingCells, int32(index), isNewGlowing)
	g.isListedGlowing[index] = g.isListedGlowing[index] || isNewGlowing
}

func (g *Ground) indexAt(x, y float32) int {
	column := mathx.ClampInt(int(x)/g.CellSize, 0, g.Columns-1)
	row := mathx.ClampInt(int(y)/g.CellSize, 0, g.Rows-1)
	return row*g.Columns + column
}

func (g *Ground) isInside(column, row int) bool {
	return column >= 0 && row >= 0 && column < g.Columns && row < g.Rows
}

func (g *Ground) stimulateCellIf(column, row int, stimulus StimulusID, shouldStimulate bool) {
	if !shouldStimulate || !g.isInside(column, row) {
		return
	}
	g.stimulate(row*g.Columns+column, stimulus)
}

func (g *Ground) paintCellIf(column, row int, material MaterialID, shouldPaint bool) {
	if !shouldPaint || !g.isInside(column, row) {
		return
	}
	g.setCell(row*g.Columns+column, material)
}

func (g *Ground) stimulate(index int, stimulus StimulusID) {
	change := g.transitions[stimulus][g.cells[index]]
	if g.random.Next()&1023 >= change.chancePer1024 {
		return
	}
	g.setCell(index, change.into)
}

func (g *Ground) setCell(index int, material MaterialID) {
	definition := &g.materials[material]
	lifetimeRange := uint32(definition.LifetimeMaxTicks-definition.LifetimeMinTicks) + 1
	g.cells[index] = material
	g.lifetimes[index] = definition.LifetimeMinTicks + uint16(g.random.Next()%lifetimeRange)
	g.shades[index] = uint8(g.random.Next() % ShadeLevels)
	g.bornOnTick[index] = g.tick
	g.markChanged(index, material)
}

func (g *Ground) updateCell(index int) {
	material := g.cells[index]
	g.spread(index, material)
	g.ageCell(index, material)
}

func (g *Ground) spread(index int, material MaterialID) {
	emitted := g.materials[material].Emits
	if emitted == StimulusNone || g.bornOnTick[index] == g.tick {
		return
	}
	attempts := int(g.SpreadMultiplier) + mathx.BoolToIndex(g.random.Float() < g.SpreadMultiplier-mathx.Floor(g.SpreadMultiplier))
	for range attempts {
		offset := cardinalOffsets[g.random.Next()&3]
		g.stimulateCellIf(index%g.Columns+offset[0], index/g.Columns+offset[1], emitted, true)
	}
}

func (g *Ground) ageCell(index int, material MaterialID) {
	if g.materials[material].LifetimeMaxTicks == 0 || g.bornOnTick[index] == g.tick {
		return
	}
	if g.lifetimes[index] == 0 {
		g.stimulate(index, StimulusExpire)
		return
	}
	g.lifetimes[index]--
}

func latticeValue(seed uint32, column, row int) float32 {
	hash := uint32(column)*374761393 + uint32(row)*668265263 + seed*2246822519
	hash = (hash ^ (hash >> 13)) * 1274126177
	return float32((hash^(hash>>16))&0xFFFF) / 0xFFFF
}
