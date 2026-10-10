package editor

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/mathx"
)

// ===== Types =====

type feature struct {
	title   string
	tip     string
	steps   []string
	related string
}

// ===== Constants =====

const (
	tooltipDelay   = 24
	tooltipWidth   = 380
	tutorialWidth  = 720
	lineHeight     = 18
	tutorialMargin = 24
)

var features = map[string]feature{
	"help": {"HELP", "Click this, then click anything else: instead of acting, it opens a short guide about it.",
		[]string{"Click ? HELP: it lights up.", "Click any button, list, colour or the drawing.", "A guide like this one opens. CLOSE or Esc brings you back.", "Hovering a button for a moment also shows a short tip."},
		"Esc cancels the help click."},

	"mode.skeleton": {"STEP 1: SKELETON", "Set the size and proportions of the body. Its bones carry every drawing and every move.",
		[]string{"A new sprite already has a full human skeleton and five moves (idle, walk, punch, kick, hurt).", "Use BODY SIZE on the right to make it taller, shorter, wider or narrower.", "With the BONE tool, drag the end of a bone to change the length of a limb.", "Watch the PREVIEW: the moves follow your changes.", "Then go to 2 DRAW."},
		"Comes before DRAW. Changing bones later moves the parts already drawn with them."},
	"mode.poses": {"STEP 2: POSES", "Move the limbs freely and keep the postures you like as snapshots.",
		[]string{"Grab a bone with the POSE tool and drag it around its joint.", "The purple bars are hangers: the shoulders bar carries both arms, the hips bar both legs. Drag a bar to tilt it like a balance.", "Ctrl+click several bones to turn them together.", "When the posture is right, click TAKE SNAPSHOT. It appears in the gallery at the bottom.", "Click a snapshot to load it, change it, then REPLACE IT WITH THIS POSE. RESET goes back to the start.", "In ANIMATE, click a snapshot to put it on a frame."},
		"Comes after SKELETON. Snapshots are saved in the sprite file."},
	"snap.take":          {"TAKE SNAPSHOT", "Keeps the current posture as a new snapshot in the gallery.", nil, "Rename it in the field that appears. Saved with the sprite."},
	"snap.rename":        {"SNAPSHOT NAME", "Click to rename the selected snapshot, Enter to confirm.", nil, "Choose names you will recognise: guard, crouch, jump..."},
	"snap.replace":       {"REPLACE IT WITH THIS POSE", "Overwrites the selected snapshot with the posture on the drawing.", nil, "UNDO brings the old posture back."},
	"snap.delete":        {"DELETE IT", "Removes the selected snapshot from the gallery.", nil, "UNDO brings it back."},
	"snap.gallery":       {"SNAPSHOT GALLERY", "All your snapshots. Click one to load it on the drawing and change it.", nil, "Loading replaces the posture on the drawing: take a snapshot first if you want to keep it."},
	"snap.apply":         {"USE SNAPSHOT", "Adds this snapshot as the next frame of the move, or puts it on the current frame, depending on the choice above.", nil, "On a new empty move, the first click fills the first frame. UNDO cancels."},
	"snap.modeadd":       {"ADD AS NEXT FRAME", "Clicking a snapshot adds it after the current frame: click them in order to build the move.", nil, "One of two choices with PUT ON THIS FRAME."},
	"snap.modeput":       {"PUT ON THIS FRAME", "Clicking a snapshot changes the pose of the current frame only.", nil, "One of two choices with ADD AS NEXT FRAME."},
	"snap.smooth":        {"SMOOTH", "When on, an AUTO frame is added before each new snapshot so the body moves smoothly from one pose to the next.", nil, "Turn it off for snappy moves like a punch impact."},
	"snap.fromframe":     {"SNAPSHOT IT", "Keeps the posture of this frame as a new snapshot.", nil, "Find it in 2 POSES and reuse it on other frames."},
	"pose.resetselected": {"RESET SELECTED", "Puts the selected parts back to their starting position.", nil, "RESET ALL does it for the whole body. Ctrl+click selects several parts."},
	"pose.resetall":      {"RESET WHOLE BODY", "Puts every part, hangers included, back to the starting posture.", nil, "UNDO cancels in ANIMATE. In POSES, take a snapshot first to keep a posture."},
	"select.all":         {"SELECT ALL", "Selects every part, so PLACE moves the whole body and POSE turns everything together.", nil, "Ctrl+click a part or a bone to add or remove it."},
	"select.one":         {"ONLY THIS ONE", "Keeps only the highlighted part selected.", nil, "A normal click on a part does the same."},
	"mode.draw": {"STEP 2: DRAW", "Paint the body one part at a time, each on its own bone.",
		[]string{"Click a part in the list on the left.", "Pick a colour on the right.", "Paint on the drawing with the PENCIL (right click erases).", "The other parts stay faint behind (GHOST), so the pieces fit together.", "Zoom with the mouse wheel to place single pixels.", "Optional: drop a picture on the window and paint over it."},
		"Comes after SKELETON, before ANIMATE. The PREVIEW shows your parts moving already."},
	"mode.animate": {"STEP 3: ANIMATE", "Change or create moves: poses, timing, and final retouches.",
		[]string{"Pick a move in the timeline at the bottom (idle, walk...).", "Pick a frame. KEY frames are poses you set, AUTO frames are computed between two keys.", "On a KEY frame, grab a bone with the POSE tool and drag to turn it.", "TWIN LIMBS moves the other leg or arm along to save time.", "Optional: BAKE a frame to retouch its pixels by hand."},
		"Comes last. The parts you drew in step 2 follow the bones."},

	"file.open": {"OPEN / NEW", "Opens another sprite of the same folder, or starts a new one with a ready skeleton.",
		[]string{"Click OPEN / NEW.", "Click a sprite on the left to open it, or type a name on the right and click CREATE.", "Your changes are saved before the other sprite opens."},
		"The game finds every sprite of the folder by its file name."},
	"open.file":    {"SPRITE FILE", "Opens this sprite.", nil, "Unsaved changes of the current one are saved first."},
	"open.name":    {"NEW SPRITE NAME", "The file name of the new sprite: letters, digits and _ only.", nil, "The game calls its moves name.move, for example robot.walk."},
	"open.create":  {"CREATE", "Creates the sprite with a ready skeleton and moves, or opens it if it already exists.", nil, "Then follow the steps 1 to 4."},
	"dialog.close": {"CLOSE", "Closes this window without doing anything else.", nil, "Esc does the same."},
	"anim.import": {"IMPORT MOVES", "Copies moves made for another sprite into this one, even when its skeleton has other proportions or another rest pose.",
		[]string{"Click IMPORT.", "Pick the sprite that has the moves.", "Click each move you want: it is added to this sprite.", "Check it in the PREVIEW and fix the poses that need it."},
		"Every bone points the same way as in the other sprite, so a guard pose stays a guard pose."},
	"import.file":      {"SOURCE SPRITE", "Shows the moves of this sprite.", nil, "Nothing changes until you click a move."},
	"import.move":      {"IMPORT THIS MOVE", "Adds this move to the sprite you are editing.", nil, "If the name is taken, a number is added. UNDO cancels."},
	"import.snapshots": {"IMPORT SNAPSHOTS", "Adds the snapshots of the other sprite to this one.", nil, "They appear in 2 POSES."},
	"ref.captureall": {"COPY INTO ALL PARTS", "Copies the reference picture into the parts: each pixel goes to the closest bone. Only empty pixels are painted.",
		[]string{"Drop a picture and place it over the skeleton with TRACE.", "Place the bones on the body of the picture (step 1, or POSE in step 2).", "Click COPY INTO ALL PARTS.", "Hide the picture to see the result, retouch with the PENCIL."},
		"New colours join the palette. ERASE ALL PAINT first to start from a clean body."},
	"ref.captureselected": {"COPY INTO SELECTED", "Like COPY INTO ALL PARTS, but only the selected parts take paint.",
		[]string{"Pose the skeleton on a second picture where a limb is fully visible (an arm stretched in a punch...).", "Select that limb, Ctrl+click for several parts.", "Click INTO SELECTED: the hidden side of the limb gets its pixels."},
		"Only empty pixels are painted, so the first picture stays."},
	"file.save": {"SAVE", "Writes the sprite to its .spr file (Ctrl+S). A * in the window title means unsaved changes.", nil, "The game reads the saved file."},
	"file.undo": {"UNDO", "Cancels the last change (Ctrl+Z). You can undo many times.", nil, "REDO brings it back."},
	"file.redo": {"REDO", "Brings back what UNDO cancelled (Ctrl+Y).", nil, "Opposite of UNDO. A new change clears what could be redone."},

	"tool.bone": {"BONE tool  (V)", "Changes the length of a bone, or places a new bone for the selected part.",
		[]string{"Press on the end of a bone (the end away from its joint) and drag: the limb gets longer or shorter and what hangs below follows.", "Press anywhere else: the selected part gets its joint there, then drag to where the bone ends."},
		"Used in SKELETON. Pair it with BODY SIZE for the whole body."},
	"tool.pencil": {"PENCIL tool  (B)", "Paints pixels with the chosen colour. Right click erases.",
		[]string{"Choose the part on the left and a colour on the right.", "Click or drag on the drawing.", "Right click or the ERASE colour removes pixels.", "In ANIMATE, on a BAKED frame, it retouches that frame only."},
		"Works with COLOURS, GHOST, SOLO and the REFERENCE picture. UNDO cancels a stroke."},
	"tool.fill":   {"FILL tool  (G)", "Fills a closed area of the selected part with the chosen colour.", []string{"Draw an outline with the PENCIL first.", "Click inside it with FILL.", "Right click fills with transparency."}, "Works with PENCIL and COLOURS."},
	"tool.picker": {"PICK tool  (I)", "Takes the colour under the cursor and makes it the current colour.", nil, "Holding Alt does the same with any tool."},
	"tool.trace": {"TRACE tool  (F)", "Moves and resizes the reference picture under the drawing.",
		[]string{"Drop a picture file (PNG, JPG...) on this window: it loads and this tool is selected.", "Drag to move the picture.", "Turn the mouse wheel to make it bigger or smaller.", "Then take the PENCIL and paint over it."},
		"Goes with REFERENCE PICTURE on the right. The picture is never saved into the sprite."},
	"tool.pose": {"POSE tool  (R)", "Turns a bone around its joint to make a pose.",
		[]string{"Choose a KEY frame in the timeline.", "Press on a bone and drag around its joint: it turns, and everything hanging from it follows.", "Shift + drag moves the bone instead of turning it.", "Joints have human limits: a knee cannot bend forwards."},
		"Goes with TWIN LIMBS. On an AUTO frame it first turns that frame into a KEY."},
	"tool.place": {"PLACE tool  (N)", "Drag to move the whole body inside the canvas: bones and paint together.", nil, "The POSITION buttons do the same pixel by pixel, CENTRE puts it back in the middle."},
	"tool.hand":  {"HAND tool  (P)", "Drag to move the view when it is zoomed in.", nil, "The middle mouse button does the same with any tool. FIT shows everything again."},

	"view.zoomout": {"ZOOM OUT", "Shows the drawing smaller (minus key, or wheel over the drawing).", nil, "Opposite of ZOOM IN. FIT goes back to the whole sprite."},
	"view.zoomin":  {"ZOOM IN", "Shows the drawing bigger to work pixel by pixel (plus key, or wheel over the drawing).", nil, "The grid appears when pixels are big. HAND moves the view."},
	"view.fit":     {"FIT", "Shows the whole sprite again, centred (key 0).", nil, "Cancels ZOOM and HAND moves."},

	"parts.list": {"PARTS", "The pieces of the body. Click one to select it, Ctrl+click to select several (PLACE and POSE then move them together).",
		[]string{"Click a name to select it.", "Indented names hang from the one above them: a shin hangs from its thigh.", "The list order is the drawing order: lower in the list is drawn in front."},
		"Up and Down arrow keys also select."},
	"part.rename": {"PART NAME", "Click to rename the selected part, Enter to confirm, Esc to cancel.", nil, "Names ending in front/back or left/right make TWIN LIMBS find the matching limb."},
	"part.add":    {"ADD PART", "Adds a new empty part hanging from the selected one (a hat, a weapon, a tail...).", []string{"Select the part it should follow.", "Click ADD.", "Use the BONE tool to place its joint, then paint it in DRAW."}, "DELETE removes it, UNDO too."},
	"part.copy":   {"COPY PART", "Duplicates the selected part with its pixels and its moves.", nil, "Handy for a second arm: copy, then MIRROR it in DRAW."},
	"part.delete": {"DELETE PART", "Removes the selected part. Parts hanging from it hang from its parent instead.", nil, "UNDO brings it back."},
	"part.parent": {"HANGS FROM", "Chooses which part this one follows when it moves. Click to go through the choices.", nil, "Defines the indents in the PARTS list."},
	"part.up":     {"BEHIND", "Draws the selected part earlier, so it goes behind the others.", nil, "Opposite of IN FRONT."},
	"part.down":   {"IN FRONT", "Draws the selected part later, so it covers the others.", nil, "Opposite of BEHIND."},
	"part.mirror": {"MIRROR PIXELS", "Flips the pixels of the selected part left to right around its joint.", nil, "Click again to flip back."},

	"paint.clearall": {"ERASE ALL PAINT", "Removes every painted pixel from every part, for example the grey example body. The bones, joints and moves stay.",
		[]string{"Click it once: the canvas shows only the skeleton.", "Adjust the skeleton if needed.", "Go to 2 DRAW and paint your own character."},
		"UNDO brings the paint back. ERASE THIS PART'S PAINT in DRAW clears a single part."},
	"part.clear":      {"ERASE THIS PART'S PAINT", "Removes every pixel of the selected part only.", nil, "UNDO brings it back. ERASE ALL PAINT in SKELETON clears every part."},
	"place.left":      {"LEFT", "Moves the whole body 1 pixel left.", nil, "Opposite of RIGHT. The PLACE tool drags it freely."},
	"place.right":     {"RIGHT", "Moves the whole body 1 pixel right.", nil, "Opposite of LEFT."},
	"place.up":        {"UP", "Moves the whole body 1 pixel up.", nil, "Opposite of DOWN."},
	"place.down":      {"DOWN", "Moves the whole body 1 pixel down.", nil, "Opposite of UP."},
	"place.centre":    {"CENTRE", "Puts the body in the middle of the canvas, standing on the bottom.", nil, "Use it after changing the canvas size or turning the body."},
	"size.smaller":    {"SMALLER", "Makes the whole body 10% smaller in every direction. The feet stay on the ground.", nil, "Opposite of BIGGER. Paint is moved, not stretched: resize before drawing."},
	"size.bigger":     {"BIGGER", "Makes the whole body 10% bigger in every direction. The feet stay on the ground.", nil, "Opposite of SMALLER. Make the CANVAS bigger first if the body does not fit."},
	"orient.flip":     {"FACE OTHER WAY", "Mirrors the body, its paint and all its moves left to right.", nil, "Click again to face back. The game already flips sprites when they walk the other way."},
	"orient.left":     {"TURN LEFT", "Turns the body, its paint and its moves a quarter turn anticlockwise, for creatures that lie or fly.", nil, "Opposite of TURN RIGHT. CENTRE afterwards puts it back in the middle."},
	"orient.right":    {"TURN RIGHT", "Turns the body, its paint and its moves a quarter turn clockwise.", nil, "Opposite of TURN LEFT."},
	"canvas.narrower": {"CANVAS NARROWER", "Removes 8 pixels of width around the body.", nil, "Opposite of WIDER. Pixels outside the new canvas are lost (UNDO brings them back)."},
	"canvas.wider":    {"CANVAS WIDER", "Adds 8 pixels of width, for a bigger character or a long weapon.", nil, "Opposite of NARROWER."},
	"canvas.shorter":  {"CANVAS SHORTER", "Removes 8 pixels of height at the top.", nil, "Opposite of TALLER."},
	"canvas.taller":   {"CANVAS TALLER", "Adds 8 pixels of height at the top, for a taller boss.", nil, "Opposite of SHORTER. Then BIGGER or TALLER to fill it."},
	"size.shorter":    {"SHORTER", "Makes the whole body 5% shorter. The feet stay on the ground.", nil, "Opposite of TALLER. UNDO cancels."},
	"size.taller":     {"TALLER", "Makes the whole body 5% taller. The feet stay on the ground.", nil, "Opposite of SHORTER."},
	"size.narrower":   {"NARROWER", "Brings shoulders and hips 1 pixel closer to the spine.", nil, "Opposite of WIDER."},
	"size.wider":      {"WIDER", "Moves shoulders and hips 1 pixel away from the spine.", nil, "Opposite of NARROWER."},

	"colours": {"COLOURS", "Click a colour to paint with it. The first square (ERASE) paints transparency.", nil, "Used by PENCIL and FILL. PICK or Alt takes a colour from the drawing."},

	"ref.show":    {"SHOW / HIDE PICTURE", "Shows or hides the reference picture without forgetting it.", nil, "The picture is loaded by dropping a file on the window."},
	"ref.smaller": {"SMALLER PICTURE", "Shrinks the reference picture.", nil, "Opposite of BIGGER. The wheel with TRACE does it at the cursor."},
	"ref.bigger":  {"BIGGER PICTURE", "Grows the reference picture.", nil, "Opposite of SMALLER."},
	"ref.fainter": {"FAINTER", "Makes the reference picture more transparent, so your pixels stand out.", nil, "Opposite of BOLDER."},
	"ref.bolder":  {"BOLDER", "Makes the reference picture less transparent.", nil, "Opposite of FAINTER."},
	"ref.fit":     {"FIT PICTURE", "Resizes the reference picture to fit inside the sprite.", nil, "Cancels SMALLER, BIGGER and TRACE moves."},

	"show.ghost":   {"GHOST", "While painting, the other parts stay visible but faint, so you see where the selected part goes.", nil, "SOLO hides them completely instead."},
	"show.solo":    {"SOLO", "Shows only the selected part.", nil, "Stronger than GHOST: when SOLO is on, GHOST has no effect."},
	"show.hangers": {"HANGERS  (J)", "Shows or hides the purple shoulder and hip bars. Hidden, they cannot be grabbed by mistake but still carry the arms and legs.", nil, "Turn it on to tilt the shoulders or hips like a balance with POSE."},
	"show.bones":   {"BONES", "Shows the skeleton lines on the drawing.", nil, "Needed to grab bones with POSE or BONE."},
	"show.onion":   {"ONION", "Shows the previous frame faintly behind, to compare poses.", nil, "Only in ANIMATE."},

	"preview.play": {"PLAY / PAUSE", "Starts or stops the small preview of the current move (Space).", nil, "The move shown is the one selected in ANIMATE."},

	"link.off":    {"TWIN: OFF", "Each limb moves on its own.", nil, "One of three choices with MIRROR and SAME."},
	"link.mirror": {"TWIN: MIRROR", "Turning a leg or arm swings the other one the opposite way, like when walking. Knees and elbows bend alike.", nil, "One of three choices with OFF and SAME. Key M cycles."},
	"link.same":   {"TWIN: SAME", "Turning a leg or arm turns the other one the same way, like when jumping.", nil, "One of three choices with OFF and MIRROR."},

	"anim.list":   {"MOVES", "The animations of this sprite. Click one to show its frames.", nil, "The game plays them by name: idle, walk, punch, kick, hurt."},
	"anim.rename": {"MOVE NAME", "Click to rename the selected move, Enter to confirm.", nil, "The game finds moves by this name."},
	"anim.add": {"NEW MOVE", "Starts a new move with an empty first frame and lets you type its name.",
		[]string{"Click 1 NEW.", "Type the name the game will use (punch, jump, dance...) and press Enter.", "Click snapshots on the right, in order: each one becomes the next frame.", "Watch the PREVIEW, change durations with SHORTER and LONGER.", "SAVE. The game finds the move as sprite.move, for example hero.jump."},
		"Snapshots are made in 2 POSES."},
	"anim.copy":   {"COPY MOVE", "Duplicates the selected move.", nil, "Good start for a variant, like a stronger punch."},
	"anim.delete": {"DELETE MOVE", "Removes the selected move.", nil, "UNDO brings it back."},
	"anim.loop":   {"LOOPS / ONCE", "Whether the move starts over at the end (walk) or plays once (punch).", nil, "Check it in the PREVIEW."},

	"frame.list": {"FRAMES", "The pictures of the move, left to right. KEY = a pose you set. AUTO = computed between two keys. The number is how long it stays (60 per second).",
		[]string{"Click a frame to show it on the drawing.", "Pose KEY frames with the POSE tool.", "AUTO frames follow by themselves.", "BAKED frames are frozen pixels you retouch with the PENCIL."},
		"Left and Right arrow keys also move between frames."},
	"frame.addkey":  {"+ KEY", "Adds a key frame after this one, starting with the same pose.", nil, "Then change its pose with POSE."},
	"frame.addauto": {"+ AUTO", "Adds a computed frame after this one, half way between the keys around it.", nil, "Makes the move smoother."},
	"frame.earlier": {"EARLIER", "Moves the frame one step to the left.", nil, "Opposite of LATER."},
	"frame.later":   {"LATER", "Moves the frame one step to the right.", nil, "Opposite of EARLIER."},
	"frame.delete":  {"DELETE FRAME", "Removes this frame.", nil, "UNDO brings it back."},
	"frame.shorter": {"SHORTER", "This frame stays on screen a bit less time.", nil, "Opposite of LONGER. Watch the PREVIEW."},
	"frame.longer":  {"LONGER", "This frame stays on screen a bit more time.", nil, "Opposite of SHORTER."},
	"frame.kind":    {"MAKE KEY / MAKE AUTO", "Turns a computed frame into a pose you can change, or a key back into a computed frame.", nil, "POSE turns an AUTO frame into a KEY by itself."},
	"frame.bake":    {"BAKE / UNBAKE", "BAKE freezes the frame into plain pixels you can retouch with the PENCIL. UNBAKE goes back to the bones.", nil, "A baked frame no longer follows the bones or later drawing changes."},

	"guide.next": {"NEXT STEP", "Goes to the next step of the work.", nil, "The step tabs at the top do the same."},
}

