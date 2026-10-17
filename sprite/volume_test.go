package sprite

import (
	"math"
	"slices"
	"testing"
)

// ===== Helpers =====

func squarePart(size int, value uint8) *Part {
	part := &Part{Name: "block", Pivot: Point{10, 10}, Tip: Point{10, 10}, Layer: NewLayer()}
	paintSquare(part.Layer, Point{10, 10}, size, size, value)
	return part
}

func paintSquare(layer *Layer, corner Point, width, height int, value uint8) {
	for y := corner.Y; y < corner.Y+height; y++ {
		for x := corner.X; x < corner.X+width; x++ {
			layer.Set(x, y, value)
		}
	}
}

func filledVoxels(volume *Volume) int {
	count := 0
	volume.Each(func(int, int, int, uint8) { count++ })
	return count
}

func expectVoxel(t *testing.T, volume *Volume, x, y, z int, wanted uint8, reason string) {
	t.Helper()
	if got := volume.At(x, y, z); got != wanted {
		t.Fatalf("%s: voxel %d,%d,%d should be %d, got %d", reason, x, y, z, wanted, got)
	}
}

func expectSeen(t *testing.T, volume *Volume, x, y, z int, toward [3]float64, wanted uint8, reason string) {
	t.Helper()
	if got := volume.seenColor(volume.indexOf(x, y, z), volume.faceOrder(toward)); got != wanted {
		t.Fatalf("%s: voxel %d,%d,%d should show %d, got %d", reason, x, y, z, wanted, got)
	}
}

// ===== Tests =====

func TestThreeSquareFacesCarveACubeColouredByFace(t *testing.T) {
	part := squarePart(4, 1)
	paintSquare(part.AddFace(90, 0).Layer, Point{10, 10}, 4, 4, 2)
	paintSquare(part.AddFace(0, 90).Layer, Point{10, 10}, 4, 4, 3)
	volume := part.Carve()
	if count := filledVoxels(volume); count != 64 {
		t.Fatalf("three 4 by 4 faces should carve 64 voxels, got %d", count)
	}
	expectSeen(t, volume, 10, 10, -3, [3]float64{0, 0, -1}, 1, "the front corner seen from the front shows the front drawing")
	expectSeen(t, volume, 10, 10, -3, [3]float64{-1, 0, 0}, 2, "the same corner seen from the side shows the side drawing")
	expectSeen(t, volume, 10, 10, -3, [3]float64{0, -1, 0}, 3, "the same corner seen from above shows the top drawing")
	expectVoxel(t, volume, 12, 12, -1, 1, "an inside voxel keeps the front colour")
	expectVoxel(t, volume, 12, 12, 1, Transparent, "nothing is carved beyond the side drawing")
}

func TestAShallowTopFaceCarvesAThinSlab(t *testing.T) {
	part := squarePart(4, 1)
	paintSquare(part.AddFace(0, 90).Layer, Point{10, 10}, 4, 2, 3)
	volume := part.Carve()
	if count := filledVoxels(volume); count != 32 {
		t.Fatalf("a top face two pixels deep should carve a slab of 32 voxels, got %d", count)
	}
}

func TestAPartDrawnFromTheFrontOnlyStaysOneVoxelThick(t *testing.T) {
	part := squarePart(4, 1)
	volume := part.Carve()
	if count := filledVoxels(volume); count != 16 {
		t.Fatalf("a front drawing alone should carve one voxel deep, got %d", count)
	}
	expectVoxel(t, volume, 11, 11, 0, 1, "the flat volume lies on the pivot plane")
}

func paintedCube(t *testing.T) *Volume {
	t.Helper()
	part := squarePart(4, 1)
	faces := map[uint8][2]float64{2: {90, 0}, 3: {0, 90}, 4: {180, 0}, 5: {-90, 0}, 6: {0, -90}}
	corners := map[uint8]Point{2: {10, 10}, 3: {10, 10}, 4: {7, 10}, 5: {7, 10}, 6: {10, 7}}
	for value, face := range faces {
		paintSquare(part.AddFace(face[0], face[1]).Layer, corners[value], 4, 4, value)
	}
	return part.Carve()
}

func shootCube(volume *Volume, turn, tilt float64) []uint8 {
	area := Area{X: 0, Y: 0, Width: 40, Height: 40}
	pixels, depths := make([]uint8, area.Width*area.Height), make([]float64, area.Width*area.Height)
	for index := range depths {
		depths[index] = math.Inf(1)
	}
	volume.render(yawOrientation(turn).then(tiltOrientation(tilt)), [3]float64{20.5, 20.5, 0}, cubeOutline, area, pixels, depths)
	return pixels
}

const cubeOutline = 7

func TestACubeTurnedToEachFaceShowsThatFace(t *testing.T) {
	volume := paintedCube(t)
	for value, face := range map[uint8][2]float64{1: {0, 0}, 2: {90, 0}, 3: {0, 90}, 4: {180, 0}, 5: {-90, 0}, 6: {0, -90}} {
		pixels := shootCube(volume, face[0], face[1])
		center := yawOrientation(face[0]).then(tiltOrientation(face[1])).apply([3]float64{1.5, 1.5, -1.5})
		x, y := int(math.Floor(20.5+center[0])), int(math.Floor(20.5+center[1]))
		if got := pixels[y*40+x]; got != value {
			t.Fatalf("turned to %v the cube should show colour %d, got %d", face, value, got)
		}
		for _, shown := range pixels {
			if shown != Transparent && shown != cubeOutline && shown != value {
				t.Fatalf("turned to %v the cube should show only colour %d, also shows %d", face, value, shown)
			}
		}
	}
}

func TestAQuarterTurnBetweenFacesShowsBoth(t *testing.T) {
	pixels := shootCube(paintedCube(t), 45, 0)
	if !slices.Contains(pixels, 1) || !slices.Contains(pixels, 2) {
		t.Fatal("halfway between the front and the side, both faces should show")
	}
}

func TestTheOutlineSurroundsTheSilhouette(t *testing.T) {
	pixels := shootCube(paintedCube(t), 0, 0)
	if pixels[21*40+19] != cubeOutline || pixels[21*40+24] != cubeOutline || pixels[19*40+21] != cubeOutline {
		t.Fatal("the pixels around the cube should take the outline colour")
	}
	if pixels[21*40+18] != Transparent || pixels[19*40+19] != Transparent {
		t.Fatal("the outline is one pixel wide and leaves the corners free")
	}
	for _, value := range pixels {
		if value > cubeOutline {
			t.Fatalf("a render should only use the drawing colours and the outline, got %d", value)
		}
	}
}
