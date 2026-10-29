package shell

import (
	"reflect"
	"testing"
)

// ===== Constants =====

const testTick = float32(1.0 / 60)

// ===== Tests =====

func TestHitsFromTheSameSourceAreCountedAndSummed(t *testing.T) {
	var log DamageLog
	log.RecordHit(10.0, "Horde Brute", 40)
	log.RecordHit(10.6, "Swarmer", 6)
	log.RecordHit(11.2, "Horde Brute", 40)
	want := []DamageCause{{Source: "Horde Brute", Hits: 2, Damage: 80}, {Source: "Swarmer", Hits: 1, Damage: 6}}
	if got := log.Causes(11.2); !reflect.DeepEqual(got, want) {
		t.Fatalf("causes = %+v, want %+v", got, want)
	}
}

func TestOnlyTheLastSecondsBeforeDeathAreReported(t *testing.T) {
	log := DamageLog{WindowSeconds: 2}
	log.RecordHit(1, "Runner", 5)
	log.RecordHit(8.5, "Bloater", 10)
	log.RecordHit(10, "Brute", 108)
	want := []DamageCause{{Source: "Brute", Hits: 1, Damage: 108}, {Source: "Bloater", Hits: 1, Damage: 10}}
	if got := log.Causes(10); !reflect.DeepEqual(got, want) {
		t.Fatalf("causes = %+v, want %+v", got, want)
	}
}

func TestContinuousDamageCountsAsOneExposure(t *testing.T) {
	var log DamageLog
	for tick := range 30 {
		log.RecordContinuous(5+float32(tick)*testTick, testTick, "Burning Ground", 0.25)
	}
	causes := log.Causes(5 + 29*testTick)
	if len(causes) != 1 || causes[0].Hits != 1 || causes[0].Damage < 7.49 || causes[0].Damage > 7.51 {
		t.Fatalf("causes = %+v, want one exposure of 7.5 damage", causes)
	}
}

func TestLeavingAndReenteringContinuousDamageCountsTwice(t *testing.T) {
	var log DamageLog
	log.RecordContinuous(5, testTick, "Spider Tank Beam", 2)
	log.RecordContinuous(5+testTick, testTick, "Spider Tank Beam", 2)
	log.RecordContinuous(5.5, testTick, "Spider Tank Beam", 2)
	causes := log.Causes(5.5)
	if len(causes) != 1 || causes[0].Hits != 2 || causes[0].Damage != 6 {
		t.Fatalf("causes = %+v, want 2 exposures for 6 damage", causes)
	}
}

func TestAHitBetweenTwoContinuousTicksSplitsTheExposure(t *testing.T) {
	var log DamageLog
	log.RecordContinuous(5, testTick, "Burning Ground", 1)
	log.RecordHit(5+testTick, "Swarmer", 6)
	log.RecordContinuous(5+2*testTick, testTick, "Burning Ground", 1)
	want := []DamageCause{{Source: "Swarmer", Hits: 1, Damage: 6}, {Source: "Burning Ground", Hits: 2, Damage: 2}}
	if got := log.Causes(5 + 2*testTick); !reflect.DeepEqual(got, want) {
		t.Fatalf("causes = %+v, want %+v", got, want)
	}
}

func TestNoDamageLeavesNothingToReport(t *testing.T) {
	var log DamageLog
	log.RecordHit(3, "Swarmer", 0)
	log.RecordContinuous(3, testTick, "Burning Ground", 0)
	if causes := log.Causes(3); len(causes) != 0 {
		t.Fatalf("causes = %+v, want none", causes)
	}
	log.RecordHit(4, "Swarmer", 6)
	log.Clear()
	if causes := log.Causes(4); len(causes) != 0 {
		t.Fatalf("causes after Clear = %+v, want none", causes)
	}
}

func TestOldDamageIsForgottenSoTheLogStaysSmall(t *testing.T) {
	log := DamageLog{WindowSeconds: 2}
	for tick := range 6000 {
		log.RecordHit(float32(tick)*0.6, "Swarmer", 6)
	}
	if len(log.events) > 5 {
		t.Fatalf("the log kept %d events for a 2 s window", len(log.events))
	}
}

func TestHitCountsShowOnlyRepeats(t *testing.T) {
	for hits, want := range map[int]string{1: "", 2: "x2", 12: "x12"} {
		if got := hitsLabel(hits); got != want {
			t.Fatalf("hitsLabel(%d) = %q, want %q", hits, got, want)
		}
	}
}