// ===== Tooltip =====

func (e *Editor) layoutTooltip() {
	id, ticks := e.ui.Hovered()
	entry, isKnown := features[id]
	if !isKnown || ticks < tooltipDelay || e.ui.IsHelping {
		return
	}
	lines := append([]string{entry.title}, e.ui.Wrap(entry.tip, tooltipWidth-2*textPadding)...)
	if entry.related != "" {
		lines = append(lines, e.ui.Wrap(entry.related, tooltipWidth-2*textPadding)...)
	}
	height := float32(len(lines))*lineHeight + 2*textPadding
	x := min(e.ui.MouseX+16, LogicalWidth-tooltipWidth-4)
	y := min(e.ui.MouseY+20, LogicalHeight-height-4)
	if y < e.ui.MouseY+20 {
		y = e.ui.MouseY - height - 8
	}
	box := Rect{x, y, tooltipWidth, height}
	e.ui.Fill(box, colorButton)
	e.ui.Outline(box, colorAccent)
	for index, line := range lines {
		tint := [2]color.Color{colorText, colorAccent}[mathx.BoolToIndex(index == 0)]
		e.ui.Text(x+textPadding, y+textPadding+float32(index)*lineHeight, line, tint, text.AlignStart)
	}
}

// ===== Tutorial =====

