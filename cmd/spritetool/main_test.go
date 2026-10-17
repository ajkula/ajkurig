package main

import (
	"bytes"
	"os"
	"path/filepath"
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
