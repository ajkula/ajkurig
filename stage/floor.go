package stage

// ===== Types =====

type Floor struct {
	FarY      float32
	NearY     float32
	FarScale  float32
	NearScale float32
	VanishX   float32
}

type Spot struct {
	X     float32
	Y     float32
	Scale float32
}

// ===== Public API =====

func (f Floor) Project(x, y, height float32) Spot {
	shrink := f.FarScale / f.NearScale
	depth := (f.NearY - y) / (f.NearY - f.FarY)
	ratio := shrink / (shrink + depth*(1-shrink))
	groundY := f.NearY - (f.NearY-f.FarY)*depth*ratio/shrink
	return Spot{X: f.VanishX + (x-f.VanishX)*ratio, Y: groundY - height*ratio, Scale: f.NearScale * ratio}
}

func (f Floor) Unproject(screenX, y float32) float32 {
	ratio := f.Project(f.VanishX, y, 0).Scale / f.NearScale
	return f.VanishX + (screenX-f.VanishX)/ratio
}