func (e *Editor) openPickedTutorial() {
	picked := e.ui.TakePicked()
	if _, isKnown := features[picked]; !isKnown {
		return
	}
	e.tutorial = picked
}

func (e *Editor) layoutTutorial() {
	entry, isOpen := features[e.tutorial]
	if !isOpen {
		return
	}
	width := float32(tutorialWidth - 2*tutorialMargin)
	lines := e.ui.Wrap(entry.tip, width)
	for index, step := range entry.steps {
		lines = append(lines, "")
		lines = append(lines, e.ui.Wrap(stepPrefix(index)+step, width)...)
	}
	if entry.related != "" {
		lines = append(append(lines, ""), e.ui.Wrap("Good to know: "+entry.related, width)...)
	}
	height := float32(len(lines)+4)*lineHeight + 2*tutorialMargin
	box := Rect{(LogicalWidth - tutorialWidth) / 2, (LogicalHeight - height) / 2, tutorialWidth, height}
	e.ui.Fill(Rect{0, 0, LogicalWidth, LogicalHeight}, colorShade)
	e.ui.Fill(box, colorPanel)
	e.ui.Outline(box, colorAccent)
	e.ui.Text(box.X+tutorialMargin, box.Y+tutorialMargin, entry.title, colorAccent, text.AlignStart)
	for index, line := range lines {
		e.ui.Text(box.X+tutorialMargin, box.Y+tutorialMargin+float32(index+2)*lineHeight, line, colorText, text.AlignStart)
	}
	closeRect := Rect{box.X + box.W - tutorialMargin - 100, box.Y + box.H - tutorialMargin - buttonHeight, 100, buttonHeight}
	isClosing := e.ui.Button(closeRect, "CLOSE", true, "tutorial.close") || inpututil.IsKeyJustPressed(ebiten.KeyEscape)
	e.tutorial = [2]string{e.tutorial, ""}[mathx.BoolToIndex(isClosing)]
}

func stepPrefix(index int) string {
	return fmt.Sprintf("%d.  ", index+1)
}
