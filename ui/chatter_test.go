package ui

import (
	"testing"

	"github.com/ajkula/ajkurig/rng"
)

// ===== Constants =====

const (
	testTopicHeal = iota
	testTopicBoss
)

var testTopics = []Topic{
	testTopicHeal: {Key: "heal", LineCount: 3, CooldownSeconds: 60},
	testTopicBoss: {Key: "boss", LineCount: 2, CooldownSeconds: 20, IsUrgent: true},
}

// ===== Public API =====

func TestChatterEntersRightAndLeavesLeftPixelByPixel(t *testing.T) {
	boxWidth, textWidth := float32(160), float32(300)
	speed := DefaultBubbleStyle.ScrollSpeed
	if marqueeOffset(0, boxWidth) != boxWidth {
		t.Fatal("the line does not start beyond the right edge")
	}
	if step := marqueeOffset(0.5, boxWidth); step != boxWidth {
		t.Fatalf("the line moved by a fraction of a pixel: %v", step)
	}
	if marqueeOffset(1.5, boxWidth) != boxWidth-1 {
		t.Fatal("the line does not advance one whole pixel at a time")
	}
	chatter := NewChatter(DefaultBubbleStyle, testTopics)
	chatter.Speak("x")
	chatter.Seconds = (textWidth + boxWidth) / speed * 0.99
	if !chatter.IsSpeaking(textWidth, boxWidth) {
		t.Fatal("the line stopped before leaving the left edge")
	}
	chatter.Seconds = (textWidth + boxWidth) / speed * 1.01
	if chatter.IsSpeaking(textWidth, boxWidth) {
		t.Fatal("the line is still shown after leaving")
	}
}

func TestChatterIsDebounced(t *testing.T) {
	chatter := NewChatter(DefaultBubbleStyle, testTopics)
	random := rng.New(3)
	chatter.QuietSeconds = DefaultBubbleStyle.GlobalGap
	if !chatter.Say(testTopicHeal, &random) || len(chatter.Runes) == 0 {
		t.Fatal("the first heal line was not said")
	}
	first := string(chatter.Runes)
	chatter.QuietSeconds = DefaultBubbleStyle.GlobalGap
	if chatter.Say(testTopicHeal, &random) || string(chatter.Runes) != first || chatter.QuietSeconds != DefaultBubbleStyle.GlobalGap {
		t.Fatal("a second heal line was said within its cooldown")
	}
	chatter.QuietSeconds = 1
	if chatter.Say(testTopicBoss, &random) || string(chatter.Runes) != first {
		t.Fatal("a line was said within the urgent gap")
	}
	chatter.IsSilenced = true
	chatter.QuietSeconds = DefaultBubbleStyle.GlobalGap
	if chatter.Say(testTopicBoss, &random) || string(chatter.Runes) != first {
		t.Fatal("a silenced chatter talked")
	}
}

func TestUrgentTopicsCutTheGlobalGapShort(t *testing.T) {
	chatter := NewChatter(DefaultBubbleStyle, testTopics)
	random := rng.New(4)
	chatter.QuietSeconds = DefaultBubbleStyle.UrgentGap
	if chatter.Say(testTopicHeal, &random) {
		t.Fatal("an ordinary line skipped the global gap")
	}
	if !chatter.Say(testTopicBoss, &random) {
		t.Fatal("an urgent line waited for the global gap")
	}
}

func TestChatterNeverRepeatsTheLastLine(t *testing.T) {
	chatter := NewChatter(DefaultBubbleStyle, []Topic{{Key: "idle", LineCount: 2}})
	random := rng.New(5)
	previous := ""
	for range 40 {
		chatter.QuietSeconds = DefaultBubbleStyle.GlobalGap
		chatter.Say(0, &random)
		if string(chatter.Runes) == previous {
			t.Fatalf("the line %q was said twice in a row", previous)
		}
		previous = string(chatter.Runes)
	}
}
