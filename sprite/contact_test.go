package sprite

import (
	"slices"
	"testing"
)

// ===== Helpers =====

const bellyBody = testBody + `
part torso 13 12 0.6 p
  lump 0.3 0 3 7 8 6
`

func contactNames(sheet *Sheet, pose Pose) []string {
	names := []string{}
	for _, contact := range sheet.Contacts(pose) {
		names = append(names, sheet.Parts[contact.First].Name+"+"+sheet.Parts[contact.Second].Name)
	}
	return names
}

// ===== Tests =====

func TestPartsJoinedByAJointMayTouchOnlyThere(t *testing.T) {
	sheet := builtFigure(t)
	if names := contactNames(sheet, sheet.Grow(nil)); len(names) > 0 {
		t.Fatalf("after the build only joints touch: shoulders, hips and elbows, got %v", names)
	}
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("arm_front_fore")].Angle = -150
	if names := contactNames(sheet, pose); !slices.Contains(names, "arm_front_upper+arm_front_fore") {
		t.Fatalf("a forearm folded onto its upper arm touches it past the elbow, got %v", names)
	}
}

func TestTheArmRestsOnTheBellyInsteadOfCrossingIt(t *testing.T) {
	sheet := builtFigure(t, TextFile{Name: "belly.body", Source: bellyBody})
	if names := contactNames(sheet, sheet.Grow(nil)); len(names) > 0 {
		t.Fatalf("the built rest pose has no part inside another, got %v", names)
	}
	upper, fore := partNamed(t, sheet, "arm_front_upper"), partNamed(t, sheet, "arm_front_fore")
	if fore.Tip.X >= upper.Pivot.X || upper.Tip != fore.Pivot {
		t.Fatalf("the near arm swings out around the belly and stays jointed: upper %v-%v, fore %v-%v", upper.Pivot, upper.Tip, fore.Pivot, fore.Tip)
	}
}

func TestTheLimbYieldsNotTheLegItTouches(t *testing.T) {
	sheet := builtFigure(t, TextFile{Name: "belly.body", Source: bellyBody})
	for _, name := range []string{"leg_front_thigh", "leg_front_shin", "leg_back_thigh", "leg_back_shin"} {
		if leg := partNamed(t, sheet, name); leg.Pivot.X != leg.Tip.X {
			t.Fatalf("the legs carry the body and keep standing straight, %s goes %v to %v", name, leg.Pivot, leg.Tip)
		}
	}
}

func TestComposedMovesStopOnTheBody(t *testing.T) {
	sheet := builtFigure(t, TextFile{Name: "belly.body", Source: bellyBody})
	scripts, err := ParseMoves("move hug once\nkey 4\n  arm_front_upper 60\n  arm_front_fore 0\n")
	if err != nil {
		t.Fatal(err)
	}
	pose := sheet.Compose(scripts[0]).Frames[0].Pose
	if names := contactNames(sheet, pose); len(names) > 0 {
		t.Fatalf("an arm aimed through the belly stops on it, got %v", names)
	}
}

func TestDrawnSpritesAreNeverSettled(t *testing.T) {
	sheet := NewHumanoid("drawn", 64, 64)
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("arm_front_fore")].Angle = 90
	sheet.Settle(pose, func(int) bool { return false })
	if pose[sheet.FindPart("arm_front_fore")].Angle != 90 || len(sheet.Contacts(pose)) > 0 {
		t.Fatal("parts without a body have no volume to collide")
	}
}
