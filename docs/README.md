# GoShortsGenerator 使用說明（CPU 版）

## 系統特色
- Docker 內即可運行，無 GPU 依賴。
- 以任務 API 觸發，背景 worker 自動執行。
- 支援圖片/影片 URL 或上傳路徑、TTS（Google/Azure/免費 espeak）、字幕樣式、背景音樂。

## 環境變數
- `PORT`：後端埠號，預設 8080
- `STORAGE_PATH`：任務資料儲存目錄，預設 `/data`
- `BGM_PATH`：預設音樂目錄，預設 `/assets/bgm`
- `GOOGLE_TTS_KEY`：Google TTS API Key
- `AZURE_TTS_KEY`、`AZURE_TTS_REGION`：Azure 語音設定
- `FREE_TTS_MODEL_PATH`：保留給其他離線模型

## 快速啟動
```bash
docker compose up --build
# 或
make build && make run
```
（請確保 `assets/bgm/default.mp3` 存在，否則背景音樂將自動略過）

前端：`http://localhost:8080`  
API：`http://localhost:8080/api/v1`
Swagger JSON：`http://localhost:8080/api/v1/swagger.json`

## API 範例
### 建立任務
```
POST /api/v1/jobs
Content-Type: application/json
{
  "script": "這是一段腳本。第二句。",
  "materials": [
    {"type":"image","source":"url","path_or_url":"https://picsum.photos/720/1280","duration_sec":3},
    {"type":"video","source":"url","path_or_url":"https://sample-videos.com/video321/mp4/720/big_buck_bunny_720p_1mb.mp4","duration_sec":5}
  ],
  "tts":{"provider":"free","voice":"","locale":"en","speed":1,"pitch":0},
  "video":{"resolution":"1080x1920","fps":30,"speed":1},
  "bgm":{"source":"preset","path_or_url_or_name":"default.mp3","volume":0.2},
  "subtitle_style":{"font":"NotoSansTC","size":36,"color":"FFFFFF","y_offset":40,"max_line_width":24}
}
```

### 查詢狀態
```
GET /api/v1/jobs/{id}
```

### 列表
```
GET /api/v1/jobs?page=1&limit=20
```

### 下載成品
```
GET /api/v1/jobs/{id}/result
```

### 取消 / 刪除
```
POST /api/v1/jobs/{id}/cancel
DELETE /api/v1/jobs/{id}
```

### 列出預設 BGM
```
GET /api/v1/presets/bgm
```

## 時間軸策略
- 無 TTS provider 時間戳時，使用各句音檔長度累加為字幕時間，確保逐句對齊。
- 素材時間軸若不足則重複最後一個素材補滿（最安全且符合短影片常用做法）。

## 字幕規則
- `autoSpacing` 自動在中英/數字交界插入空格。
- 長度超過 `max_line_width` 會自動切分。
- ASS 樣式可調整字型/大小/顏色/Y 偏移。

## 測試
```bash
cd backend
go test ./...          # 單元測試：autoSpacing/斷句/時間軸
RUN_E2E=1 go test ./... -run ProcessPipeline  # 整合測試（需 ffmpeg+espeak）
```

## 一鍵驗證（端到端）
```bash
docker run --rm -it -p 8080:8080 \
  -v ${PWD}/data:/data -v ${PWD}/assets:/assets \
  -e PORT=8080 -e STORAGE_PATH=/data -e BGM_PATH=/assets/bgm \
  goshortsgenerator
```
預期：前端可開啟、透過範例腳本建立任務，數分鐘內產生短影片，可下載 MP4。
