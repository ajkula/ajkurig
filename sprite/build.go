package sprite

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// ===== Types =====

type bodyPlan struct {
	sheet   *Sheet
	palette []Swatch
	shades  [256]Shade
	sizes   map[string]int
	bodies  map[string]*Body
	body    *Body
}

type limbLayout struct {
	upper  string
	lower  string
	isNear bool
	isArm  bool
}

// ===== Constants =====

const (
	GuardTurn          = 45
	shoulderDropShare  = 0.15
	hipsRootLength     = 2
	restSettlePasses   = 2
	missingSize        = -1
	proportionsMissing = "the body files never give: "
)

var proportionNames = []string{"ground", "head", "torso", "legs", "arms", "shoulders", "hips"}

var bodyDirectives = map[string]func(p *bodyPlan, fields []string) error{
	"size":  (*bodyPlan).readSize,
	"color": (*bodyPlan).readColor,
	"shade": (*bodyPlan).readShade,
	"part":  (*bodyPlan).readPart,
	"band":  (*bodyPlan).readBand,
	"lump":  (*bodyPlan).readLump,
}

var limbs = []limbLayout{
	{"arm_front_upper", "arm_front_fore", true, true},
	{"arm_back_upper", "arm_back_fore", false, true},
	{"leg_back_thigh", "leg_back_shin", true, false},
	{"leg_front_thigh", "leg_front_shin", false, false},
}

// ===== Public API =====

func (s *Sheet) Build(files ...TextFile) error {
	plan := &bodyPlan{sheet: s, palette: slices.Clone(s.Palette), shades: s.Shades, sizes: map[string]int{}, bodies: map[string]*Body{}}
	for _, name := range append(proportionNames, "width", "height") {
		plan.sizes[name] = missingSize
	}
	plan.sizes["width"], plan.sizes["height"] = s.Width, s.Height
	for _, file := range files {
		if err := plan.parseFile(file); err != nil {
			return err
		}
	}
	if err := plan.check(); err != nil {
		return err
	}
	plan.apply()
	return nil
}

// ===== Parsing =====

func (p *bodyPlan) parseFile(file TextFile) error {
	p.body = nil
	for number, line := range strings.Split(file.Source, "\n") {
		if err := p.parseLine(strings.Fields(line)); err != nil {
			return fmt.Errorf("%s: %w", strings.TrimSpace(file.Name+" line "+strconv.Itoa(number+1)), err)
		}
	}
	return nil
}

func (p *bodyPlan) parseLine(fields []string) error {
	if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
		return nil
	}
	if read, isDirective := bodyDirectives[fields[0]]; isDirective {
		return read(p, fields[1:])
	}
	if slices.Contains(proportionNames, fields[0]) {
		return p.readProportion(fields)
	}
	return fmt.Errorf("unknown line %q, expected a proportion (%s) or one of size, color, shade, part, band, lump", fields[0], strings.Join(proportionNames, ", "))
}

func (p *bodyPlan) readProportion(fields []string) error {
	value, err := strconv.Atoi(append(fields, "")[1])
	if len(fields) != 2 || err != nil || value < 0 {
		return fmt.Errorf("%s needs a length in pixels, like: %s 20", fields[0], fields[0])
	}
	p.sizes[fields[0]] = value
	return nil
}

func (p *bodyPlan) readSize(fields []string) error {
	values, err := readInts(fields, 2)
	if err != nil || min(values[0], values[1]) < 1 {
		return errors.New("size needs a width and a height in pixels, like: size 64 96")
	}
	p.sizes["width"], p.sizes["height"] = values[0], values[1]
	return nil
}

