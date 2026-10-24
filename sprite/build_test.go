package sprite

import (
	"math"
	"strings"
	"testing"
)

// ===== Helpers =====

const testBody = `
size 64 64
ground 60
head 10
torso 20
legs 26
arms 20
shoulders 14
hips 7
color A b8a6e0
color B 5a488c
shade p A B
part torso 13 12 0.6 p
  band 0.45 1 s around 0 35
  lump 0.3 0 3 6 6 5
part arm_front_upper 5 4 0.9 p
part arm_front_fore 4 4 0.9 p
  band 0.8 1 f
  lump 1 0 0 2.5 2.5 2.5
part arm_back_upper 5 4 0.9 p
part arm_back_fore 4 4 0.9 p
  band 0.8 1 f
part leg_front_thigh 6 5 0.9 p
part leg_front_shin 5 4 0.9 p
part leg_back_thigh 6 5 0.9 p
part leg_back_shin 5 4 0.9 p
`

func builtFigure(t *testing.T, extra ...TextFile) *Sheet {
	t.Helper()
	sheet := NewHumanoid("figure", 64, 64)
	if err := sheet.Build(append([]TextFile{{Name: "test.body", Source: testBody}}, extra...)...); err != nil {
		t.Fatal(err)
	}
	return sheet
}

func partNamed(t *testing.T, sheet *Sheet, name string) *Part {
	t.Helper()
	index := sheet.FindPart(name)
	if index < 0 {
		t.Fatalf("the sprite has no %s", name)
	}
	return sheet.Parts[index]
}

// ===== Tests =====

func TestBuildingTwiceGivesTheSameSprite(t *testing.T) {
	sheet := builtFigure(t)
	once := Write(sheet)
	if err := sheet.Build(TextFile{Name: "test.body", Source: testBody}); err != nil {
		t.Fatal(err)
	}
	if Write(sheet) != once {
		t.Fatal("a second build of the same files changes nothing")
	}
}

func TestTheShouldersStandInTheThreeQuarterGuard(t *testing.T) {
	sheet := builtFigure(t)
	near, far, torso := partNamed(t, sheet, "arm_front_upper"), partNamed(t, sheet, "arm_back_upper"), partNamed(t, sheet, "torso")
	if near.Pivot.X >= torso.Pivot.X || near.Depth >= 0 || far.Pivot.X <= torso.Pivot.X || far.Depth <= 0 {
		t.Fatalf("the near arm is left and toward the camera, the far one right and away: near %v depth %d, far %v depth %d", near.Pivot, near.Depth, far.Pivot, far.Depth)
	}
	if gap := far.Pivot.X - near.Pivot.X; gap < 9 || gap > 11 {
		t.Fatalf("14 wide shoulders seen at 45 degrees are about 10 apart, got %d", gap)
	}
	if foot := partNamed(t, sheet, "leg_back_shin"); foot.Tip.Y != 60 {
		t.Fatalf("the feet stand on the ground line, got %v", foot.Tip)
	}
}

func TestADrawnHeadSurvivesTheBuild(t *testing.T) {
	sheet := NewHumanoid("figure", 64, 64)
	head := partNamed(t, sheet, "head")
	head.Clear()
	head.Set(head.Pivot.X+1, head.Pivot.Y-3, 3)
	if err := sheet.Build(TextFile{Name: "test.body", Source: testBody}); err != nil {
		t.Fatal(err)
	}
	if head.Body != nil || head.At(head.Pivot.X+1, head.Pivot.Y-3) != 3 {
		t.Fatal("the drawn head keeps its pixels and follows its pivot to the neck")
	}
	if torso := partNamed(t, sheet, "torso"); head.Pivot != torso.Tip {
		t.Fatalf("the head sits on the neck, head %v neck %v", head.Pivot, torso.Tip)
	}
}

func TestALaterBodyFileOverridesTheEarlierOne(t *testing.T) {
	sheet := builtFigure(t, TextFile{Name: "tall.body", Source: "legs 30\npart torso 16 15 0.7 p\n"})
	if foot := partNamed(t, sheet, "leg_front_shin"); foot.Tip.Y != 60 || partNamed(t, sheet, "leg_front_thigh").Pivot.Y != 30 {
		t.Fatal("the longer legs start higher and still stand on the ground")
	}
	torso := partNamed(t, sheet, "torso").Body
	if torso.StartWidth != 16 || len(torso.Bands) != 0 || len(torso.Lumps) != 0 {
		t.Fatalf("a part line replaces the whole earlier body, got %+v", torso)
	}
}

func TestCommonMovesKeepTheFeetPlantedOnABuiltBody(t *testing.T) {
	sheet := builtFigure(t)
	scripts, err := ParseMoves("move lunge once\nkey 4 plant leg_back_shin\n  torso -70\n  leg_front_thigh 60\n")
	if err != nil {
		t.Fatal(err)
	}
	pose := sheet.Compose(scripts[0]).Frames[0].Pose
	shin := sheet.FindPart("leg_back_shin")
	rest := sheet.BoneEnds(sheet.Transforms(sheet.Grow(nil)))[shin][1]
	posed := sheet.BoneEnds(sheet.Transforms(pose))[shin][1]
	if math.Hypot(posed[0]-rest[0], posed[1]-rest[1]) > 1 {
		t.Fatalf("the planted foot stays put on the built body: %v then %v", rest, posed)
	}
}

func TestBodyFilesExplainTheirMistakes(t *testing.T) {
	cases := map[string]string{
		"legs 26\n":                         "the body files never give: ground, head, torso, arms, shoulders, hips",
		testBody + "part tail 4 2 1 p\n":    "the sprite has no part",
		testBody + "part torso 13 12 1 Z\n": "palette colour",
		testBody + "wings 3\n":              "unknown line",
	}
	for source, wanted := range cases {
		err := NewHumanoid("figure", 64, 64).Build(TextFile{Name: "bad.body", Source: source})
		if err == nil || !strings.Contains(err.Error(), wanted) {
			t.Fatalf("expected an error about %q, got %v", wanted, err)
		}
	}
}
