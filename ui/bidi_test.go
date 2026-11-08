package ui

import (
	"reflect"
	"testing"
)

// ===== Tests =====

func TestWidthCountsTheWidestLineOfMultilineText(t *testing.T) {
	kit := NewText(&Theme{SmallSize: 16}, DefaultFontFiles())
	wide := "a much longer second line"
	if got, want := kit.Width("short\n"+wide, FaceSmall), kit.Width(wide, FaceSmall); got != want {
		t.Fatalf("multiline width %v, want the widest line %v", got, want)
	}
}

func TestLatinTextStaysASingleLeftToRightRun(t *testing.T) {
	got := visualRuns("Horde 12 / 30")
	want := []directedRun{{Text: "Horde 12 / 30"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
}

func TestLatinNameInsideArabicIsPlacedLeftOfTheOpeningWords(t *testing.T) {
	runs := visualRuns("تم تدمير Spider Tank")
	if len(runs) != 2 || runs[0].IsRightToLeft || !runs[1].IsRightToLeft || runs[0].Text != "Spider Tank" {
		t.Fatalf("%+v", runs)
	}
}

func TestNumberInsideArabicKeepsItsDigitsInOrder(t *testing.T) {
	runs := visualRuns("المستوى 12")
	if len(runs) != 2 || runs[0].Text != "12" || runs[0].IsRightToLeft {
		t.Fatalf("%+v", runs)
	}
}

func TestDirectionFollowsTheFirstLetter(t *testing.T) {
	cases := map[string]bool{"العربية": true, "Spider Tank يقترب": false, "12 مستوى": true, "12 / 30": false, "": false}
	for message, want := range cases {
		if startsRightToLeft(message) != want {
			t.Errorf("%q should be right to left: %v", message, want)
		}
	}
}

func TestMarqueeEntersFromTheSideWhereReadingStarts(t *testing.T) {
	const boxWidth, textWidth = 100, 300
	if offset := marqueeOffset(0, boxWidth, textWidth, false); offset != boxWidth {
		t.Fatalf("left to right starts at %v", offset)
	}
	if offset := marqueeOffset(0, boxWidth, textWidth, true); offset != -textWidth {
		t.Fatalf("right to left starts at %v", offset)
	}
	if offset := marqueeOffset(textWidth+boxWidth, boxWidth, textWidth, true); offset != boxWidth {
		t.Fatalf("right to left ends at %v", offset)
	}
}
