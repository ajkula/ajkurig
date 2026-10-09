package scores

import (
	"fmt"
	"slices"

	"ajkurig/mathx"
	"ajkurig/store"
)

// ===== Types =====

type Entry struct {
	Initials string         `json:"initials"`
	Score    int            `json:"score"`
	Seconds  float32        `json:"seconds"`
	Stats    map[string]int `json:"stats,omitempty"`
}

type Table struct {
	Entries []Entry `json:"entries"`
	Size    int     `json:"-"`
}

type NameEntry struct {
	Letters     []byte
	Cursor      int
	HoldSeconds float32
	Alphabet    string
}

// ===== Constants =====

const (
	DefaultTableSize    = 10
	NoRank              = -1
	MaximumScore        = 999999999
	TableFileName       = "highscores.json"
	DefaultAlphabet     = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789."
	DefaultInitials     = "AAA"
	letterRepeatDelay   = 1
	letterRepeatSeconds = 0.07
)

var RankOrdinals = []string{"1ST", "2ND", "3RD", "4TH", "5TH", "6TH", "7TH", "8TH", "9TH", "10TH", "11TH", "12TH", "13TH", "14TH", "15TH", "16TH", "17TH", "18TH", "19TH", "20TH"}

// ===== Public API =====

func NewTable(size int) Table {
	return Table{Size: mathx.Pick(size <= 0, size, DefaultTableSize)}
}

func (t *Table) Best() Entry {
	return t.EntryAt(0)
}

func (t *Table) EntryAt(rank int) Entry {
	if rank < 0 || rank >= len(t.Entries) {
		return Entry{}
	}
	return t.Entries[rank]
}

func (t *Table) RankOf(score int) int {
	rank := len(t.Entries)
	for index, entry := range t.Entries {
		if score >= entry.Score {
			rank = index
			break
		}
	}
	isRanked := score > 0 && rank < t.Size
	return mathx.Pick(isRanked, NoRank, rank)
}

func (t *Table) Insert(entry Entry) int {
	entry.Score = min(entry.Score, MaximumScore)
	rank := t.RankOf(entry.Score)
	if rank == NoRank {
		return NoRank
	}
	t.Entries = slices.Insert(t.Entries, rank, entry)
	t.Entries = t.Entries[:min(len(t.Entries), t.Size)]
	return rank
}

func (t *Table) Clear() {
	t.Entries = t.Entries[:0]
}

func Load(saves store.Store, size int) (Table, error) {
	table := NewTable(size)
	err := saves.Load(TableFileName, &table)
	table.Size = mathx.Pick(size <= 0, size, DefaultTableSize)
	for index := range table.Entries {
		table.Entries[index].Score = min(table.Entries[index].Score, MaximumScore)
	}
	table.Entries = table.Entries[:min(len(table.Entries), table.Size)]
	return table, err
}

func Save(saves store.Store, table Table) error {
	return saves.Save(TableFileName, table)
}

func Ordinal(rank int) string {
	if rank < 0 {
		return ""
	}
	if rank < len(RankOrdinals) {
		return RankOrdinals[rank]
	}
	return fmt.Sprintf("%dTH", rank+1)
}

func FormatThousands(value int) string {
	digits := fmt.Sprint(value)
	grouped := make([]byte, 0, len(digits)+len(digits)/3)
	for index := range len(digits) {
		isGroupStart := index > 0 && (len(digits)-index)%3 == 0 && digits[index-1] != '-'
		grouped = mathx.AppendIf(grouped, ',', isGroupStart)
		grouped = append(grouped, digits[index])
	}
	return string(grouped)
}

func FormatClock(seconds float32) string {
	return fmt.Sprintf("%02d:%02d", int(seconds)/60, int(seconds)%60)
}

func NewNameEntry(initials string, length int, alphabet string) NameEntry {
	entry := NameEntry{Letters: make([]byte, length), Alphabet: mathx.Pick(alphabet == "", alphabet, DefaultAlphabet)}
	copy(entry.Letters, Sanitize(initials, length, entry.Alphabet))
	return entry
}

func (n *NameEntry) Update(letterDelta, cursorDelta int, heldDirection int, isFreshPress bool, deltaSeconds float32) bool {
	n.HoldSeconds = (n.HoldSeconds + deltaSeconds) * mathx.BoolToFloat(heldDirection != 0 && !isFreshPress)
	repeats := mathx.RepeatCount(n.HoldSeconds, letterRepeatDelay, letterRepeatSeconds) - mathx.RepeatCount(n.HoldSeconds-deltaSeconds, letterRepeatDelay, letterRepeatSeconds)
	totalDelta := letterDelta + heldDirection*repeats
	n.shiftLetter(totalDelta)
	n.Cursor = mathx.ClampInt(n.Cursor+cursorDelta, 0, len(n.Letters)-1)
	return totalDelta != 0 || cursorDelta != 0
}

func (n *NameEntry) Advance() bool {
	isLast := n.Cursor >= len(n.Letters)-1
	n.Cursor = min(n.Cursor+1, len(n.Letters)-1)
	return isLast
}

func (n *NameEntry) Initials() string {
	return string(n.Letters)
}

func Sanitize(initials string, length int, alphabet string) string {
	isValid := len(initials) == length
	for index := range min(len(initials), length) {
		isValid = isValid && indexOf(alphabet, initials[index]) >= 0
	}
	fallback := make([]byte, length)
	for index := range fallback {
		fallback[index] = alphabet[0]
	}
	return mathx.Pick(isValid, string(fallback), initials)
}

// ===== Internal =====

func (n *NameEntry) shiftLetter(delta int) {
	letter := &n.Letters[n.Cursor]
	size := len(n.Alphabet)
	*letter = n.Alphabet[mathx.WrapIndex(max(0, indexOf(n.Alphabet, *letter))+delta, size)]
}

func indexOf(alphabet string, letter byte) int {
	for index := range len(alphabet) {
		if alphabet[index] == letter {
			return index
		}
	}
	return -1
}
