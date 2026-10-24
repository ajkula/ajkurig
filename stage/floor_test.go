package stage

import (
	"testing"

	"github.com/ajkula/ajkurig/mathx"
)

// ===== Helpers =====

var tilted = Floor{FarY: 400, NearY: 700, FarScale: 3, NearScale: 4.5, VanishX: 640}

func isNear(a, b float32) bool {
	return mathx.Abs(a-b) < 1e-3
}

// ===== Tests =====

func TestTheNearEdgeIsDrawnAsIs(t *testing.T) {
	spot := tilted.Project(100, tilted.NearY, 0)
	if !isNear(spot.X, 100) || !isNear(spot.Y, tilted.NearY) || !isNear(spot.Scale, tilted.NearScale) {
		t.Fatalf("near edge projected to %+v", spot)
	}
}

func TestTheFarEdgeShrinksTowardTheVanishingPoint(t *testing.T) {
	spot := tilted.Project(100, tilted.FarY, 0)
	wantedX := tilted.VanishX + (100-tilted.VanishX)*tilted.FarScale/tilted.NearScale
	if !isNear(spot.X, wantedX) || !isNear(spot.Y, tilted.FarY) || !isNear(spot.Scale, tilted.FarScale) {
		t.Fatalf("far edge projected to %+v, wanted x %v", spot, wantedX)
	}
}

func TestTheMiddleOfTheFloorLiesFartherThanHalfway(t *testing.T) {
	middle := tilted.Project(tilted.VanishX, (tilted.FarY+tilted.NearY)/2, 0)
	isBetween := middle.Scale > tilted.FarScale && middle.Scale < (tilted.FarScale+tilted.NearScale)/2
	if !isBetween || middle.Y >= (tilted.FarY+tilted.NearY)/2 {
		t.Fatalf("perspective should squeeze the far half, got %+v", middle)
	}
}

func TestHeightShrinksWithDistance(t *testing.T) {
	near := tilted.Project(0, tilted.NearY, 0).Y - tilted.Project(0, tilted.NearY, 50).Y
	far := tilted.Project(0, tilted.FarY, 0).Y - tilted.Project(0, tilted.FarY, 50).Y
	if !isNear(near, 50) || !isNear(far, 50*tilted.FarScale/tilted.NearScale) {
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
	x := tilted.Unproject(0, y)
	if spot := tilted.Project(x, y, 0); !isNear(spot.X, 0) {
		t.Fatalf("world column %v is drawn at %v", x, spot.X)
	}
}
