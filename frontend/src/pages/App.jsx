import React, { useEffect, useState } from 'react'
import api from '../services/api'
import MaterialList from '../components/MaterialList'
import SearchableSelect from '../components/SearchableSelect'
import ToastContainer from '../components/ToastContainer'
import ConfirmDialog from '../components/ConfirmDialog'

const blankMaterial = { type: 'image', source: 'url', path: '', duration_sec: 3 }
const defaultRequest = {
  script: '這是一段示範腳本。\n第二句會跟字幕同步。',
  materials: [{ ...blankMaterial, path: 'https://picsum.photos/720/1280', duration_sec: 3 }],
  tts: { provider: 'azure_v1', voice: '', locale: 'en-US', speed: 1, pitch: 0 },
  video: { resolution: '1080x1920', fps: 30, speed: 1, background: '000000' },
  bgm: { source: 'preset', path: 'default.mp3', volume: 0.2 },
  subtitle_style: { font: 'Noto Sans TC', size: 16, color: 'FFFFFF', y_offset: 70, max_line_width: 16, outline_width: 0.1, outline_color: '000000' }
}

export default function App() {
  const [form, setForm] = useState(defaultRequest)
  const [jobs, setJobs] = useState([])
  const [bgmList, setBgmList] = useState([])
  const [fontList, setFontList] = useState([])
  const [voiceList, setVoiceList] = useState([])
  const [previewImage, setPreviewImage] = useState(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [bgmPlaying, setBgmPlaying] = useState(false)
  const audioRef = React.useRef(null)
  
  // Toast Notification System
  const [toasts, setToasts] = useState([])

  // Custom Confirm Dialog
  const [confirmModal, setConfirmModal] = useState({ show: false, message: '', onConfirm: null })

  // About Modal
  const [showAboutModal, setShowAboutModal] = useState(false)

  const addToast = (type, message) => {
    const id = Date.now()
    setToasts(prev => [...prev, { id, type, message }])
    setTimeout(() => {
      setToasts(prev => prev.filter(t => t.id !== id))
    }, 3000)
  }

  const toggleBgm = (filename) => {
    if (!filename) return

    const url = `/assets/bgm/${filename}`
    
    if (bgmPlaying) {
      audioRef.current.pause()
      setBgmPlaying(false)
      // 如果是同一首，就只是暫停；如果是不同首，則切換
      if (!audioRef.current.src.endsWith(encodeURI(filename))) {
        audioRef.current = new Audio(url)
        audioRef.current.onended = () => setBgmPlaying(false)
        audioRef.current.play()
        setBgmPlaying(true)
      }
    } else {
      if (!audioRef.current || !audioRef.current.src.endsWith(encodeURI(filename))) {
        audioRef.current = new Audio(url)
        audioRef.current.onended = () => setBgmPlaying(false)
      }
      audioRef.current.play()
      setBgmPlaying(true)
    }
  }

  const generatePreview = async () => {
    try {
      setPreviewLoading(true)
      // 優化：只取第一行或前 8 個字
      let previewText = form.script ? form.script.split('\n')[0] : '預覽文字 Preview'
      if (previewText.length > 8) {
        previewText = previewText.slice(0, 8)
      }
      
      const url = await api.previewSubtitle({
        text: previewText,
        style: form.subtitle_style,
        background: form.video.background,
        resolution: form.video.resolution
      })
      if (previewImage) {
        URL.revokeObjectURL(previewImage)
      }
      setPreviewImage(url)
    } catch (err) {
      console.error('預覽失敗', err)
      addToast('error', '產生預覽圖失敗')
    } finally {
      setPreviewLoading(false)
    }
  }

  const loadJobs = async () => {
    const res = await api.listJobs()
    const jobsList = res.data || []
    // 按建立時間排序，新的在前面
    const sortedJobs = jobsList.sort((a, b) => {
      const timeA = new Date(a.created_at).getTime()
      const timeB = new Date(b.created_at).getTime()
      return timeB - timeA // 降序排列（新 → 舊）
    })
    setJobs(sortedJobs)
  }
  const loadBgm = async () => {
    const res = await api.listBGM()
    setBgmList(res.data || [])
  }
  const loadFonts = async () => {
    const res = await api.listFonts()
    setFontList(res.data || [])
  }

  useEffect(() => {
    loadJobs()
    loadBgm()
    loadFonts()
    const id = setInterval(loadJobs, 4000)
    return () => clearInterval(id)
  }, [])

  useEffect(() => {
    const handleMouseMove = (e) => {
      const x = e.clientX
      const y = e.clientY
      document.documentElement.style.setProperty('--cursor-x', `${x}px`)
      document.documentElement.style.setProperty('--cursor-y', `${y}px`)
    }

    window.addEventListener('mousemove', handleMouseMove)
    return () => window.removeEventListener('mousemove', handleMouseMove)
  }, [])

  // Fetch voices when provider changes
  useEffect(() => {
    const fetchVoices = async () => {
      try {
        const res = await api.listVoices(form.tts.provider)
        setVoiceList(res.data || [])
      } catch (err) {
        console.error('Failed to load voices', err)
        setVoiceList([])
      }
    }
    fetchVoices()
  }, [form.tts.provider])

  const updateMaterial = (index, field, value) => {
    const newMaterials = [...form.materials]
    newMaterials[index][field] = value
    setForm({ ...form, materials: newMaterials })
  }
  const addMaterial = () => setForm({ ...form, materials: [...form.materials, { ...blankMaterial }] })
  const removeMaterial = (index) => setForm({ ...form, materials: form.materials.filter((_, i) => i !== index) })
  
  const moveMaterial = (index, direction) => {
    const newMaterials = [...form.materials]
    const targetIndex = index + direction
    if (targetIndex >= 0 && targetIndex < newMaterials.length) {
      const temp = newMaterials[index]
      newMaterials[index] = newMaterials[targetIndex]
      newMaterials[targetIndex] = temp
      setForm({ ...form, materials: newMaterials })
    }
  }

  const submit = async () => {
    try {
      await api.createJob(form)
      addToast('success', '任務建立成功')
      setForm({ ...form }) // Don't reset script
      loadJobs()
    } catch (err) {
      addToast('error', '建立失敗: ' + (err.response?.data?.error || err.message))
    }
  }

  const cancel = async (id) => {
    await api.cancelJob(id)
    await loadJobs()
  }

  const remove = async (id) => {
    await api.deleteJob(id)
    await loadJobs()
  }

  const removeAll = async () => {
    setConfirmModal({
      show: true,
      message: '確定要刪除所有任務嗎？此動作將會刪除所有任務紀錄與相關檔案，且無法復原。',
      onConfirm: async () => {
        try {
          await api.deleteAllJobs()
          addToast('success', '已刪除所有任務')
          await loadJobs()
        } catch (err) {
          addToast('error', '刪除失敗')
        }
      }
    })
  }

  const finished = (status) => ['success', 'failed', 'canceled'].includes(status)

  const formatTime = (dateString) => {
    if (!dateString) return '-'
    try {
      const date = new Date(dateString)
      return date.toLocaleString('zh-TW', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      })
    } catch {
      return dateString
    }
  }

  // Form Validation
  const isFormValid = () => {
    // 1. Check Script
    if (!form.script || form.script.trim() === '') return false

    // 2. Check Materials
    if (form.materials.length === 0) return false
    for (const m of form.materials) {
      if (!m.path || m.path.trim() === '') return false
      if (m.duration_sec <= 0) return false
    }

    // 3. Check TTS
    if (!form.tts.voice || form.tts.voice === '') return false

    // 4. Check BGM
    if (form.bgm.source !== 'none') {
      if (!form.bgm.path || form.bgm.path.trim() === '') return false
    }

    return true
  }

  const handleCopyTask = (task) => {
    // Deep copy to avoid reference issues
    const newForm = JSON.parse(JSON.stringify(task.request))
    setForm(newForm)
    addToast('success', '參數已複製到表單')
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const handleDuplicateTask = async (task) => {
    try {
      await api.createJob(task.request)
      addToast('success', '任務已再次執行')
      loadJobs()
    } catch (err) {
      addToast('error', '執行失敗: ' + (err.response?.data?.error || err.message))
    }
  }



  return (
    <div className="container">
      <ToastContainer toasts={toasts} />
      <ConfirmDialog 
        show={confirmModal.show}
        message={confirmModal.message}
        onConfirm={() => {
          if (confirmModal.onConfirm) confirmModal.onConfirm()
          setConfirmModal({ ...confirmModal, show: false })
        }}
        onCancel={() => setConfirmModal({ ...confirmModal, show: false })}
      />
      
      {/* About Modal */}
      {showAboutModal && (
        <div className="modal-overlay" onClick={() => setShowAboutModal(false)}>
          <div className="modal-content" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3><i className="fas fa-info-circle" style={{ marginRight: '8px' }}></i> 關於 (About)</h3>
              <button className="close-btn" onClick={() => setShowAboutModal(false)}>&times;</button>
            </div>
            <div className="modal-body">
              <div style={{ marginBottom: '16px' }}>
                <h4 style={{ margin: '0 0 8px 0', color: '#fff' }}>回報問題 (Report a bug)</h4>
                <a href="https://github.com/reggie-pan/go-shorts-generator/issues" target="_blank" rel="noreferrer" className="about-link">
                  <i className="fab fa-github"></i> GitHub Issues
                </a>
              </div>
              <hr style={{ borderColor: 'rgba(255,255,255,0.1)', margin: '16px 0' }} />
              <div>
                <h4 style={{ margin: '0 0 8px 0', color: '#fff' }}>GoShortsGenerator</h4>
                <p>
                  一個自動化影片生成平台，專為快速製作短影音 (Shorts) 而設計。透過整合先進的 AI 語言模型與語音合成技術，使用者僅需提供腳本與素材，系統即可自動完成斷句、配音、字幕生成與影片合成，大幅縮短內容創作週期。
                </p>
                <a href="https://github.com/reggie-pan/go-shorts-generator" target="_blank" rel="noreferrer" className="about-link">
                  <i className="fab fa-github"></i> Project Repository
                </a>
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="app-header">
        <h1 className="header-title">
          <i className="fas fa-video"></i>
          GoShortsGenerator
        </h1>
        <div className="header-actions">
          <a href="/swagger.html" target="_blank" rel="noreferrer" className="btn-text">
            <i className="fas fa-file-code"></i> API 文件
          </a>
          <button 
            onClick={() => setShowAboutModal(true)} 
            className="btn-text" 
          >
            <i className="fas fa-info-circle"></i> 關於
          </button>
        </div>
      </div>

      <div className="card">
        <h2><i className="fas fa-plus-circle"></i> 建立任務</h2>
        <label><i className="fas fa-scroll"></i> 腳本</label>
        <textarea rows="4" value={form.script} onChange={(e) => setForm({ ...form, script: e.target.value })} />

        <MaterialList 
          materials={form.materials}
          onUpdate={updateMaterial}
          onAdd={addMaterial}
          onRemove={removeMaterial}
          onMove={moveMaterial}
        />

        <h3><i className="fas fa-microphone"></i> 語音合成 (TTS)</h3>
        <div className="grid">
          <div>
            <label>Provider</label>
            <select value={form.tts.provider} onChange={(e) => setForm({ ...form, tts: { ...form.tts, provider: e.target.value } })}>
              <option value="azure_v1">azure_v1</option>
              <option value="azure_v2">azure_v2</option>
            </select>
          </div>
          <div>
            <label>Locale (自動帶入)</label>
            <input 
              value={form.tts.locale} 
              disabled
              className="input-disabled"
            />
          </div>
          <div>
            <label>Voice Name</label>
            {voiceList.length > 0 ? (
              <SearchableSelect 
                options={voiceList.map(v => ({ label: v.name, value: v.name }))}
                value={form.tts.voice}
                placeholder="請選擇語音..."
                onChange={(val) => {
                  const selectedVoice = voiceList.find(v => v.name === val)
                  setForm({ 
                    ...form, 
                    tts: { 
                      ...form.tts, 
                      voice: val,
                      locale: selectedVoice ? selectedVoice.locale : form.tts.locale 
                    } 
                  })
                }}
              />
            ) : (
              <input value={form.tts.voice} onChange={(e) => setForm({ ...form, tts: { ...form.tts, voice: e.target.value } })} />
            )}
          </div>
          <div>
            <label>語速</label>
            <input type="number" step="0.1" value={form.tts.speed} onChange={(e) => setForm({ ...form, tts: { ...form.tts, speed: Number(e.target.value) } })} />
          </div>
          <div>
            <label>音高</label>
            <input type="number" step="0.1" value={form.tts.pitch} onChange={(e) => setForm({ ...form, tts: { ...form.tts, pitch: Number(e.target.value) } })} />
          </div>
        </div>

        <h3><i className="fas fa-film"></i> 影片設定</h3>
        <div className="grid">
          <div>
            <label>解析度</label>
            <select value={form.video.resolution} onChange={(e) => setForm({ ...form, video: { ...form.video, resolution: e.target.value } })}>
              <option value="1080x1920">1080x1920 (9:16)</option>
              <option value="720x1280">720x1280 (9:16)</option>
              <option value="1080x1080">1080x1080 (1:1)</option>
              <option value="1920x1080">1920x1080 (16:9)</option>
            </select>
          </div>
          <div>
            <label>FPS</label>
            <input type="number" value={form.video.fps} onChange={(e) => setForm({ ...form, video: { ...form.video, fps: Number(e.target.value) } })} />
          </div>
          <div>
            <label>轉場動畫</label>
            <select value={form.video.transition || 'none'} onChange={(e) => setForm({ ...form, video: { ...form.video, transition: e.target.value } })}>
              <option value="none">無</option>
              <option value="fade">淡入淡出</option>
              <option value="wipeleft">向左擦除</option>
              <option value="wiperight">向右擦除</option>
              <option value="slideleft">向左滑動</option>
              <option value="slideright">向右滑動</option>
              <option value="circleopen">圓形展開</option>
              <option value="circleclose">圓形收縮</option>
            </select>
          </div>
          <div>
            <label><i className="fas fa-palette"></i> 背景顏色</label>
            <div className="color-picker-group">
              <div className="color-preview-wrapper">
                <input 
                  type="color" 
                  value={`#${form.video.background || '000000'}`} 
                  onChange={(e) => setForm({ ...form, video: { ...form.video, background: e.target.value.replace('#', '').toUpperCase() } })} 
                  disabled={form.video.blur_background}
                  className="color-input"
                />
              </div>
              <div className="hex-input-wrapper">
                <div className="hex-input-group">
                  <span>#</span>
                  <input 
                    type="text" 
                    value={form.video.background || '000000'} 
                    onChange={(e) => setForm({ ...form, video: { ...form.video, background: e.target.value.replace('#', '').toUpperCase() } })} 
                    disabled={form.video.blur_background}
                    placeholder="000000"
                    maxLength="6"
                    className="hex-input"
                  />
                </div>
              </div>
            </div>
            <div style={{ marginTop: '8px' }}>
              <label className="checkbox-label">
                <input 
                  type="checkbox" 
                  checked={form.video.blur_background || false} 
                  onChange={(e) => setForm({ ...form, video: { ...form.video, blur_background: e.target.checked } })} 
                />
                模糊背景邊緣 (Blur Edge)
              </label>
            </div>
          </div>
        </div>

        <h3><i className="fas fa-music"></i> 背景音樂 (BGM)</h3>
        <div className="grid">
          <div>
            <label>來源</label>
            <select value={form.bgm.source} onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, source: e.target.value } })}>
              <option value="preset">preset</option>
              <option value="url">url</option>
              <option value="upload">upload(檔案)</option>
              <option value="none">none (無音樂)</option>
            </select>
          </div>
          {form.bgm.source !== 'none' && (
            <div>
              <label>音量(0~1)</label>
              <input type="number" step="0.05" value={form.bgm.volume} onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, volume: Number(e.target.value) } })} />
            </div>
          )}
        </div>
        {form.bgm.source !== 'none' && (
            <div style={{ marginTop: '12px' }}>
              <label>檔名/網址/路徑</label>
              <div className="bgm-input-group">
                {form.bgm.source === 'preset' ? (
                  <select 
                    value={form.bgm.path} 
                    onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, path: e.target.value } })}
                  >
                    {['random', form.bgm.path, ...bgmList].filter((v, i, arr) => v && arr.indexOf(v) === i).map((name) => (
                      <option key={name} value={name}>{name === 'random' ? '隨機 (Random)' : name}</option>
                    ))}
                  </select>
                ) : (
                  <input 
                    value={form.bgm.path} 
                    onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, path: e.target.value } })} 
                    placeholder={form.bgm.source === 'upload' ? "請上傳檔案..." : "請輸入網址..."}
                  />
                )}

                {form.bgm.source === 'preset' && form.bgm.path && (
                  <button 
                    className="btn-secondary btn-play"
                    onClick={() => toggleBgm(form.bgm.path)}
                    title={bgmPlaying ? "停止試聽" : "試聽音樂"}
                  >
                    <i className={bgmPlaying ? "fas fa-stop" : "fas fa-play"}></i>
                  </button>
                )}

                {form.bgm.source === 'upload' && (
                  <label className="btn btn-secondary btn-file-select">
                    <i className="fas fa-cloud-upload-alt"></i>
                    <span>選擇檔案</span>
                    <input 
                      type="file" 
                      style={{ display: 'none' }} 
                      onChange={async (e) => {
                        if (e.target.files[0]) {
                          try {
                            const res = await api.uploadFile(e.target.files[0])
                            setForm({ ...form, bgm: { ...form.bgm, path: res.path } })
                          } catch (err) {
                            alert('上傳失敗')
                          }
                        }
                      }} 
                    />
                  </label>
                )}
              </div>
            </div>
        )}

        <h3><i className="fas fa-closed-captioning"></i> 字幕樣式</h3>
        <div className="grid">
          <div>
            <label>字體</label>
            <select value={form.subtitle_style.font} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, font: e.target.value } })}>
              {fontList.length > 0 ? (
                fontList.map((font) => (
                  <option key={font.name} value={font.name}>{font.name}</option>
                ))
              ) : (
                <option value={form.subtitle_style.font}>{form.subtitle_style.font}</option>
              )}
            </select>
          </div>
          <div>
            <label>字型大小</label>
            <input type="number" value={form.subtitle_style.size} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, size: Number(e.target.value) } })} />
          </div>
          <div>
            <label>Y Offset (高度)</label>
            <input type="number" value={form.subtitle_style.y_offset} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, y_offset: Number(e.target.value) } })} />
          </div>
          <div>
            <label><i className="fas fa-palette"></i> 顏色</label>
            <div className="color-picker-group">
              <div className="color-preview-wrapper">
                <input 
                  type="color" 
                  value={`#${form.subtitle_style.color || 'FFFFFF'}`} 
                  onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, color: e.target.value.replace('#', '').toUpperCase() } })} 
                  className="color-input"
                />
              </div>
              <div className="hex-input-wrapper">
                <div className="hex-input-group">
                  <span>#</span>
                  <input 
                    type="text" 
                    value={form.subtitle_style.color || 'FFFFFF'} 
                    onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, color: e.target.value.replace('#', '').toUpperCase() } })} 
                    placeholder="FFFFFF"
                    maxLength="6"
                    className="hex-input"
                  />
                </div>
              </div>
            </div>
          </div>
          <div>
            <label>單行最大字數</label>
            <input type="number" value={form.subtitle_style.max_line_width} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, max_line_width: Number(e.target.value) } })} />
          </div>
          <div>
            <label>邊框寬度</label>
            <input type="number" step="0.1" value={form.subtitle_style.outline_width} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, outline_width: Number(e.target.value) } })} />
          </div>
          <div>
            <label><i className="fas fa-palette"></i> 邊框顏色</label>
            <div className="color-picker-group">
              <div className="color-preview-wrapper">
                <input 
                  type="color" 
                  value={`#${form.subtitle_style.outline_color || '000000'}`} 
                  onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, outline_color: e.target.value.replace('#', '').toUpperCase() } })} 
                  className="color-input"
                />
              </div>
              <div className="hex-input-wrapper">
                <div className="hex-input-group">
                  <span>#</span>
                  <input 
                    type="text" 
                    value={form.subtitle_style.outline_color || '000000'} 
                    onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, outline_color: e.target.value.replace('#', '').toUpperCase() } })} 
                    placeholder="000000"
                    maxLength="6"
                    className="hex-input"
                  />
                </div>
              </div>
            </div>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
            <label style={{ visibility: 'hidden' }}>Placeholder</label>
            <div style={{ flex: 1, display: 'flex', alignItems: 'center' }}>
              <button 
                className="btn-secondary btn-preview" 
                onClick={generatePreview} 
                disabled={previewLoading}
              >
                {previewLoading ? <i className="fas fa-spinner fa-spin"></i> : <i className="fas fa-sync"></i>} 產生預覽
              </button>
            </div>
          </div>
        </div>

        {previewImage && (
          <div className="preview-box" style={{ marginTop: '20px' }}>
            <div className="preview-label">
              <i className="fas fa-eye"></i> 字幕預覽
            </div>
            <div className="preview-image-container">
              <div 
                className="preview-image-wrapper"
                style={{ 
                  aspectRatio: (() => {
                    const [w, h] = form.video.resolution.split('x').map(Number)
                    return `${w}/${h}`
                  })()
                }}
              >
                <img src={previewImage} alt="Subtitle Preview" />
              </div>
            </div>
          </div>
        )}

        <div style={{ marginTop: 24 }}>
          <button 
            onClick={submit} 
            disabled={!isFormValid()}
            className="btn-submit"
          >
            <i className="fas fa-paper-plane" style={{ marginRight: '8px' }}></i> 
            {isFormValid() ? '建立任務' : '請填寫所有必填欄位'}
          </button>
        </div>
      </div>

      <div className="card">
        <div className="task-list-header">
          <h2><i className="fas fa-list-check"></i> 任務列表</h2>
          {jobs.length > 0 && (
            <button className="btn-danger btn-delete-all" onClick={removeAll}>
              <i className="fas fa-trash-alt"></i> 刪除全部
            </button>
          )}
        </div>
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>建立時間</th>
              <th>狀態</th>
              <th>進度</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {jobs.length === 0 ? (
              <tr>
                <td colSpan="5" className="empty-state">
                  <i className="fas fa-inbox"></i>
                  <div className="empty-title">目前沒有任務</div>
                  <div className="empty-desc">建立新任務後將顯示在這裡</div>
                </td>
              </tr>
            ) : (
              jobs.map((j) => (
                <tr key={j.id}>
                  <td className="id-cell">{j.id}</td>
                  <td className="date-cell">
                    <div className="date-row">
                      <i className="fas fa-calendar-alt"></i>
                      {new Date(j.created_at).toLocaleDateString('zh-TW', { year: 'numeric', month: '2-digit', day: '2-digit' })}
                    </div>
                    <div className="time-row">
                      {new Date(j.created_at).toLocaleTimeString('zh-TW', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                    </div>
                  </td>
                  <td>
                    <div className="status-container">
                      <span className={`status-badge status-${j.status}`}>
                        {j.status === 'pending' && <i className="fas fa-clock"></i>}
                        {j.status === 'running' && <i className="fas fa-spinner fa-spin"></i>}
                        {j.status === 'success' && <i className="fas fa-check-circle"></i>}
                        {j.status === 'failed' && <i className="fas fa-times-circle"></i>}
                        {j.status === 'canceled' && <i className="fas fa-ban"></i>}
                        {' '}{j.status}
                      </span>
                      {j.status === 'failed' && j.error_message && (
                        <div className="error-tooltip">
                          {j.error_message}
                        </div>
                      )}
                    </div>
                  </td>
                  <td>
                    <div className="progress-container">
                      <div className="progress-track">
                        <div 
                          className="progress-bar"
                          style={{ 
                            width: `${j.progress}%`, 
                            background: j.status === 'success' ? 'linear-gradient(90deg, var(--color-secondary), #059669)' : 
                                       j.status === 'failed' ? 'linear-gradient(90deg, var(--color-danger), #dc2626)' :
                                       'linear-gradient(90deg, var(--color-primary), var(--color-primary-light))'
                          }}
                        ></div>
                      </div>
                      <span className="progress-text">{j.progress}%</span>
                    </div>
                  </td>
                  <td className="actions-row">
                    <button className="btn-secondary" onClick={() => handleCopyTask(j)} title="複製參數"><i className="fas fa-copy"></i></button>
                    <button className="btn-secondary" onClick={() => handleDuplicateTask(j)} title="再次執行"><i className="fas fa-redo"></i></button>
                    {!finished(j.status) && <button className="btn-secondary" onClick={() => cancel(j.id)}><i className="fas fa-stop"></i> 取消</button>}
                    <button className="btn-danger" onClick={() => remove(j.id)} title="刪除"><i className="fas fa-trash"></i></button>
                    {j.status === 'success' && <a href={`/api/v1/jobs/${j.id}/result`} className="download-link" title="下載"><i className="fas fa-download"></i></a>}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

    </div>
  )
}
