package editor

import "slices"

// ===== Types =====

type Mode uint8

type modeSpec struct {
	label      string
	feature    string
	tools      []Tool
	isRest     bool
	partsTitle string
	guideTitle string
	guide      []string
}

// ===== Constants =====

const (
	ModeSkeleton Mode = iota
	ModePoses
	ModeDraw
	ModeAnimate
	modeCount
)

var modes = [modeCount]modeSpec{
	ModeSkeleton: {
		label:      "1  SKELETON",
		feature:    "mode.skeleton",
		tools:      []Tool{ToolPlace, ToolBone, ToolHand},
		isRest:     true,
		partsTitle: "BONES  Ctrl+click: several",
		guideTitle: "STEP 1 OF 4  -  SKELETON: the frame every drawing hangs on",
		guide: []string{
			"Blue lines are bones, purple bars are the shoulder and hip hangers. ERASE ALL PAINT removes the example drawing.",
			"Set the CANVAS, then the whole body: POSITION, BODY SIZE, ORIENTATION.",
			"For a front or top view, Ctrl+click several bones (both arms...) and drag them with PLACE: the hangers stretch along.",
			"With the BONE tool, drag the end of a bone to make one limb longer or shorter. Then go to step 2.",
		},
	},
	ModePoses: {
		label:      "2  POSES",
		feature:    "mode.poses",
		tools:      []Tool{ToolPose, ToolHand},
		isRest:     false,
		partsTitle: "PARTS  Ctrl+click: several",
		guideTitle: "STEP 2 OF 4  -  POSES",
	},
	ModeDraw: {
		label:      "3  DRAW",
		feature:    "mode.draw",
		tools:      []Tool{ToolPencil, ToolFill, ToolPicker, ToolTrace, ToolHand},
		isRest:     true,
		partsTitle: "PARTS  click the one to paint",
		guideTitle: "STEP 3 OF 4  -  DRAW: one piece of the body at a time",
		guide: []string{
			"Click a part on the left (head, torso, a thigh...). Only that part takes paint, the others stay faint behind it.",
			"Pick a colour on the right, then paint with the PENCIL. Right click erases. Zoom with the wheel to work pixel by pixel.",
			"To copy a model, drop a picture on this window: it appears under the drawing and you paint over it.",
			"Each part moves with its bone, so the PREVIEW already animates what you paint. Then go to step 4.",
		},
	},
	ModeAnimate: {
		label:      "4  ANIMATE",
		feature:    "mode.animate",
		tools:      []Tool{ToolPose, ToolPencil, ToolPicker, ToolHand},
		isRest:     false,
		partsTitle: "PARTS  Ctrl+click: several",
		guideTitle: "STEP 4 OF 4  -  ANIMATE",
	},
}

var toolHomes = [toolCount]Mode{
	ToolPencil: ModeDraw,
	ToolFill:   ModeDraw,
	ToolPicker: ModeDraw,
	ToolBone:   ModeSkeleton,
	ToolPose:   ModePoses,
	ToolHand:   ModeDraw,
	ToolTrace:  ModeDraw,
	ToolPlace:  ModeSkeleton,
}

var toolFeatures = [toolCount]string{
	ToolPencil: "tool.pencil",
	ToolFill:   "tool.fill",
	ToolPicker: "tool.picker",
	ToolBone:   "tool.bone",
	ToolPose:   "tool.pose",
	ToolHand:   "tool.hand",
	ToolTrace:  "tool.trace",
	ToolPlace:  "tool.place",
}

// ===== Modes =====

func (e *Editor) setMode(mode Mode) {
	e.mode = mode
	e.isRestView = modes[mode].isRest
	if !e.isToolInMode(e.tool) {
		e.tool = modes[mode].tools[0]
	}
}

func (e *Editor) selectTool(tool Tool) {
	if !e.isToolInMode(tool) {
		e.setMode(toolHomes[tool])
	}
	e.tool = tool
}

func (e *Editor) isToolInMode(tool Tool) bool {
	return slices.Contains(modes[e.mode].tools, tool)
}
