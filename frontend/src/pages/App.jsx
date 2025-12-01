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

  const addToast = (type, message) => {
    const id = Date.now()
    setToasts(prev => [...prev, { id, type, message }])
    setTimeout(() => {
      setToasts(prev => prev.filter(t => t.id !== id))
    }, 3000)
  }

  // Custom Confirm Dialog
  const [confirmModal, setConfirmModal] = useState({ show: false, message: '', onConfirm: null })

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
      <h1 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 600, background: 'linear-gradient(135deg, #fff 0%, #a5b4fc 100%)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}>
        <i className="fas fa-video" style={{ marginRight: '10px', color: '#a5b4fc' }}></i>
        GoShortsGenerator
      </h1>
      <div style={{ marginBottom: 24 }}>
        <a href="/swagger.html" target="_blank" rel="noreferrer" className="api-docs-link">
          <i className="fas fa-file-code"></i> 查看 API 文件
        </a>
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
              style={{ 
                opacity: 0.7, 
                cursor: 'not-allowed', 
                backgroundColor: 'rgba(0, 0, 0, 0.2)',
                color: 'var(--text-muted)',
                border: '1px solid var(--border-primary)'
              }} 
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
            <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginTop: '6px' }}>
              <div style={{ position: 'relative' }}>
                <input 
                  type="color" 
                  value={`#${form.video.background || '000000'}`} 
                  onChange={(e) => setForm({ ...form, video: { ...form.video, background: e.target.value.replace('#', '').toUpperCase() } })} 
                  disabled={form.video.blur_background}
                  style={{ 
                    width: 50, 
                    height: 50, 
                    border: '2px solid var(--border-primary)',
                    borderRadius: 'var(--radius-md)',
                    cursor: form.video.blur_background ? 'not-allowed' : 'pointer',
                    opacity: form.video.blur_background ? 0.5 : 1,
                    transition: 'all var(--transition-base)'
                  }}
                />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ color: 'var(--text-muted)', fontSize: '0.875rem' }}>#</span>
                  <input 
                    type="text" 
                    value={form.video.background || '000000'} 
                    onChange={(e) => setForm({ ...form, video: { ...form.video, background: e.target.value.replace('#', '').toUpperCase() } })} 
                    disabled={form.video.blur_background}
                    placeholder="000000"
                    maxLength="6"
                    style={{ 
                      flex: 1, 
                      textTransform: 'uppercase', 
                      fontFamily: 'monospace', 
                      letterSpacing: '0.05em',
                      opacity: form.video.blur_background ? 0.5 : 1,
                      cursor: form.video.blur_background ? 'not-allowed' : 'text'
                    }}
                  />
                </div>
              </div>
            </div>
            <div style={{ marginTop: '8px' }}>
              <label style={{ display: 'flex', alignItems: 'center', cursor: 'pointer', fontSize: '0.9rem' }}>
                <input 
                  type="checkbox" 
                  checked={form.video.blur_background || false} 
                  onChange={(e) => setForm({ ...form, video: { ...form.video, blur_background: e.target.checked } })} 
                  style={{ marginRight: '8px', width: 'auto' }}
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
            <div style={{ display: 'flex', gap: '12px', alignItems: 'stretch' }}>
              {form.bgm.source === 'preset' ? (
                <select 
                  value={form.bgm.path} 
                  onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, path: e.target.value } })}
                  style={{ flex: 1, marginTop: 0 }}
                >
                  {['random', form.bgm.path, ...bgmList].filter((v, i, arr) => v && arr.indexOf(v) === i).map((name) => (
                    <option key={name} value={name}>{name === 'random' ? '隨機 (Random)' : name}</option>
                  ))}
                </select>
              ) : (
                <input 
                  value={form.bgm.path} 
                  onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, path: e.target.value } })} 
                  style={{ flex: 1, marginTop: 0 }}
                  placeholder={form.bgm.source === 'upload' ? "請上傳檔案..." : "請輸入網址..."}
                />
              )}

              {form.bgm.source === 'preset' && form.bgm.path && (
                <button 
                  className="btn-secondary"
                  onClick={() => toggleBgm(form.bgm.path)}
                  style={{ padding: '0 16px', minWidth: '48px' }}
                  title={bgmPlaying ? "停止試聽" : "試聽音樂"}
                >
                  <i className={bgmPlaying ? "fas fa-stop" : "fas fa-play"}></i>
                </button>
              )}

              {form.bgm.source === 'upload' && (
                <label className="btn btn-secondary" style={{ margin: 0, display: 'flex', alignItems: 'center', whiteSpace: 'nowrap' }}>
                  <i className="fas fa-cloud-upload-alt"></i>
                  <span style={{ marginLeft: '8px' }}>選擇檔案</span>
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
            <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginTop: '6px' }}>
              <div style={{ position: 'relative' }}>
                <input 
                  type="color" 
                  value={`#${form.subtitle_style.color || 'FFFFFF'}`} 
                  onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, color: e.target.value.replace('#', '').toUpperCase() } })} 
                  style={{ 
                    width: 50, 
                    height: 50, 
                    border: '2px solid var(--border-primary)',
                    borderRadius: 'var(--radius-md)',
                    cursor: 'pointer',
                    transition: 'all var(--transition-base)'
                  }}
                />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ color: 'var(--text-muted)', fontSize: '0.875rem' }}>#</span>
                  <input 
                    type="text" 
                    value={form.subtitle_style.color || 'FFFFFF'} 
                    onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, color: e.target.value.replace('#', '').toUpperCase() } })} 
                    placeholder="FFFFFF"
                    maxLength="6"
                    style={{ flex: 1, textTransform: 'uppercase', fontFamily: 'monospace', letterSpacing: '0.05em' }}
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
            <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginTop: '6px' }}>
              <div style={{ position: 'relative' }}>
                <input 
                  type="color" 
                  value={`#${form.subtitle_style.outline_color || '000000'}`} 
                  onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, outline_color: e.target.value.replace('#', '').toUpperCase() } })} 
                  style={{ 
                    width: 50, 
                    height: 50, 
                    border: '2px solid var(--border-primary)',
                    borderRadius: 'var(--radius-md)',
                    cursor: 'pointer',
                    transition: 'all var(--transition-base)'
                  }}
                />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ color: 'var(--text-muted)', fontSize: '0.875rem' }}>#</span>
                  <input 
                    type="text" 
                    value={form.subtitle_style.outline_color || '000000'} 
                    onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, outline_color: e.target.value.replace('#', '').toUpperCase() } })} 
                    placeholder="000000"
                    maxLength="6"
                    style={{ flex: 1, textTransform: 'uppercase', fontFamily: 'monospace', letterSpacing: '0.05em' }}
                  />
                </div>
              </div>
            </div>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', justifyContent: 'flex-end' }}>
            <label>&nbsp;</label>
            <button 
              className="btn-secondary" 
              onClick={generatePreview} 
              disabled={previewLoading}
              style={{ 
                width: '100%', 
                height: '50px',
                fontSize: '0.9rem', 
                borderRadius: 'var(--radius-md)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: '8px'
              }}
            >
              {previewLoading ? <i className="fas fa-spinner fa-spin"></i> : <i className="fas fa-sync"></i>} 產生預覽
            </button>
          </div>
        </div>

        {previewImage && (
          <div className="preview-box" style={{ marginTop: '20px' }}>
            <div className="preview-label">
              <i className="fas fa-eye"></i> 字幕預覽
            </div>
            <div style={{ display: 'flex', justifyContent: 'center', padding: '20px', background: '#0d1117' }}>
              <div 
                style={{ 
                  position: 'relative',
                  width: '100%',
                  maxWidth: '400px',
                  aspectRatio: (() => {
                    const [w, h] = form.video.resolution.split('x').map(Number)
                    return `${w}/${h}`
                  })(),
                  border: '1px dashed var(--border-primary)',
                  borderRadius: '4px',
                  overflow: 'hidden',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  backgroundColor: '#000'
                }}
              >
                <img src={previewImage} alt="Subtitle Preview" style={{ width: '100%', height: '100%', objectFit: 'contain' }} />
              </div>
            </div>
          </div>
        )}

        <div style={{ marginTop: 24 }}>
          <button 
            onClick={submit} 
            disabled={!isFormValid()}
            style={{
              opacity: isFormValid() ? 1 : 0.5,
              cursor: isFormValid() ? 'pointer' : 'not-allowed',
              width: '100%',
              padding: '16px',
              fontSize: '1.1rem',
              fontWeight: 'bold',
              letterSpacing: '1px',
              boxShadow: isFormValid() ? '0 4px 12px rgba(59, 130, 246, 0.4)' : 'none'
            }}
          >
            <i className="fas fa-paper-plane" style={{ marginRight: '8px' }}></i> 
            {isFormValid() ? '建立任務' : '請填寫所有必填欄位'}
          </button>
        </div>
      </div>

      <div className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <h2 style={{ margin: 0 }}><i className="fas fa-list-check"></i> 任務列表</h2>
          {jobs.length > 0 && (
            <button className="btn-danger" onClick={removeAll} style={{ padding: '8px 16px', fontSize: '0.9rem' }}>
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
                <td colSpan="5" style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>
                  <i className="fas fa-inbox" style={{ fontSize: '3rem', marginBottom: '16px', display: 'block', opacity: 0.3 }}></i>
                  <div style={{ fontSize: '1.125rem' }}>目前沒有任務</div>
                  <div style={{ fontSize: '0.875rem', marginTop: '8px' }}>建立新任務後將顯示在這裡</div>
                </td>
              </tr>
            ) : (
              jobs.map((j) => (
                <tr key={j.id}>
                  <td className="id-cell">{j.id}</td>
                  <td style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                    <div style={{ display: 'flex', alignItems: 'center' }}>
                      <i className="fas fa-calendar-alt" style={{ marginRight: '6px', color: 'var(--color-primary)' }}></i>
                      {new Date(j.created_at).toLocaleDateString('zh-TW', { year: 'numeric', month: '2-digit', day: '2-digit' })}
                    </div>
                    <div style={{ marginLeft: '20px', fontSize: '0.75rem', opacity: 0.8 }}>
                      {new Date(j.created_at).toLocaleTimeString('zh-TW', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                    </div>
                  </td>
                  <td>
                    <div style={{ position: 'relative', display: 'inline-block' }} className="status-container">
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
                    <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                      <div style={{ flex: 1, height: '8px', background: 'var(--bg-input)', borderRadius: '4px', overflow: 'hidden' }}>
                        <div style={{ 
                          width: `${j.progress}%`, 
                          height: '100%', 
                          background: j.status === 'success' ? 'linear-gradient(90deg, var(--color-secondary), #059669)' : 
                                     j.status === 'failed' ? 'linear-gradient(90deg, var(--color-danger), #dc2626)' :
                                     'linear-gradient(90deg, var(--color-primary), var(--color-primary-light))',
                          transition: 'width 0.3s ease',
                          borderRadius: '4px'
                        }}></div>
                      </div>
                      <span style={{ minWidth: '45px', fontSize: '0.875rem', fontWeight: 600 }}>{j.progress}%</span>
                    </div>
                  </td>
                  <td className="actions-row">
                    <button className="btn-secondary" onClick={() => handleCopyTask(j)} title="複製參數"><i className="fas fa-copy"></i></button>
                    <button className="btn-secondary" onClick={() => handleDuplicateTask(j)} title="再次執行"><i className="fas fa-redo"></i></button>
                    {!finished(j.status) && <button className="btn-secondary" onClick={() => cancel(j.id)}><i className="fas fa-stop"></i> 取消</button>}
                    <button className="btn-danger" onClick={() => remove(j.id)}><i className="fas fa-trash"></i> 刪除</button>
                    {j.status === 'success' && <a href={`/api/v1/jobs/${j.id}/result`} className="download-link"><i className="fas fa-download"></i> 下載</a>}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
      <style>{`
        @keyframes slideIn {
          from { transform: translateX(100%); opacity: 0; }
          to { transform: translateX(0); opacity: 1; }
        }
        .status-container:hover .error-tooltip {
          visibility: visible;
          opacity: 1;
          transform: translateX(-50%) translateY(0);
        }
        .error-tooltip {
          visibility: hidden;
          opacity: 0;
          position: absolute;
          bottom: 100%;
          left: 50%;
          transform: translateX(-50%) translateY(10px);
          background-color: #ef4444;
          color: white;
          padding: 8px 12px;
          border-radius: 6px;
          font-size: 0.8rem;
          white-space: pre-wrap;
          z-index: 10;
          box-shadow: 0 4px 6px rgba(0,0,0,0.1);
          transition: all 0.2s ease;
          margin-bottom: 8px;
          min-width: 200px;
          max-width: 400px;
          max-height: 200px;
          overflow-y: auto;
          text-align: left;
          pointer-events: auto;
        }
        .error-tooltip::after {
          content: "";
          position: absolute;
          top: 100%;
          left: 50%;
          margin-left: -5px;
          border-width: 5px;
          border-style: solid;
          border-color: #ef4444 transparent transparent transparent;
        }
      `}</style>
    </div>
  )
}
