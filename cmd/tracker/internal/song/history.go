package song

// ===== Types =====

type History struct {
	undo []*Document
	redo []*Document
}

// ===== Constants =====

const historyDepth = 200

// ===== Public API =====

func (h *History) Record(document *Document) {
	h.undo = append(h.undo, document.Clone())
	h.undo = h.undo[max(0, len(h.undo)-historyDepth):]
	h.redo = h.redo[:0]
}

func (h *History) Undo(current *Document) *Document {
	if len(h.undo) == 0 {
		return current
	}
	previous := h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	h.redo = append(h.redo, current.Clone())
	return previous
}

func (h *History) Redo(current *Document) *Document {
	if len(h.redo) == 0 {
		return current
	}
	next := h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	h.undo = append(h.undo, current.Clone())
	return next
}

func (h *History) Clear() {
	h.undo, h.redo = h.undo[:0], h.redo[:0]
}
