package scores

import (
	"testing"

	"ajkurig/store"
)

// ===== Public API =====

func TestTableKeepsTheBestEntriesInOrder(t *testing.T) {
	table := NewTable(3)
	for _, score := range []int{50, 300, 10, 120, 5} {
		table.Insert(Entry{Initials: "AAA", Score: score})
	}
	if len(table.Entries) != 3 || table.Best().Score != 300 || table.EntryAt(2).Score != 50 {
		t.Fatalf("table %+v", table.Entries)
	}
	if table.RankOf(40) != NoRank || table.RankOf(0) != NoRank || table.RankOf(200) != 1 {
		t.Fatal("ranks are wrong")
	}
}

func TestScoresSurviveSavingAndAreClamped(t *testing.T) {
	saves := store.InDirectory(t.TempDir())
	table := NewTable(10)
	table.Insert(Entry{Initials: "GRG", Score: MaximumScore * 2, Seconds: 61, Stats: map[string]int{"kills": 42}})
	if err := Save(saves, table); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(saves, 10)
	if err != nil || loaded.Best().Score != MaximumScore || loaded.Best().Stats["kills"] != 42 {
		t.Fatalf("loaded %+v err %v", loaded.Best(), err)
	}
	missing, err := Load(store.InDirectory(t.TempDir()), 10)
	if err == nil || len(missing.Entries) != 0 || missing.Size != 10 {
		t.Fatal("a missing file should give an empty table and an error")
	}
}

func TestNameEntryWrapsRepeatsAndConfirms(t *testing.T) {
	entry := NewNameEntry("ZZ?", 3, "")
	if entry.Initials() != "AAA" {
		t.Fatalf("invalid initials not sanitized: %q", entry.Initials())
	}
	entry.Update(-1, 0, 0, true, 1.0/60)
	if entry.Initials() != ".AA" {
		t.Fatalf("letter did not wrap backward: %q", entry.Initials())
	}
	for range 120 {
		entry.Update(0, 0, 1, false, 1.0/60)
	}
	if entry.Letters[0] == '.' {
		t.Fatal("holding did not repeat")
	}
	isDone := entry.Advance() || entry.Advance() || entry.Advance()
	if !isDone || entry.Cursor != 2 {
		t.Fatalf("advance did not reach the end")
	}
}

func TestFormatting(t *testing.T) {
	if FormatThousands(1234567) != "1,234,567" || FormatThousands(999) != "999" {
		t.Fatal("thousands grouping is wrong")
	}
	if FormatClock(125) != "02:05" || Ordinal(0) != "1ST" || Ordinal(24) != "25TH" {
		t.Fatal("clock or ordinal is wrong")
	}
}
