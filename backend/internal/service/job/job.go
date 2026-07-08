package job

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Material struct {
	Type        string  `json:"type"`
	Source      string  `json:"source"` // "upload", "url"
	Path        string  `json:"path"`   // 上傳後的路徑或 URL
	DurationSec float64 `json:"duration_sec"`
	Mute        bool    `json:"mute"`   // 是否靜音 (僅 video 有效)
	Volume      float64 `json:"volume"` // 音量 0~1 (僅 video 且不靜音有效)
	Effect      string  `json:"effect"` // 運鏡特效 (僅 image 有效)
}

type TTSSetting struct {
	Provider string  `json:"provider"`
	Voice    string  `json:"voice"`
	Locale   string  `json:"locale"`
	Speed    float64 `json:"speed"`
	Pitch    float64 `json:"pitch"`
}

type VideoSetting struct {
	Resolution     string `json:"resolution"` // e.g. "1920x1080"
	FPS            int    `json:"fps"`
	Background     string `json:"background"`      // e.g. "000000"
	BlurBackground bool   `json:"blur_background"` // 是否使用模糊背景
	Transition     string `json:"transition"`      // e.g. "none", "fade", "wipeleft"
}

type BGMSetting struct {
	Source string  `json:"source"`
	Path   string  `json:"path"`
	Volume float64 `json:"volume"`
}

type SubtitleStyle struct {
	Font         string  `json:"font"`
	Size         int     `json:"size"`
	Color        string  `json:"color"`
	YOffset      int     `json:"y_offset"`
	OutlineWidth float64 `json:"outline_width"`
	OutlineColor string  `json:"outline_color"`
	MaxLineWidth int     `json:"max_line_width"`
}

// CoverStyle 封面樣式設定
type CoverStyle struct {
	Enabled         bool          `json:"enabled"`          // 是否啟用封面
	Title           string        `json:"title"`            // 標題文字
	TitleVoice      bool          `json:"title_voice"`      // 是否為標題生成語音
	Duration        float64       `json:"duration"`         // 封面顯示時長（秒），若 TitleVoice=true 則忽略
	ExtendDuration  float64       `json:"extend_duration"`  // 延長時長（秒），封面結束後延長幾秒才開始腳本
	BackgroundType  string        `json:"background_type"`  // 背景類型：gradient / solid / image
	BackgroundBlur  bool          `json:"background_blur"`  // 是否模糊背景
	GradientColors  []string      `json:"gradient_colors"`  // 漸層顏色（當 BackgroundType=gradient）
	BackgroundImage string        `json:"background_image"` // 背景圖片路徑或 URL（當 BackgroundType=image）
	TitleStyle      SubtitleStyle `json:"title_style"`      // 標題文字樣式
}

// ProgressBar 動態圖片進度條設定
type ProgressBar struct {
	Enabled   bool   `json:"enabled"`    // 是否啟用動態圖片進度條
	ImagePath string `json:"image_path"` // 圖片路徑（上傳後的本地路徑）或 URL
	Direction string `json:"direction"`  // 移動方向：top, bottom, left, right
}

// AddProgressBarRequest 獨立進度條處理請求
type AddProgressBarRequest struct {
	VideoURL    string      `json:"video_url"`    // 來源影片 URL
	ProgressBar ProgressBar `json:"progress_bar"` // 進度條設定
}

// JobCreateRequest 建立影片任務的請求參數
type JobCreateRequest struct {
	Script                 string        `json:"script"`                   // 影片腳本內容
	Materials              []Material    `json:"materials"`                // 素材列表（圖片或影片）
	TTS                    TTSSetting    `json:"tts"`                      // 語音合成設定
	Video                  VideoSetting  `json:"video"`                    // 影片輸出設定
	BGM                    BGMSetting    `json:"bgm"`                      // 背景音樂設定
	SubtitleStyle          SubtitleStyle `json:"subtitle_style"`           // 字幕樣式設定
	CoverStyle             CoverStyle    `json:"cover_style"`              // 封面樣式設定
	ProgressBar            ProgressBar   `json:"progress_bar"`             // 動態圖片進度條設定
	AutoDistributeDuration bool          `json:"auto_distribute_duration"` // 是否自動平均分配素材時長（根據語音總長度）
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
		return errors.New("素材至少要有一個")
	}
	// 當 AutoDistributeDuration=true 時，跳過素材時長驗證
	if !r.AutoDistributeDuration {
		for _, m := range r.Materials {
			if m.DurationSec <= 0 {
				return errors.New("素材時長必須大於 0")
			}
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
	if r.BGM.Source != "upload" && r.BGM.Source != "url" && r.BGM.Source != "preset" && r.BGM.Source != "none" {
		return fmt.Errorf("bgm.source must be upload, url, preset or none")
	}
	if r.BGM.Source != "none" && r.BGM.Path == "" {
		return fmt.Errorf("bgm.path is required when source is not none")
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
	if r.SubtitleStyle.OutlineWidth == 0 {
		r.SubtitleStyle.OutlineWidth = 0.1
	}
	if r.SubtitleStyle.OutlineColor == "" {
		r.SubtitleStyle.OutlineColor = "000000"
	}
	// CoverStyle 預設值
	if r.CoverStyle.Enabled {
		if r.CoverStyle.Duration <= 0 && !r.CoverStyle.TitleVoice {
			r.CoverStyle.Duration = 2.0 // 預設 2 秒
		}
		if r.CoverStyle.BackgroundType == "" {
			r.CoverStyle.BackgroundType = "gradient"
		}
		if r.CoverStyle.BackgroundType != "gradient" && r.CoverStyle.BackgroundType != "solid" && r.CoverStyle.BackgroundType != "image" {
			return fmt.Errorf("cover_style.background_type 必須是 gradient、solid 或 image")
		}
		if r.CoverStyle.BackgroundType == "image" && r.CoverStyle.BackgroundImage == "" {
			return fmt.Errorf("cover_style.background_image 在背景類型為 image 時必填")
		}
	}
	// ProgressBar 驗證
	if r.ProgressBar.Enabled {
		if strings.TrimSpace(r.ProgressBar.ImagePath) == "" {
			return errors.New("啟用進度條時，圖片路徑不可為空")
		}
		if r.ProgressBar.Direction == "" {
			r.ProgressBar.Direction = "bottom"
		}
		validDirections := map[string]bool{"top": true, "bottom": true, "left": true, "right": true}
		if !validDirections[r.ProgressBar.Direction] {
			return fmt.Errorf("無效的進度條方向: %s", r.ProgressBar.Direction)
		}
	}
	return nil
}

func NewJobRecord(req JobCreateRequest, storagePath string) (*Record, error) {
	now := time.Now()
	id := uuid.NewString()
	if storagePath == "" {
		storagePath = "/data"
	}
	base := filepath.Join(storagePath, "jobs", id)
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
