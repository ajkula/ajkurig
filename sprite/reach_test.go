package sprite

import (
	"math"
	"slices"
	"testing"
)

// ===== Helpers =====

func newReachSheet(t *testing.T) (*Sheet, Pose) {
	t.Helper()
	sheet := NewHumanoid("hero", 64, 64)
	return sheet, sheet.Grow(nil)
}

func limbEnd(sheet *Sheet, pose Pose, index int) [2]float64 {
	return sheet.BoneEnds(sheet.Transforms(pose))[index][1]
}

func distanceBetween(first, second [2]float64) float64 {
	return math.Hypot(first[0]-second[0], first[1]-second[1])
}

// ===== Tests =====

func TestHandsAndFeetEndATwoBoneLimb(t *testing.T) {
	sheet, _ := newReachSheet(t)
	for end, upper := range map[string]string{"leg_front_shin": "leg_front_thigh", "arm_back_fore": "arm_back_upper"} {
		found, isLimb := sheet.LimbOf(sheet.FindPart(end))
		if !isLimb || found != sheet.FindPart(upper) {
			t.Fatalf("%s should end a limb starting at %s", end, upper)
		}
	}
	for _, name := range []string{"leg_front_thigh", "hips", "pelvis", "shoulders", "head"} {
		if _, isLimb := sheet.LimbOf(sheet.FindPart(name)); isLimb {
			t.Fatalf("%s is not the end of a limb", name)
		}
	}
}

func TestReachPutsTheFootOnTheTarget(t *testing.T) {
	sheet, pose := newReachSheet(t)
	shin := sheet.FindPart("leg_front_shin")
	before := slices.Clone(pose)
	foot := limbEnd(sheet, pose, shin)
	target := [2]float64{foot[0] + 4, foot[1] - 6}
	sheet.Reach(pose, shin, target)
	if distance := distanceBetween(limbEnd(sheet, pose, shin), target); distance > 0.5 {
		t.Fatalf("the foot should land on the target, %v away", distance)
	}
	for index := range pose {
		isLeg := index == shin || index == sheet.FindPart("leg_front_thigh")
		if !isLeg && pose[index] != before[index] {
			t.Fatalf("only the front leg should move, %s moved", sheet.Parts[index].Name)
		}
	}
}

func TestReachBendsTheKneeTheWayItCan(t *testing.T) {
	sheet, pose := newReachSheet(t)
	shin := sheet.FindPart("leg_front_shin")
	for _, side := range []float64{-5, 5} {
		clear(pose)
		foot := limbEnd(sheet, pose, shin)
		target := [2]float64{foot[0] + side, foot[1] - 7}
		sheet.Reach(pose, shin, target)
		knee := pose[shin].Angle
		if sheet.Parts[shin].Limit(AxisZ, knee) != knee || distanceBetween(limbEnd(sheet, pose, shin), target) > 0.5 {
			t.Fatalf("lifting the foot to the side %v should bend the knee within its limits, knee %v", side, knee)
		}
	}
}

func TestReachStretchesTowardAFarTarget(t *testing.T) {
	sheet, pose := newReachSheet(t)
	fore := sheet.FindPart("arm_front_fore")
	ends := sheet.BoneEnds(sheet.Transforms(pose))
	shoulder := ends[sheet.FindPart("arm_front_upper")][0]
	reach := distanceBetween(shoulder, ends[fore][1])
	target := [2]float64{shoulder[0] + 100, shoulder[1]}
	sheet.Reach(pose, fore, target)
	hand := limbEnd(sheet, pose, fore)
	if math.Abs(hand[1]-shoulder[1]) > 0.5 || math.Abs(distanceBetween(shoulder, hand)-reach) > 0.5 {
		t.Fatalf("a far target should stretch the arm straight toward it, hand at %v", hand)
	}
}
