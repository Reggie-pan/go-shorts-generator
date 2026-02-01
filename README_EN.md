# GoShortsGenerator

<div align="center">

#### [中文](README.md) | English

</div>

![Brand Banner](assets/images/banner.jpg)

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.24-blue?style=flat-square&logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react)](https://reactjs.org/)
[![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat-square&logo=docker)](https://www.docker.com/)
[![Docker Hub](https://img.shields.io/docker/pulls/reggiepan/goshortsgenerator?style=flat-square&logo=docker)](https://hub.docker.com/r/reggiepan/goshortsgenerator)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

**Fully Automated Short Video Generation Platform**

[Features](#features-) • [Quick Start](#quick-start-) • [Requirements](#requirements-) • [Usage](#usage-) • [API Docs](#api-docs-)

</div>

---

**GoShortsGenerator** is an automated platform designed for quickly creating short videos (Shorts). By integrating advanced AI language models and speech synthesis technology, users simply need to provide a script and assets, and the system automatically handles sentence segmentation, voiceover, subtitle generation, and video synthesis, significantly reducing the content creation cycle.

<div align="center">
  <img src="assets/images/ui_screenshot.png" alt="UI Screenshot">
</div>

## Features 🎯

*   **🤖 Fully Automated Workflow**: One-click completion of complex processes from script to final video, with no manual intervention required.
*   **🧠 AI Powered**:
    *   Integrates **Google Gemini** (default: `gemini-2.5-flash-lite`) for precise script segmentation and semantic analysis.
    *   Supports **Edge TTS** (Free), **Azure TTS v1**, and **Azure TTS v2** to generate natural, fluid neural network speech.
*   **🎨 Highly Customizable**:
    *   Supports custom subtitle styles (font, color, size, outline).
    *   Freely mix background music, transition effects, and background blur processing.
    *   Multiple camera effects available: `zoom_in`, `zoom_out`, `pan_left`, `pan_right`, `pan_up`, `pan_down`, `diagonal_pan`, `rotate`, `shake`.
*   **🎬 Title Cover**:
    *   Supports automatic generation of video opening covers with title text and gradient/blur/image backgrounds.
    *   Optional title voice, with cover duration automatically adjusted based on voice length.
*   **⏱️ Auto Duration Distribution**:
    *   When enabled, the system automatically distributes material durations evenly based on total voice duration.
*   **🐳 Containerized Deployment**: Built on Docker architecture for simple deployment and consistent environments.

## Tech Stack 🛠️

| Area | Technology |
| :--- | :--- |
| **Frontend** | React v18, Vite, Sass |
| **Backend** | Go 1.24 (Gorilla Mux) |
| **Data Storage** | Local File System |
| **Containerization** | Docker, Docker Compose ([Docker Hub](https://hub.docker.com/r/reggiepan/goshortsgenerator)) |
| **AI Engine** | Google Gemini (LLM), Edge TTS / Microsoft Azure TTS |
| **Video Processing** | FFmpeg |

## Quick Start 🚀

### 1. Clone Project

```bash
git clone https://github.com/Reggie-pan/go-shorts-generator.git
cd go-shorts-generator
```

### 2. Set Environment Variables

Modify `docker-compose.yml` and fill in your API Keys:

```yaml
environment:
  - PORT=8080
  - STORAGE_PATH=/data
  - BGM_PATH=/assets/bgm
  - TZ=Asia/Taipei                     # Timezone setting
  - AZURE_TTS_KEY=your_azure_key       # Optional (Not required if using Edge TTS)
  - AZURE_TTS_REGION=your_azure_region # Optional (Not required if using Edge TTS)
  - GEMINI_API_KEY=your_gemini_key     # Required
  - AI_MODEL=gemini-2.5-flash-lite     # Gemini Model Version
```

### 3. Start Services

Start with one click using Docker Compose:

```bash
docker-compose up -d --build
```

### 4. Access Application

*   **Web Interface**: [http://localhost:8080](http://localhost:8080)
*   **API Docs**: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

## Requirements 📦

The following are descriptions of key environment variables in `docker-compose.yml`:

| Variable Name | Description | Required | Example Value |
| :--- | :--- | :---: | :--- |
| `PORT` | Application service port | ○ | `8080` |
| `STORAGE_PATH` | Task data storage path | ○ | `/data` |
| `BGM_PATH` | Background music storage path | ○ | `/assets/bgm` |
| `TZ` | Timezone setting | ○ | `Asia/Taipei` |
| `GEMINI_API_KEY` | Google Gemini API Key | ✓ | `AIza...` |
| `AI_MODEL` | Gemini Model Version | ○ | `gemini-2.5-flash-lite` |
| `AZURE_TTS_KEY` | Azure TTS Service Key | ✗ | `...` |
| `AZURE_TTS_REGION` | Azure TTS Service Region | ✗ | `japaneast` |

> ✓ Required　✗ Optional (Not required if using Edge TTS)　○ Default value available

## Usage 📖

1.  **Prepare Assets** 📂
    *   Prepare your video assets (images or videos) and background music.
    
2.  **Write Script** ✍️
    *   Enter your video script on the Web interface.

3.  **Configure Settings** ⚙️
    *   Select **TTS Voice Provider** (`edge_tts`, `azure_v1`, `azure_v2`).
    *   Set **Subtitle Style** (font, color, size, outline).
    *   Adjust **Video Settings** (resolution, fps, background blur, transition effects).
    *   Optionally enable **Auto Duration Distribution**.
    *   Configure **Cover Style** (title, background type, title voice generation).

4.  **Submit Task** ▶️
    *   Click "Create Task" and the system will automatically start processing.

5.  **Download Result** 🎬
    *   Once the task is complete, you can preview and download the generated video.

## API Docs 📄

This project provides a complete RESTful API for developers to extend or integrate:

<img src="assets/images/api_screenshot.png" alt="API Screenshot">

## License 📝

This project is licensed under the [MIT License](LICENSE).
