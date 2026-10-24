package sprite

import (
	"errors"
	"fmt"
	"strconv"
)

// ===== Types =====

type Grip uint8

type Hold struct {
	Grip   Grip
	X      float64
	Depth  float64
	Height float64
}

// ===== Constants =====

const (
	GripNone Grip = iota
	GripHeld
	GripReleased
)

var gripNames = map[Grip]string{GripHeld: "hold", GripReleased: "release"}

var gripByName = map[string]Grip{"hold": GripHeld, "release": GripReleased}

// ===== Public API =====

func (a *Animation) HoldAt(tick int) (Hold, bool) {
	hold := a.FrameHold(a.FrameAt(tick))
	return hold, hold.Grip == GripHeld
}

func (a *Animation) FrameHold(index int) Hold {
	frame := &a.Frames[index]
	if frame.Kind == FrameKey {
		return frame.Hold
	}
	previous, before, hasPrevious := a.findKey(index, -1)
	if !hasPrevious || previous.Hold.Grip != GripHeld {
		return Hold{}
	}
	next, after, hasNext := a.findKey(index, 1)
	if !hasNext || next.Hold.Grip == GripNone {
		return previous.Hold
	}
	return previous.Hold.toward(next.Hold, float64(before)/float64(before+after))
}

// ===== Internal =====

func (h Hold) toward(target Hold, progress float64) Hold {
	return Hold{
		Grip:   GripHeld,
		X:      h.X + (target.X-h.X)*progress,
		Depth:  h.Depth + (target.Depth-h.Depth)*progress,
		Height: h.Height + (target.Height-h.Height)*progress,
	}
}

func readHold(fields []string) (Hold, error) {
	grip, isKnown := gripByName[append(fields, "")[0]]
	if !isKnown || len(fields) != 4 {
		return Hold{}, fmt.Errorf("a partner is placed like: hold x depth height, or: release x depth height, got %q", fields)
	}
	values := [3]float64{}
	for index := range values {
		value, err := strconv.ParseFloat(fields[index+1], 64)
		if err != nil {
			return Hold{}, fmt.Errorf("bad partner offset %q", fields[index+1])
		}
		values[index] = value
	}
	return Hold{Grip: grip, X: values[0], Depth: values[1], Height: values[2]}, nil
}

func readFrameHold(kind FrameKind, fields []string) (Hold, error) {
	if len(fields) == 0 {
		return Hold{}, nil
	}
	if kind != FrameKey {
		return Hold{}, errors.New("only a key frame places the partner")
	}
	return readHold(fields)
}

func readGripOf(name string) func(step *Step, fields []string) (int, error) {
	return func(step *Step, fields []string) (int, error) {
		hold, err := readHold(append([]string{name}, fields[:min(3, len(fields))]...))
		step.Hold = hold
		return 3, err
	}
}

func holdText(hold Hold) string {
	if hold.Grip == GripNone {
		return ""
	}
	return fmt.Sprintf(" %s %s %s %s", gripNames[hold.Grip], formatOffset(hold.X), formatOffset(hold.Depth), formatOffset(hold.Height))
}

func formatOffset(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
