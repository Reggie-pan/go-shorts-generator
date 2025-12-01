# GoShortsGenerator

一個自動化影片生成平台，專為快速製作短影音 (Shorts) 而設計。透過整合先進的 AI 語言模型與語音合成技術，使用者僅需提供腳本與素材，系統即可自動完成斷句、配音、字幕生成與影片合成，大幅縮短內容創作週期。

## 核心價值

*   **自動化流程**：一鍵完成從腳本到成片的複雜工序。
*   **AI 賦能**：利用 Gemini AI 進行精準斷句，搭配 Azure TTS 生成自然語音。
*   **高度客製**：支援自訂字幕樣式、背景音樂、轉場效果與模糊背景。
*   **容器化部署**：基於 Docker 架構，部署簡單且環境一致。

## 技術棧

*   **前端**：React v18, Vite, Sass
*   **後端**：Go 1.24 (Gin Framework)
*   **資料儲存**：本地檔案系統 (Local File System)
*   **容器化**：Docker, Docker Compose
*   **核心功能/AI 引擎**：
    *   **LLM**：Google Gemini 2.0 Flash (用於腳本斷句)
    *   **TTS**：Microsoft Azure TTS (支援多國語言與神經網路語音)
    *   **影片處理**：FFmpeg (高效能影片合成與特效處理)

## 前置需求

在開始之前，請確保您的系統已安裝以下軟體：

*   [Docker Engine](https://docs.docker.com/engine/install/)
*   [Docker Compose](https://docs.docker.com/compose/install/)

## 快速啟動指南

1.  **複製專案**

    ```bash
    git clone <repository-url>
    cd video-smith
    ```

2.  **設定環境變數**

    修改 `docker-compose.yml` 中的環境變數，填入您的 API Key：

    ```yaml
    environment:
      - AZURE_TTS_KEY=your_azure_key
      - AZURE_TTS_REGION=your_azure_region
      - GEMINI_API_KEY=your_gemini_key
    ```

3.  **啟動服務**

    使用 Docker Compose 一鍵啟動所有服務：

    ```bash
    docker-compose up -d --build
    ```

4.  **訪問應用程式**

    *   **Web 介面**：[http://localhost:8080](http://localhost:8080)
    *   **API 文件 (Swagger)**：[http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

## 環境變數設定

以下為 `docker-compose.yml` 中的關鍵環境變數說明：

| 變數名稱 | 說明 | 範例值 |
| :--- | :--- | :--- |
| `PORT` | 應用程式服務埠號 | `8080` |
| `STORAGE_PATH` | 任務資料儲存路徑 | `/data` |
| `BGM_PATH` | 背景音樂儲存路徑 | `/assets/bgm` |
| `AZURE_TTS_KEY` | Azure TTS 服務金鑰 (**必填**) | `BQKuPmv...` |
| `AZURE_TTS_REGION` | Azure TTS 服務區域 (**必填**) | `japaneast` |
| `GEMINI_API_KEY` | Google Gemini API 金鑰 (**必填**) | `AIzaSy...` |
| `AI_MODEL` | 使用的 Gemini 模型版本 | `gemini-2.0-flash` |

## 使用方法

1.  **準備素材**：準備好您的影片素材 (圖片或影片) 與背景音樂。
2.  **填寫腳本**：在 Web 介面上輸入您的影片腳本。
3.  **設定參數**：
    *   選擇 **TTS 語音** (支援多國語言)。
    *   設定 **字幕樣式** (字體、顏色、大小)。
    *   調整 **影片設定** (解析度、背景模糊、轉場效果)。
4.  **提交任務**：點擊「建立任務」，系統將自動開始處理。
5.  **下載成品**：待任務完成後，即可預覽並下載生成的影片。

## API 文件

本專案提供完整的 RESTful API，您可以使用 Swagger UI 進行測試與整合：

*   **URL**: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
