package job

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Material struct {
	Type        string  `json:"type"`
	Source      string  `json:"source"`
	PathOrURL   string  `json:"path_or_url"`
	DurationSec float64 `json:"duration_sec"`
}

type TTSSetting struct {
	Provider string  `json:"provider"`
	Voice    string  `json:"voice"`
	Locale   string  `json:"locale"`
	Speed    float64 `json:"speed"`
	Pitch    float64 `json:"pitch"`
}

type VideoSetting struct {
	Resolution string `json:"resolution"`
	FPS        int    `json:"fps"`
}

type BGMSetting struct {
	Source     string  `json:"source"`
	PathOrName string  `json:"path_or_url_or_name"`
	Volume     float64 `json:"volume"`
}

type SubtitleStyle struct {
	Font         string `json:"font"`
	Size         int    `json:"size"`
	Color        string `json:"color"`
	YOffset      int    `json:"y_offset"`
	MaxLineWidth int    `json:"max_line_width"`
}

type JobCreateRequest struct {
	Script        string        `json:"script"`
	Materials     []Material    `json:"materials"`
	TTS           TTSSetting    `json:"tts"`
	Video         VideoSetting  `json:"video"`
	BGM           BGMSetting    `json:"bgm"`
	SubtitleStyle SubtitleStyle `json:"subtitle_style"`
}

type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

type Record struct {
	ID           string           `json:"id"`
	Status       Status           `json:"status"`
	Progress     int              `json:"progress"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	ErrorMessage string           `json:"error_message"`
	ResultURL    string           `json:"result_url"`
	Request      JobCreateRequest `json:"request"`
	BasePath     string           `json:"-"`
}

func (r *JobCreateRequest) Validate() error {
	if strings.TrimSpace(r.Script) == "" {
		return errors.New("腳本不可空白")
	}
	if len(r.Materials) == 0 {
		return errors.New("至少需要一個素材")
	}
	for _, m := range r.Materials {
		if m.DurationSec <= 0 {
			return errors.New("素材時長必須大於 0")
		}
	}
	if r.SubtitleStyle.Size == 0 {
		r.SubtitleStyle.Size = 36
	}
	if r.Video.Resolution == "" {
		r.Video.Resolution = "1080x1920"
	}
	if r.Video.FPS == 0 {
		r.Video.FPS = 30
	}
	if r.TTS.Speed == 0 {
		r.TTS.Speed = 1.0
	}
	if r.BGM.Volume == 0 {
		r.BGM.Volume = 0.25
	}
	if r.SubtitleStyle.Font == "" {
		r.SubtitleStyle.Font = "NotoSansTC"
	}
	if r.SubtitleStyle.Color == "" {
		r.SubtitleStyle.Color = "FFFFFF"
	}
	if r.SubtitleStyle.MaxLineWidth == 0 {
		r.SubtitleStyle.MaxLineWidth = 16
	}
	return nil
}

func NewJobRecord(req JobCreateRequest) (*Record, error) {
	now := time.Now()
	id := uuid.NewString()
	base := "/data/jobs/" + id
	return &Record{
		ID:        id,
		Status:    StatusPending,
		Progress:  0,
		CreatedAt: now,
		UpdatedAt: now,
		Request:   req,
		BasePath:  base,
	}, nil
}
