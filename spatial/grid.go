package spatial

// ===== Types =====

type Grid struct {
	cellSize      float32
	columns       int
	rows          int
	cellStart     []int32
	cellCursor    []int32
	cellOfEntity  []int32
	sortedIndices []int32
	nearbyCells   []int32
}

// ===== Public API =====

func NewGrid(worldWidth, worldHeight, cellSize float32, capacity int) *Grid {
	columns := int(worldWidth/cellSize) + 1
	rows := int(worldHeight/cellSize) + 1
	cellCount := columns * rows
	return &Grid{
		cellSize:      cellSize,
		columns:       columns,
		rows:          rows,
		cellStart:     make([]int32, cellCount+1),
		cellCursor:    make([]int32, cellCount+1),
		cellOfEntity:  make([]int32, capacity),
		sortedIndices: make([]int32, capacity),
		nearbyCells:   make([]int32, 0, 64),
	}
}

func (g *Grid) Rebuild(positionX, positionY []float32, count int) {
	clear(g.cellStart)
	for index := range count {
		cell := g.cellIndexAt(positionX[index], positionY[index])
		g.cellOfEntity[index] = cell
		g.cellStart[cell+1]++
	}
	for cell := 1; cell < len(g.cellStart); cell++ {
		g.cellStart[cell] += g.cellStart[cell-1]
	}
	copy(g.cellCursor, g.cellStart)
	for index := range count {
		cell := g.cellOfEntity[index]
		g.sortedIndices[g.cellCursor[cell]] = int32(index)
		g.cellCursor[cell]++
	}
}

func (g *Grid) AppendNearby(x, y, radius float32, buffer []int32) []int32 {
	for _, cell := range g.collectNearbyCells(x, y, radius) {
		buffer = append(buffer, g.sortedIndices[g.cellStart[cell]:g.cellStart[cell+1]]...)
	}
	return buffer
}

func (g *Grid) AppendNearbyLimited(x, y, radius float32, buffer []int32, limit int) []int32 {
	for _, cell := range g.collectNearbyCells(x, y, radius) {
		entities := g.sortedIndices[g.cellStart[cell]:g.cellStart[cell+1]]
		buffer = append(buffer, entities[:max(0, min(len(entities), limit-len(buffer)))]...)
	}
	return buffer
}

func IsTurn(index int, tick uint64, every int) bool {
	return (uint64(index)+tick)%uint64(max(1, every)) == 0
}

// ===== Internal =====

func (g *Grid) cellIndexAt(x, y float32) int32 {
	column := clampIndex(int(x/g.cellSize), 0, g.columns-1)
	row := clampIndex(int(y/g.cellSize), 0, g.rows-1)
	return int32(row*g.columns + column)
}

func (g *Grid) collectNearbyCells(x, y, radius float32) []int32 {
	g.nearbyCells = g.nearbyCells[:0]
	minimumColumn := clampIndex(int((x-radius)/g.cellSize), 0, g.columns-1)
	maximumColumn := clampIndex(int((x+radius)/g.cellSize), 0, g.columns-1)
	minimumRow := clampIndex(int((y-radius)/g.cellSize), 0, g.rows-1)
	maximumRow := clampIndex(int((y+radius)/g.cellSize), 0, g.rows-1)
	for row := minimumRow; row <= maximumRow; row++ {
		for column := minimumColumn; column <= maximumColumn; column++ {
			g.nearbyCells = append(g.nearbyCells, int32(row*g.columns+column))
		}
	}
	return g.nearbyCells
}

func clampIndex(value, minimum, maximum int) int {
	return max(minimum, min(maximum, value))
}
