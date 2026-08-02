package media

import (
	"strings"
	"testing"
)

func TestParseResolution(t *testing.T) {
	w, h := ParseResolution("16:9", 0, 0)
	if w != 1920 || h != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", w, h)
	}

	w, h = ParseResolution("9:16", 0, 0)
	if w != 1080 || h != 1920 {
		t.Errorf("expected 1080x1920, got %dx%d", w, h)
	}

	w, h = ParseResolution("", 720, 1280)
	if w != 720 || h != 1280 {
		t.Errorf("expected 720x1280, got %dx%d", w, h)
	}
}

func TestFormatFFmpegColor(t *testing.T) {
	if col := FormatFFmpegColor("#FF0000"); col != "0xFF0000" {
		t.Errorf("expected 0xFF0000, got %s", col)
	}
	if col := FormatFFmpegColor("00FF00"); col != "0x00FF00" {
		t.Errorf("expected 0x00FF00, got %s", col)
	}
	if col := FormatFFmpegColor(""); col != "0x000000" {
		t.Errorf("expected 0x000000, got %s", col)
	}
}

func TestBuildConvertAspectRatioFilter(t *testing.T) {
	// 1. Color filter
	filter, isComplex := BuildConvertAspectRatioFilter("color", "#000000", 1080, 1920)
	if isComplex {
		t.Errorf("color filter should not be complex")
	}
	if !strings.Contains(filter, "pad=1080:1920") {
		t.Errorf("filter missing pad parameters: %s", filter)
	}

	// 2. Blur filter
	filter, isComplex = BuildConvertAspectRatioFilter("blur", "", 1080, 1920)
	if !isComplex {
		t.Errorf("blur filter should be complex")
	}
	if !strings.Contains(filter, "boxblur=20:5") {
		t.Errorf("filter missing boxblur: %s", filter)
	}

	// 3. Crop filter
	filter, isComplex = BuildConvertAspectRatioFilter("crop", "", 1080, 1920)
	if isComplex {
		t.Errorf("crop filter should not be complex")
	}
	if !strings.Contains(filter, "crop=1080:1920") {
		t.Errorf("filter missing crop: %s", filter)
	}
}
