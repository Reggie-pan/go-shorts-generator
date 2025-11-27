package worker

import (
	"fmt"
	"strings"
	"testing"
	"video-smith/backend/internal/service/job"
)

func TestVideoFilterConstruction(t *testing.T) {
	style := job.SubtitleStyle{
		Font:         "Noto Sans TC",
		Size:         50,
		Color:        "FFFFFF",
		YOffset:      100,
		MaxLineWidth: 20,
	}
	subPathFF := "/path/to/subtitle.ass"

	color := strings.TrimPrefix(style.Color, "#")
	if len(color) == 6 {
		r := color[0:2]
		g := color[2:4]
		b := color[4:6]
		color = b + g + r
	}

	videoFilter := fmt.Sprintf("subtitles='%s':force_style='FontName=%s,FontSize=%d,PrimaryColour=&H%s&,MarginV=%d,Outline=4,Shadow=1,BorderStyle=1,Alignment=2'",
		subPathFF, style.Font, style.Size, color, style.YOffset)

	expected := "subtitles='/path/to/subtitle.ass':force_style='FontName=Noto Sans TC,FontSize=50,PrimaryColour=&HFFFFFF&,MarginV=100,Outline=4,Shadow=1,BorderStyle=1,Alignment=2'"

	// Let's check the output
	t.Logf("Video Filter: %s", videoFilter)

	if videoFilter != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, videoFilter)
	}

	// Check if parameters are present
	if !strings.Contains(videoFilter, "FontSize=50") {
		t.Errorf("FontSize not found or incorrect")
	}
	if !strings.Contains(videoFilter, "MarginV=100") {
		t.Errorf("MarginV not found or incorrect")
	}
}
