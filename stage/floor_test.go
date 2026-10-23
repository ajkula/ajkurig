package stage

import (
	"testing"

	"github.com/ajkula/ajkurig/mathx"
)

// ===== Helpers =====

var arena = Floor{FarY: 400, NearY: 700, FarScale: 3, NearScale: 4.5, VanishX: 640}

func isNear(a, b float32) bool {
	return mathx.Abs(a-b) < 1e-3
}

// ===== Tests =====

func TestTheNearEdgeIsDrawnAsIs(t *testing.T) {
	spot := arena.Project(100, arena.NearY, 0)
	if !isNear(spot.X, 100) || !isNear(spot.Y, arena.NearY) || !isNear(spot.Scale, arena.NearScale) {
		t.Fatalf("near edge projected to %+v", spot)
	}
}

func TestTheFarEdgeShrinksTowardTheVanishingPoint(t *testing.T) {
	spot := arena.Project(100, arena.FarY, 0)
	wantedX := arena.VanishX + (100-arena.VanishX)*arena.FarScale/arena.NearScale
	if !isNear(spot.X, wantedX) || !isNear(spot.Y, arena.FarY) || !isNear(spot.Scale, arena.FarScale) {
		t.Fatalf("far edge projected to %+v, wanted x %v", spot, wantedX)
	}
}

func TestTheMiddleOfTheFloorLiesFartherThanHalfway(t *testing.T) {
	middle := arena.Project(arena.VanishX, (arena.FarY+arena.NearY)/2, 0)
	isBetween := middle.Scale > arena.FarScale && middle.Scale < (arena.FarScale+arena.NearScale)/2
	if !isBetween || middle.Y >= (arena.FarY+arena.NearY)/2 {
		t.Fatalf("perspective should squeeze the far half, got %+v", middle)
	}
}

func TestHeightShrinksWithDistance(t *testing.T) {
	near := arena.Project(0, arena.NearY, 0).Y - arena.Project(0, arena.NearY, 50).Y
	far := arena.Project(0, arena.FarY, 0).Y - arena.Project(0, arena.FarY, 50).Y
	if !isNear(near, 50) || !isNear(far, 50*arena.FarScale/arena.NearScale) {
		t.Fatalf("a 50 high lift is drawn %v near and %v far", near, far)
	}
}

func TestAFlatFloorKeepsEveryPoint(t *testing.T) {
	flat := Floor{FarY: 400, NearY: 700, FarScale: 4, NearScale: 4, VanishX: 640}
	spot := flat.Project(123, 456, 7)
	if !isNear(spot.X, 123) || !isNear(spot.Y, 449) || !isNear(spot.Scale, 4) {
		t.Fatalf("flat floor moved the point to %+v", spot)
	}
}

func TestUnprojectFindsTheWorldColumnUnderAScreenColumn(t *testing.T) {
	y := float32(500)
	x := arena.Unproject(0, y)
	if spot := arena.Project(x, y, 0); !isNear(spot.X, 0) {
		t.Fatalf("world column %v is drawn at %v", x, spot.X)
	}
}
