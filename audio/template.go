package audio

import _ "embed"

// ===== Constants =====

//go:embed template.trk
var templateSource string

// ===== Public API =====

func TemplateSource() string {
	return templateSource
}

func DefaultTemplate() *Song {
	song, err := ParseSong(templateSource)
	if err != nil {
		panic(err)
	}
	return song
}
