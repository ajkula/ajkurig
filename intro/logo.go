package intro

import (
	"image"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type Vector3 [3]float32

type VoxelFace struct {
	Corners [4]Vector3
	Normal  Vector3
	IsCap   bool
}

type projectedFace struct {
	Points [4][2]float32
	Depth  float32
	Tint   colorx.RGB
}

type VoxelLogo struct {
	Faces     []VoxelFace
	Width     float32
	Height    float32
	CapColor  colorx.RGB
	SideColor colorx.RGB
	projected []projectedFace
	vertices  []ebiten.Vertex
	indices   []uint32
}

type LogoPose struct {
	CenterX  float32
	CenterY  float32
	Distance float32
	Tumble   float32
	Spin     float32
	Roll     float32
	Lift     float32
}

// ===== Constants =====

const (
	logoCanvasPadding = 4
	logoInkThreshold  = 40
	logoDepth         = 3
	LogoFocal         = 800
)

var logoLight = normalizeVector(Vector3{-0.35, -0.55, -0.75})

// ===== Public API =====

func NewVoxelLogo(message string, fontData []byte, fontSize float64, capColor, sideColor colorx.RGB) *VoxelLogo {
	cells, width, height := rasterize(message, fontData, fontSize)
	logo := &VoxelLogo{Width: float32(width), Height: float32(height), CapColor: capColor, SideColor: sideColor}
	isLit := func(x, y int) bool { return x >= 0 && y >= 0 && x < width && y < height && cells[y*width+x] }
	for y := range height {
		for x := range width {
			logo.addCellFaces(x, y, isLit)
		}
	}
	return logo
}

func (l *VoxelLogo) Draw(screen *ebiten.Image, pose LogoPose) {
	l.projected = l.projected[:0]
	for index := range l.Faces {
		l.projectFace(&l.Faces[index], pose)
	}
	slices.SortFunc(l.projected, func(a, b projectedFace) int { return compareDepth(b.Depth, a.Depth) })
	l.vertices, l.indices = l.vertices[:0], l.indices[:0]
	for index := range l.projected {
		l.queueFace(&l.projected[index])
	}
	screen.DrawTriangles32(l.vertices, l.indices, draw.SolidPixel(), &ebiten.DrawTrianglesOptions{})
}

// ===== Internal =====

func rasterize(message string, fontData []byte, fontSize float64) ([]bool, int, int) {
	parsed, err := opentype.Parse(fontData)
	if err != nil {
		return []bool{true}, 1, 1
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: fontSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return []bool{true}, 1, 1
	}
	advance := font.MeasureString(face, message).Ceil()
	metrics := face.Metrics()
	width := advance + 2*logoCanvasPadding
	height := (metrics.Ascent + metrics.Descent).Ceil() + 2*logoCanvasPadding
	canvas := image.NewGray(image.Rect(0, 0, width, height))
	drawer := &font.Drawer{Dst: canvas, Src: image.White, Face: face, Dot: fixed.P(logoCanvasPadding, logoCanvasPadding+metrics.Ascent.Ceil())}
	drawer.DrawString(message)
	return cropInk(canvas, width, height)
}

func cropInk(canvas *image.Gray, width, height int) ([]bool, int, int) {
	bounds := image.Rectangle{Min: image.Point{width, height}}
	for y := range height {
		for x := range width {
			bounds = growBoundsIf(bounds, x, y, canvas.GrayAt(x, y).Y > logoInkThreshold)
		}
	}
	cropWidth, cropHeight := max(1, bounds.Dx()), max(1, bounds.Dy())
	cells := make([]bool, cropWidth*cropHeight)
	for index := range cells {
		x, y := bounds.Min.X+index%cropWidth, bounds.Min.Y+index/cropWidth
		cells[index] = canvas.GrayAt(x, y).Y > logoInkThreshold
	}
	return cells, cropWidth, cropHeight
}

func growBoundsIf(bounds image.Rectangle, x, y int, isInk bool) image.Rectangle {
	if !isInk {
		return bounds
	}
	return image.Rect(min(bounds.Min.X, x), min(bounds.Min.Y, y), max(bounds.Max.X, x+1), max(bounds.Max.Y, y+1))
}

func (l *VoxelLogo) addCellFaces(x, y int, isLit func(x, y int) bool) {
	if !isLit(x, y) {
		return
	}
	left, top := float32(x)-l.Width/2, float32(y)-l.Height/2
	right, bottom := left+1, top+1
	front, back := float32(-logoDepth)/2, float32(logoDepth)/2
	l.Faces = append(l.Faces,
		VoxelFace{Corners: [4]Vector3{{left, top, front}, {right, top, front}, {right, bottom, front}, {left, bottom, front}}, Normal: Vector3{0, 0, -1}, IsCap: true},
		VoxelFace{Corners: [4]Vector3{{right, top, back}, {left, top, back}, {left, bottom, back}, {right, bottom, back}}, Normal: Vector3{0, 0, 1}, IsCap: true},
	)
	sides := []struct {
		IsOpen  bool
		Corners [4]Vector3
		Normal  Vector3
	}{
		{!isLit(x, y-1), [4]Vector3{{left, top, back}, {right, top, back}, {right, top, front}, {left, top, front}}, Vector3{0, -1, 0}},
		{!isLit(x, y+1), [4]Vector3{{left, bottom, front}, {right, bottom, front}, {right, bottom, back}, {left, bottom, back}}, Vector3{0, 1, 0}},
		{!isLit(x-1, y), [4]Vector3{{left, top, back}, {left, top, front}, {left, bottom, front}, {left, bottom, back}}, Vector3{-1, 0, 0}},
		{!isLit(x+1, y), [4]Vector3{{right, top, front}, {right, top, back}, {right, bottom, back}, {right, bottom, front}}, Vector3{1, 0, 0}},
	}
	for _, side := range sides {
		l.Faces = mathx.AppendIf(l.Faces, VoxelFace{Corners: side.Corners, Normal: side.Normal}, side.IsOpen)
	}
}

func (l *VoxelLogo) projectFace(face *VoxelFace, pose LogoPose) {
	normal := rotate(face.Normal, pose)
	if normal[2] >= 0 {
		return
	}
	projected := projectedFace{}
	for index, corner := range face.Corners {
		rotated := rotate(corner, pose)
		depth := rotated[2] + pose.Distance
		projected.Points[index] = [2]float32{pose.CenterX + LogoFocal*rotated[0]/depth, pose.CenterY + LogoFocal*(rotated[1]+pose.Lift)/depth}
		projected.Depth += depth / 4
	}
	base := mathx.Pick(face.IsCap, l.SideColor, l.CapColor)
	brightness := 0.45 + 0.75*max(0, dotVector(normal, logoLight))
	projected.Tint = colorx.RGB{min(1, base[0]*brightness), min(1, base[1]*brightness), min(1, base[2]*brightness)}
	l.projected = append(l.projected, projected)
}

func (l *VoxelLogo) queueFace(face *projectedFace) {
	base := uint32(len(l.vertices))
	for _, point := range face.Points {
		l.vertices = append(l.vertices, ebiten.Vertex{
			DstX: point[0] * draw.Scale, DstY: point[1] * draw.Scale, SrcX: 1.5, SrcY: 1.5,
			ColorR: face.Tint[0], ColorG: face.Tint[1], ColorB: face.Tint[2], ColorA: 1,
		})
	}
	l.indices = append(l.indices, base, base+1, base+2, base, base+2, base+3)
}

func rotate(point Vector3, pose LogoPose) Vector3 {
	cosineTumble, sineTumble := mathx.Cos(pose.Tumble), mathx.Sin(pose.Tumble)
	y := point[1]*cosineTumble - point[2]*sineTumble
	z := point[1]*sineTumble + point[2]*cosineTumble
	cosineSpin, sineSpin := mathx.Cos(pose.Spin), mathx.Sin(pose.Spin)
	x := point[0]*cosineSpin + z*sineSpin
	z = -point[0]*sineSpin + z*cosineSpin
	cosineRoll, sineRoll := mathx.Cos(pose.Roll), mathx.Sin(pose.Roll)
	return Vector3{x*cosineRoll - y*sineRoll, x*sineRoll + y*cosineRoll, z}
}

func dotVector(a, b Vector3) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

func normalizeVector(vector Vector3) Vector3 {
	size := mathx.Sqrt(dotVector(vector, vector))
	return Vector3{vector[0] / size, vector[1] / size, vector[2] / size}
}

func compareDepth(a, b float32) int {
	return mathx.BoolToIndex(a > b) - mathx.BoolToIndex(a < b)
}