func (p *bodyPlan) readColor(fields []string) error {
	value, err := strconv.ParseUint(append(fields, "")[1], 16, 32)
	if len(fields) != 2 || len(fields[0]) != 1 || len(fields[1]) != 6 || err != nil || fields[0][0] == transparentSymbol {
		return errors.New("color needs a symbol and a hex value, like: color p 9370db")
	}
	swatch := Swatch{fields[0][0], [3]uint8{uint8(value >> 16), uint8(value >> 8), uint8(value)}}
	if index := p.colorIndex(fields[0]); index > 0 {
		p.palette[index] = swatch
		return nil
	}
	p.palette = append(p.palette, swatch)
	return nil
}

func (p *bodyPlan) readShade(fields []string) error {
	if len(fields) != 3 {
		return errors.New("shade needs a colour then its lit and shadow colours, like: shade p A B")
	}
	base, lit, shadow := p.colorIndex(fields[0]), p.colorIndex(fields[1]), p.colorIndex(fields[2])
	if min(base, lit, shadow) <= 0 {
		return fmt.Errorf("shade uses colours missing from the palette: %q", strings.Join(fields, " "))
	}
	p.shades[base] = Shade{Lit: uint8(lit), Shadow: uint8(shadow)}
	return nil
}

func (p *bodyPlan) readPart(fields []string) error {
	if len(fields) != 5 {
		return errors.New("part needs a name, a start width, an end width, a depth ratio and a colour, like: part torso 14 12 0.6 p")
	}
	values, err := readFloats(fields[1:4])
	color := p.colorIndex(fields[4])
	if err != nil || min(values[0], values[1], values[2]) <= 0 || color <= 0 {
		return fmt.Errorf("part needs positive widths and depth ratio and a palette colour, got %q", strings.Join(fields, " "))
	}
	if p.sheet.FindPart(fields[0]) < 0 {
		return fmt.Errorf("the sprite has no part %q", fields[0])
	}
	p.body = &Body{StartWidth: values[0], EndWidth: values[1], DepthRatio: values[2], Color: uint8(color)}
	p.bodies[fields[0]] = p.body
	return nil
}

func (p *bodyPlan) readBand(fields []string) error {
	isSectored := len(fields) == 6 && fields[3] == "around"
	if p.body == nil || (len(fields) != 3 && !isSectored) {
		return errors.New("band belongs to a part and needs where it starts and ends along the bone and a colour, like: band 0 0.6 s, then an optional sector from the chest, like: around 0 20")
	}
	stretch, err := readFloats(fields[:2])
	sector, sectorErr := readFloats(append(fields[min(4, len(fields)):], "0", "0")[:2])
	color := p.colorIndex(fields[2])
	if err != nil || sectorErr != nil || color <= 0 || sector[1] < 0 {
		return fmt.Errorf("band needs numbers and a palette colour, got %q", strings.Join(fields, " "))
	}
	p.body.Bands = append(p.body.Bands, Band{From: stretch[0], To: stretch[1], Color: uint8(color), Around: sector[0] + GuardTurn, HalfAngle: sector[1]})
	return nil
}

func (p *bodyPlan) readLump(fields []string) error {
	values, err := readFloats(fields)
	if p.body == nil || len(fields) != 6 || err != nil || min(values[3], values[4], values[5]) <= 0 {
		return errors.New("lump belongs to a part and needs where it sits along the bone, its angle from the chest and distance from the bone, then its three radii, like: lump 0.3 0 3 6 6 5")
	}
	p.body.Lumps = append(p.body.Lumps, Lump{At: values[0], Around: values[1] + GuardTurn, Out: values[2], Along: values[3], Across: values[4], Outward: values[5]})
	return nil
}

func (p *bodyPlan) colorIndex(symbol string) int {
	if len(symbol) != 1 {
		return missingSize
	}
	return slices.IndexFunc(p.palette, func(swatch Swatch) bool { return swatch.Symbol == symbol[0] })
}

func (p *bodyPlan) check() error {
	missing := slices.DeleteFunc(slices.Clone(proportionNames), func(name string) bool { return p.sizes[name] != missingSize })
	if len(missing) > 0 {
		return errors.New(proportionsMissing + strings.Join(missing, ", "))
	}
	if len(p.palette) > 256 {
		return errors.New("the palette holds at most 255 colours")
	}
	return nil
}

