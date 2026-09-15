package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type whisperToken struct {
	Text    string `json:"text"`
	Offsets struct {
		From int `json:"from"`
		To   int `json:"to"`
	} `json:"offsets"`
}

type whisperSegmentFull struct {
	Text   string         `json:"text"`
	Tokens []whisperToken `json:"tokens"`
}

type whisperFullOutput struct {
	Transcription []whisperSegmentFull `json:"transcription"`
}

type subtitleWord struct {
	Text    string
	StartMs int // relatif terhadap awal clip
	EndMs   int
}

// GenerateASSSubtitle membaca transcript JSON mentah dari whisper, memfilter token
// yang berada dalam range [clipStartMs, clipEndMs], lalu menulis file .ass
// dengan waktu yang sudah disesuaikan relatif ke awal clip (mulai dari 0).
func GenerateASSSubtitle(rawJSONPath string, clipStartMs, clipEndMs int, outputPath string) error {
	raw, err := os.ReadFile(rawJSONPath)
	if err != nil {
		return fmt.Errorf("failed to read raw transcript: %w", err)
	}

	var full whisperFullOutput
	if err := json.Unmarshal(raw, &full); err != nil {
		return fmt.Errorf("failed to parse raw transcript: %w", err)
	}

	var words []subtitleWord
	for _, seg := range full.Transcription {
		for _, tok := range seg.Tokens {
			text := strings.TrimSpace(tok.Text)
			if text == "" {
				continue
			}
			// skip special token seperti [_BEG_], [_TT_123], dll
			if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
				continue
			}
			// skip token yang tidak overlap dengan range clip
			if tok.Offsets.To <= clipStartMs || tok.Offsets.From >= clipEndMs {
				continue
			}

			startRel := tok.Offsets.From - clipStartMs
			endRel := tok.Offsets.To - clipStartMs
			if startRel < 0 {
				startRel = 0
			}
			if endRel > clipEndMs-clipStartMs {
				endRel = clipEndMs - clipStartMs
			}

			words = append(words, subtitleWord{Text: text, StartMs: startRel, EndMs: endRel})
		}
	}

	lines := groupWordsIntoLines(words, 4) // 4 kata per baris subtitle

	assContent := buildASSFile(lines)

	if err := os.WriteFile(outputPath, []byte(assContent), 0644); err != nil {
		return fmt.Errorf("failed to write ass file: %w", err)
	}

	return nil
}

type subtitleLine struct {
	Text    string
	StartMs int
	EndMs   int
}

func groupWordsIntoLines(words []subtitleWord, wordsPerLine int) []subtitleLine {
	var lines []subtitleLine
	for i := 0; i < len(words); i += wordsPerLine {
		end := i + wordsPerLine
		if end > len(words) {
			end = len(words)
		}
		chunk := words[i:end]
		if len(chunk) == 0 {
			continue
		}

		var texts []string
		for _, w := range chunk {
			texts = append(texts, w.Text)
		}

		lines = append(lines, subtitleLine{
			Text:    strings.Join(texts, " "),
			StartMs: chunk[0].StartMs,
			EndMs:   chunk[len(chunk)-1].EndMs,
		})
	}
	return lines
}

func buildASSFile(lines []subtitleLine) string {
	var sb strings.Builder

	sb.WriteString(`[Script Info]
ScriptType: v4.00+
PlayResX: 1080
PlayResY: 1920
WrapStyle: 0

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, OutlineColour, BackColour, Bold, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial Black,64,&H00FFFFFF,&H00000000,&H80000000,1,3,0,2,60,60,200,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`)

	for _, line := range lines {
		start := msToAssTime(line.StartMs)
		end := msToAssTime(line.EndMs)
		sb.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", start, end, line.Text))
	}

	return sb.String()
}

// msToAssTime mengubah milidetik jadi format ASS: H:MM:SS.cc (centisecond)
func msToAssTime(ms int) string {
	if ms < 0 {
		ms = 0
	}
	totalCs := ms / 10
	h := totalCs / 360000
	m := (totalCs % 360000) / 6000
	s := (totalCs % 6000) / 100
	cs := totalCs % 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}