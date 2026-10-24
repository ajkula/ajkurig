package sprite

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// ===== Types =====

type MoveFile struct {
	Name   string
	Source string
}

type movesParser struct {
	scripts []MoveScript
	shapes  map[string]map[string]Aim
	aims    map[string]Aim
	step    *Step
}

// ===== Constants =====

var moveDirectives = map[string]func(p *movesParser, fields []string) error{
	"move":  (*movesParser).readMove,
	"shape": (*movesParser).readShape,
	"key":   (*movesParser).readKey,
	"tween": (*movesParser).readTween,
	"use":   (*movesParser).readUse,
}

var aimOptions = map[string]func(aim *Aim, value string) error{
	"face": readFacing,
	"draw": readDrawing,
}

var keyOptions = map[string]func(step *Step, fields []string) (int, error){
	"plant":   readPlant,
	"brace":   readBrace,
	"at":      readPlantX,
	"lift":    readLift,
	"lead":    readLead,
	"knee":    readKnee,
	"hold":    readGripOf("hold"),
	"release": readGripOf("release"),
}

// ===== Public API =====

func ParseMoves(source string) ([]MoveScript, error) {
	return ParseMoveFiles(MoveFile{Source: source})
}

func ParseMoveFiles(files ...MoveFile) ([]MoveScript, error) {
	p := &movesParser{shapes: map[string]map[string]Aim{}}
	for _, file := range files {
		if err := p.parseFile(file); err != nil {
			return nil, err
		}
	}
	return p.scripts, nil
}

// ===== Internal =====

func (p *movesParser) parseFile(file MoveFile) error {
	p.aims, p.step = nil, nil
	for number, line := range strings.Split(file.Source, "\n") {
		if err := p.parseLine(strings.TrimSpace(line)); err != nil {
			return fmt.Errorf("%s: %w", strings.TrimSpace(file.Name+" line "+strconv.Itoa(number+1)), err)
		}
	}
	return nil
}

func (p *movesParser) parseLine(line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
		return nil
	}
	read, isDirective := moveDirectives[fields[0]]
	if isDirective {
		return read(p, fields[1:])
	}
	return p.readAim(fields)
}

func (p *movesParser) readMove(fields []string) error {
	if len(fields) < 1 || len(fields) > 2 {
		return errors.New("move needs a name, then loop or once")
	}
	p.scripts = slices.DeleteFunc(p.scripts, func(script MoveScript) bool { return script.Name == fields[0] })
	p.scripts = append(p.scripts, MoveScript{Name: fields[0], IsLooping: len(fields) < 2 || fields[1] != loopNames[false]})
	p.aims, p.step = nil, nil
	return nil
}

func (p *movesParser) readShape(fields []string) error {
	if len(fields) != 1 {
		return errors.New("shape needs a name")
	}
	p.aims, p.step = map[string]Aim{}, nil
	p.shapes[fields[0]] = p.aims
	return nil
}

func (p *movesParser) readKey(fields []string) error {
	if len(p.scripts) == 0 {
		return errors.New("key belongs to a move")
	}
	ticks, err := strconv.Atoi(append(fields, "")[0])
	if err != nil || ticks < 1 {
		return errors.New("key needs a duration of at least 1 tick")
	}
	step := Step{Kind: FrameKey, Ticks: ticks, Aims: map[string]Aim{}}
	for rest := fields[1:]; len(rest) > 0; {
		read, isKnown := keyOptions[rest[0]]
		if !isKnown {
			return fmt.Errorf("unknown key option %q (plant, brace, knee, at, lift, lead, hold, release)", rest[0])
		}
		used, err := read(&step, rest[1:])
		if err != nil {
			return err
		}
		rest = rest[1+used:]
	}
	script := &p.scripts[len(p.scripts)-1]
	script.Steps = append(script.Steps, step)
	p.step = &script.Steps[len(script.Steps)-1]
	p.aims = p.step.Aims
	return nil
}