// ===== Applying =====

func (p *bodyPlan) apply() {
	s := p.sheet
	s.Palette, s.Shades, s.Width, s.Height = p.palette, p.shades, p.sizes["width"], p.sizes["height"]
	p.layOutSkeleton()
	for _, part := range s.Parts {
		p.dress(part)
	}
	for range restSettlePasses {
		p.settleRest()
	}
}

func (p *bodyPlan) settleRest() {
	s := p.sheet
	pose := s.Grow(nil)
	s.Settle(pose, s.carriesDrawing)
	ends := s.BoneEnds(s.Transforms(pose))
	for index, part := range s.Parts {
		if part.isDrawn() {
			continue
		}
		part.Pivot, part.Tip = pixelUnder(ends[index][0]), pixelUnder(ends[index][1])
		p.dress(part)
	}
}

func (p *bodyPlan) dress(part *Part) {
	if planned := p.bodies[part.Name]; planned != nil {
		part.SetBody(*planned)
		return
	}
	if part.Body != nil {
		part.SetBody(*part.Body)
	}
}

func (p *bodyPlan) layOutSkeleton() {
	centerX, size := p.sheet.Width/2, p.sizes
	hipY := size["ground"] - size["legs"]
	neckY := hipY - size["torso"]
	shoulderY := neckY + int(math.Round(float64(size["torso"])*shoulderDropShare))
	p.place("hips", Point{centerX, hipY - hipsRootLength}, Point{centerX, hipY}, 0)
	p.place("pelvis", Point{centerX, hipY}, Point{centerX, hipY}, 0)
	p.place("torso", Point{centerX, hipY - 1}, Point{centerX, neckY}, 0)
	p.place("shoulders", Point{centerX, shoulderY}, Point{centerX, shoulderY}, 0)
	p.place("head", Point{centerX, neckY}, Point{centerX, neckY - size["head"]}, 0)
	for _, limb := range limbs {
		width, top, length := [2]int{size["hips"], size["shoulders"]}[boolToInt(limb.isArm)], [2]int{hipY, shoulderY}[boolToInt(limb.isArm)], [2]int{size["legs"], size["arms"]}[boolToInt(limb.isArm)]
		acrossX, acrossZ := guardOffset(width, limb.isNear)
		joint := top + length/2
		p.place(limb.upper, Point{centerX + acrossX, top}, Point{centerX + acrossX, joint}, acrossZ)
		p.place(limb.lower, Point{centerX + acrossX, joint}, Point{centerX + acrossX, top + length}, acrossZ)
	}
}

func (p *bodyPlan) place(name string, pivot, tip Point, depth int) {
	index := p.sheet.FindPart(name)
	if index < 0 {
		return
	}
	part := p.sheet.Parts[index]
	part.Depth = depth
	if p.bodies[name] == nil && part.isDrawn() {
		part.Translate(pivot.X-part.Pivot.X, pivot.Y-part.Pivot.Y)
		return
	}
	part.Pivot, part.Tip = pivot, tip
}

func (s *Sheet) carriesDrawing(index int) bool {
	return slices.ContainsFunc(s.Parts, func(part *Part) bool { return part.isDrawn() && s.isAncestor(index, slices.Index(s.Parts, part)) })
}

func (p *Part) isDrawn() bool {
	return p.Body == nil && p.Layer.Area().Width > 0
}

func pixelUnder(point [2]float64) Point {
	return Point{int(math.Floor(point[0])), int(math.Floor(point[1]))}
}

func guardOffset(width int, isNear bool) (int, int) {
	half := float64(width) / 2 * float64(1-2*boolToInt(!isNear))
	sine, cosine := math.Sincos(GuardTurn * math.Pi / 180)
	return -int(math.Round(half * cosine)), -int(math.Round(half * sine))
}
