package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ajkula/ajkurig/sprite"
)

// ===== Helpers =====

func writeHero(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hero.spr")
	if err := os.WriteFile(path, []byte(sprite.Write(sprite.NewHumanoid("hero", 64, 64))), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runTool(t *testing.T, name string, arguments ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	err := commands[name].run(arguments, &output)
	return output.String(), err
}

func lineStarting(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.Join(strings.Fields(line), " ")
		}
	}
	return ""
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func reloadHero(t *testing.T, path string) *sprite.Sheet {
	t.Helper()
	sheet, err := sprite.Parse(readFile(t, path))
	if err != nil {
		t.Fatal(err)
	}
	return sheet
}

// ===== Tests =====

func TestGetPartDescribesTheBoneAndItsLimits(t *testing.T) {
	output, err := runTool(t, "get", "part", writeHero(t), "leg_front_shin")
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{"part leg_front_shin", "parent leg_front_thigh", "limit z 0 160", "limit y -10 10", "solid no"} {
		if lineStarting(output, strings.Fields(wanted)[0]+" "+strings.Fields(wanted)[1]) != wanted {
			t.Fatalf("missing %q in:\n%s", wanted, output)
		}
	}
}

func TestGetPoseListsEveryJointOfTheFrame(t *testing.T) {
	output, err := runTool(t, "get", "pose", writeHero(t), "walk", "0")
	if err != nil {
		t.Fatal(err)
	}
	if lineStarting(output, "frame") != "frame walk 0 key 6 ticks" {
		t.Fatalf("the header should name the frame, got:\n%s", output)
	}
	if lineStarting(output, "leg_front_thigh ") != "leg_front_thigh z -25 x 0 y 0 shift 0 0" {
		t.Fatalf("the thigh should be listed with its three rotations, got:\n%s", output)
	}
	tween, err := runTool(t, "get", "pose", writeHero(t), "walk", "1")
	if err != nil || !strings.HasPrefix(lineStarting(tween, "frame"), "frame walk 1 tween") {
		t.Fatalf("a tween can be read too, got %v:\n%s", err, tween)
	}
}

func TestSetPoseStoresRotationsInsideTheLimits(t *testing.T) {
	path := writeHero(t)
	output, err := runTool(t, "set", "pose", path, "walk", "0", "leg_front_shin", "z", "-40", "head", "y", "30")
	if err != nil {
		t.Fatal(err)
	}
	if lineStarting(output, "leg_front_shin") != "leg_front_shin z 0 (asked -40, limited)" || lineStarting(output, "head") != "head y 30" {
		t.Fatalf("each change should report the applied angle, got:\n%s", output)
	}
	sheet, err := sprite.Parse(readFile(t, path))
	if err != nil {
		t.Fatal(err)
	}
	pose := sheet.FindAnimation("walk").Frames[0].Pose
	if pose[sheet.FindPart("leg_front_shin")].Angle != 0 || pose[sheet.FindPart("head")].Turn != 30 {
		t.Fatal("the changes should be written in the sprite file")
	}
}

func TestSetPoseRefusesBadRequestsAndKeepsTheFile(t *testing.T) {
	path := writeHero(t)
	before := readFile(t, path)
	for _, arguments := range [][]string{
		{"pose", path, "walk", "1", "head", "y", "30"},
		{"pose", path, "walk", "0", "tail", "y", "30"},
		{"pose", path, "walk", "0", "head", "w", "30"},
		{"pose", path, "walk", "0", "head", "y", "far"},
		{"pose", path, "walk", "0", "head", "y", "30", "torso", "z"},
		{"pose", path, "walk", "0", "head", "y", "30", "tail", "z", "5"},
	} {
		if _, err := runTool(t, "set", arguments...); err == nil {
			t.Fatalf("%v should be refused", arguments)
		}
	}
	if readFile(t, path) != before {
		t.Fatal("a refused change must leave the sprite file as it was")
	}
}

func TestSetShiftMovesAPartInAKeyPose(t *testing.T) {
	path := writeHero(t)
	if _, err := runTool(t, "set", "shift", path, "walk", "0", "torso", "1", "-2"); err != nil {
		t.Fatal(err)
	}
	sheet := reloadHero(t, path)
	if shift := sheet.FindAnimation("walk").Frames[0].Pose[sheet.FindPart("torso")].Shift; shift != (sprite.Point{X: 1, Y: -2}) {
		t.Fatalf("the torso should be shifted by 1 -2, got %v", shift)
	}
}

func TestSetReachBringsTheFootOnThePoint(t *testing.T) {
	path := writeHero(t)
	before := reloadHero(t, path)
	shin := before.FindPart("leg_front_shin")
	pose := before.FindAnimation("walk").Frames[0].Pose
	foot := before.BoneEnds(before.Transforms(pose))[shin][1]
	target := []string{strconv.Itoa(int(foot[0]) + 3), strconv.Itoa(int(foot[1]) - 4)}
	output, err := runTool(t, "set", "reach", path, "walk", "0", "leg_front_shin", target[0], target[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output, "leg_front_shin ends at ") {
		t.Fatalf("the tool should say where the foot landed, got:\n%s", output)
	}
	sheet := reloadHero(t, path)
	landed := sheet.BoneEnds(sheet.Transforms(sheet.FindAnimation("walk").Frames[0].Pose))[shin][1]
	wantedX, _ := strconv.ParseFloat(target[0], 64)
	wantedY, _ := strconv.ParseFloat(target[1], 64)
	if math.Hypot(landed[0]-wantedX, landed[1]-wantedY) > 1 {
		t.Fatalf("the foot should land on %v, it is at %v", target, landed)
	}
	if _, err := runTool(t, "set", "reach", path, "walk", "0", "torso", "1", "1"); err == nil {
		t.Fatal("only the end of a limb can reach")
	}
}

func TestSetLimitAndAnatomyChangeTheRanges(t *testing.T) {
	path := writeHero(t)
	if _, err := runTool(t, "set", "limit", path, "head", "y", "-30", "30"); err != nil {
		t.Fatal(err)
	}
	if _, err := runTool(t, "set", "limit", path, "leg_front_shin", "z", "free"); err != nil {
		t.Fatal(err)
	}
	sheet := reloadHero(t, path)
	if sheet.Parts[sheet.FindPart("head")].Limit(sprite.AxisY, 80) != 30 || sheet.Parts[sheet.FindPart("leg_front_shin")].Limit(sprite.AxisZ, -40) != -40 {
		t.Fatal("the head turn should stop at 30 and the knee bend be free")
	}
	output, err := runTool(t, "set", "anatomy", path, "human")
	if err != nil || !strings.Contains(output, "parts follow the human anatomy") {
		t.Fatalf("the anatomy should be applied, got %v:\n%s", err, output)
	}
	if reloadHero(t, path).Parts[sheet.FindPart("leg_front_shin")].Limit(sprite.AxisZ, -40) != 0 {
		t.Fatal("the anatomy should bring the knee limit back")
	}
	for _, arguments := range [][]string{{"limit", path, "head", "y", "30", "-30"}, {"anatomy", path, "dragon"}} {
		if _, err := runTool(t, "set", arguments...); err == nil {
			t.Fatalf("%v should be refused", arguments)
		}
	}
}

func TestSetSolidAndViewShapeTheParts(t *testing.T) {
	path := writeHero(t)
	if _, err := runTool(t, "set", "solid", path, "torso", "yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := runTool(t, "set", "view", path, "torso", "0", "90"); err != nil {
		t.Fatal(err)
	}
	sheet := reloadHero(t, path)
	torso := sheet.Parts[sheet.FindPart("torso")]
	if !torso.IsSolid || torso.FindFace(0, 90) == nil {
		t.Fatal("the torso should be solid with a top face")
	}
	if _, err := runTool(t, "set", "view", path, "torso", "0", "90"); err == nil {
		t.Fatal("a face cannot be added twice")
	}
}

func TestPaintWritesAndReadsPixelsBySymbol(t *testing.T) {
	path := writeHero(t)
	if _, err := runTool(t, "set", "view", path, "head", "90", "0"); err != nil {
		t.Fatal(err)
	}
	if _, err := runTool(t, "set", "paint", path, "head", "90", "0", "40", "10", "r", "42", "11", "y"); err != nil {
		t.Fatal(err)
	}
	output, err := runTool(t, "get", "paint", path, "head", "90", "0")
	if err != nil || output != "pixels 40 10\nr\n..y\nend\n" {
		t.Fatalf("the painted pixels should read back, got %v:\n%s", err, output)
	}
	if _, err := runTool(t, "set", "paint", path, "head", "90", "0", "40", "10", "."); err != nil {
		t.Fatal(err)
	}
	if output, _ := runTool(t, "get", "paint", path, "head", "90", "0"); output != "pixels 42 11\ny\nend\n" {
		t.Fatalf("a dot should erase, got:\n%s", output)
	}
	before := readFile(t, path)
	for _, arguments := range [][]string{
		{"paint", path, "head", "45", "0", "1", "1", "r"},
		{"paint", path, "head", "90", "0", "1", "1", "#"},
		{"paint", path, "head", "90", "0", "1", "1"},
	} {
		if _, err := runTool(t, "set", arguments...); err == nil {
			t.Fatalf("%v should be refused", arguments)
		}
	}
	if readFile(t, path) != before {
		t.Fatal("refused paint must leave the file as it was")
	}
}

func TestGetPaletteListsSymbolsAndColors(t *testing.T) {
	output, err := runTool(t, "get", "palette", writeHero(t))
	if err != nil || lineStarting(output, "r ") != "r 710000" {
		t.Fatalf("the palette should list r as blood red, got %v:\n%s", err, output)
	}
}

func TestComposeStacksMoveFilesTheLaterWinning(t *testing.T) {
	path := writeHero(t)
	directory := filepath.Dir(path)
	common, own := filepath.Join(directory, "common.moves"), filepath.Join(directory, "hero.moves")
	if err := os.WriteFile(common, []byte("move jab once\nkey 3\n  arm_front_upper 0\nmove bow once\nkey 4\n  torso 20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(own, []byte("move jab once\nkey 5\n  arm_front_upper 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := runTool(t, "compose", path, common, own)
	if err != nil {
		t.Fatal(err)
	}
	if bow, jab := lineStarting(output, "bow:"), lineStarting(output, "jab:"); bow != "bow: 1 frames, 4 ticks" || jab != "jab: 1 frames, 5 ticks" {
		t.Fatalf("compose keeps the common bow and the own jab, got %q and %q", bow, jab)
	}
	sheet, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.FindAnimation("bow") == nil || sheet.FindAnimation("jab") == nil {
		t.Fatal("both moves are written into the sprite")
	}
}

func TestBuildCreatesAMissingSpriteFromBodyFiles(t *testing.T) {
	directory := t.TempDir()
	path, body := filepath.Join(directory, "figure.spr"), filepath.Join(directory, "figure.body")
	source := "ground 60\nhead 10\ntorso 20\nlegs 26\narms 20\nshoulders 14\nhips 7\npart torso 13 12 0.6 p\n"
	if err := os.WriteFile(body, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := runTool(t, "build", path, body)
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Name != "figure" || sheet.Parts[sheet.FindPart("torso")].Body == nil || !strings.HasPrefix(output, "figure: 1 parts with a body") {
		t.Fatalf("build writes a new figure with a torso body, said %q", output)
	}
}