func (p *movesParser) readTween(fields []string) error {
	ticks, err := strconv.Atoi(append(fields, "")[0])
	if len(p.scripts) == 0 || err != nil || ticks < 1 {
		return errors.New("tween belongs to a move and needs a duration of at least 1 tick")
	}
	script := &p.scripts[len(p.scripts)-1]
	script.Steps = append(script.Steps, Step{Kind: FrameTween, Ticks: ticks})
	p.aims, p.step = nil, nil
	return nil
}

func (p *movesParser) readUse(fields []string) error {
	shape, isKnown := p.shapes[append(fields, "")[0]]
	if p.aims == nil || !isKnown {
		return errors.New("use belongs to a key or a shape and needs a shape defined above")
	}
	maps.Copy(p.aims, shape)
	return nil
}

func (p *movesParser) readAim(fields []string) error {
	if p.aims == nil {
		return fmt.Errorf("%q is not a directive and there is no key or shape to aim a part in", fields[0])
	}
	if len(fields) < 2 || len(fields)%2 != 0 {
		return errors.New("aim a part like: arm_front_upper 45, or: torso -90 face 30, or: arm_back_fore 50 draw side, with - to keep the rest direction")
	}
	aim := p.aims[fields[0]]
	if fields[1] != "-" {
		direction, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return fmt.Errorf("bad direction %q", fields[1])
		}
		aim.Direction, aim.HasDirection = direction, true
	}
	for rest := fields[2:]; len(rest) > 0; rest = rest[2:] {
		read, isKnown := aimOptions[rest[0]]
		if !isKnown {
			return fmt.Errorf("expected face and an angle or draw and a drawing name, got %q", strings.Join(rest, " "))
		}
		if err := read(&aim, rest[1]); err != nil {
			return err
		}
	}
	p.aims[fields[0]] = aim
	return nil
}

func readFacing(aim *Aim, value string) error {
	facing, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("face needs an angle, got %q", value)
	}
	aim.Facing, aim.HasFacing = facing, true
	return nil
}

func readDrawing(aim *Aim, value string) error {
	aim.Drawing = value
	return nil
}

func readPlant(step *Step, fields []string) (int, error) {
	if len(fields) < 1 {
		return 0, errors.New("plant needs a part, usually a shin, then an optional offset of the foot")
	}
	offset, used := readOptionalOffset(fields[1:])
	step.Planted, step.PlantOffset = fields[0], offset
	return 1 + used, nil
}

func readOptionalOffset(fields []string) (Point, int) {
	if len(fields) < 2 {
		return Point{}, 0
	}
	values, err := readInts(fields[:2], 2)
	if err != nil {
		return Point{}, 0
	}
	return Point{values[0], values[1]}, 2
}

func readBrace(step *Step, fields []string) (int, error) {
	if len(fields) < 1 {
		return 0, errors.New("brace needs the shin of a foot to keep on the ground, then an optional offset of that foot")
	}
	offset, used := readOptionalOffset(fields[1:])
	step.Braces = append(step.Braces, Brace{Part: fields[0], Offset: offset})
	return 1 + used, nil
}

func readPlantX(step *Step, fields []string) (int, error) {
	value, err := strconv.ParseFloat(append(fields, "")[0], 64)
	if err != nil {
		return 0, errors.New("at needs the x where the planted foot stands")
	}
	step.PlantX, step.IsAtX = value, true
	return 1, nil
}

func readLift(step *Step, fields []string) (int, error) {
	if len(fields) < 2 {
		return 0, errors.New("lift needs two offsets")
	}
	values, err := readInts(fields[:2], 2)
	if err != nil {
		return 0, err
	}
	step.Lift = Point{values[0], values[1]}
	return 2, nil
}

func readLead(step *Step, fields []string) (int, error) {
	value, err := strconv.ParseFloat(append(fields, "")[0], 64)
	if err != nil {
		return 0, errors.New("lead needs how far ahead of the planted foot the body weight sits, in pixels")
	}
	step.Lead, step.IsLeant = value, true
	return 1, nil
}

func readKnee(step *Step, fields []string) (int, error) {
	value, err := strconv.ParseFloat(append(fields, "")[0], 64)
	if err != nil || len(step.Braces) == 0 {
		return 0, errors.New("knee follows a brace and needs the least bend of that knee, in degrees")
	}
	step.Braces[len(step.Braces)-1].Knee = value
	return 1, nil
}
