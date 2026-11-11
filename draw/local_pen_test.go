package draw

import (
	"testing"

	"github.com/ajkula/ajkurig/mathx"
)

// ===== Tests =====

func TestLocalPenPlacesPointsLikeARotatedOffset(t *testing.T) {
	camera := &Camera{X: 100, Y: 40, Width: 1280, Height: 720}
	pen := NewLocalPen(nil, camera, 300, 200, 0.7, 1.5)
	for _, local := range [4][2]float32{{0, 0}, {20, 0}, {-12, 9}, {5.5, -30}} {
		offsetX, offsetY := mathx.RotateOffset(local[0]*1.5, local[1]*1.5, 0.7)
		wantX, wantY := camera.ToScreen(300+offsetX, 200+offsetY)
		x, y := pen.Point(local[0], local[1])
		if x != wantX || y != wantY {
			t.Fatalf("local %v drawn at %v,%v, want %v,%v", local, x, y, wantX, wantY)
		}
	}
}

func TestLocalPenFacesItsHeading(t *testing.T) {
	pen := NewLocalPen(nil, &Camera{}, 0, 0, mathx.Pi/2, 1)
	directionX, directionY := pen.Direction()
	if mathx.Abs(directionX) > 1e-6 || mathx.Abs(directionY-1) > 1e-6 {
		t.Fatalf("heading a quarter turn faces %v,%v", directionX, directionY)
	}
}
