import React, { useEffect, useState } from 'react'
import api from '../services/api'
import MaterialList from '../components/MaterialList'
import SearchableSelect from '../components/SearchableSelect'
import ToastContainer from '../components/ToastContainer'
import ConfirmDialog from '../components/ConfirmDialog'
import { translations } from '../utils/i18n'

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
  // Theme & Language & Pagination
  const [theme, setTheme] = useState(localStorage.getItem('theme') || 'dark')
  const [lang, setLang] = useState(localStorage.getItem('lang') || 'zh-TW')
  const [currentPage, setCurrentPage] = useState(1)
  const itemsPerPage = 10

  // Helper for translation
  const t = (key) => {
    return translations[lang][key] || key
  }

  const [form, setForm] = useState({
    ...defaultRequest,
    script: translations[localStorage.getItem('lang') || 'zh-TW']?.defaultScript || defaultRequest.script
  })

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

  // Apply theme
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('theme', theme)
  }, [theme])

  // Persist lang and update script if it's default
  useEffect(() => {
    localStorage.setItem('lang', lang)
    
    // Check if current script is one of the default scripts or empty
    const defaultScripts = Object.values(translations).map(t => t.defaultScript)
    if (!form.script || defaultScripts.includes(form.script)) {
      setForm(prev => ({
        ...prev,
        script: translations[lang].defaultScript
      }))
    }
  }, [lang])

  // Pagination logic
  const totalPages = Math.ceil(jobs.length / itemsPerPage)
  const currentJobs = jobs.slice((currentPage - 1) * itemsPerPage, currentPage * itemsPerPage)

  const handlePageChange = (page) => {
    if (page >= 1 && page <= totalPages) {
      setCurrentPage(page)
    }
  }

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
      let previewText = form.script ? form.script.split('\n')[0] : t('subtitlePreview') + ' Preview'
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
      addToast('error', t('toastPreviewFail'))
    } finally {
      setPreviewLoading(false)
    }
  }

  const loadJobs = async () => {
    const res = await api.listJobs()
    // 排序由後端決定
    setJobs(res.data || [])
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
      addToast('success', t('toastCreateSuccess'))
      setForm({ ...form }) // Don't reset script
      loadJobs()
    } catch (err) {
      addToast('error', t('toastCreateFail') + (err.response?.data?.error || err.message))
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
      message: t('confirmDeleteAll'),
      onConfirm: async () => {
        try {
          await api.deleteAllJobs()
          addToast('success', t('toastDeleteAllSuccess'))
          await loadJobs()
        } catch (err) {
          addToast('error', t('toastDeleteFail'))
        }
      }
    })
  }

  const finished = (status) => ['success', 'failed', 'canceled'].includes(status)

  const formatTime = (dateString) => {
    if (!dateString) return '-'
    try {
      const date = new Date(dateString)
      return date.toLocaleString(lang, {
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
    addToast('success', t('toastCopySuccess'))
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const handleDuplicateTask = async (task) => {
    try {
      await api.createJob(task.request)
      addToast('success', t('toastExecSuccess'))
      loadJobs()
    } catch (err) {
      addToast('error', t('toastExecFail') + (err.response?.data?.error || err.message))
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
              <h3><i className="fas fa-info-circle" style={{ marginRight: '8px' }}></i> {t('aboutTitle')}</h3>
              <button className="close-btn" onClick={() => setShowAboutModal(false)}>&times;</button>
            </div>
            <div className="modal-body">
              <div style={{ marginBottom: '16px' }}>
                <h4 style={{ margin: '0 0 8px 0', color: 'var(--text-primary)' }}>{t('reportBug')}</h4>
                <a href="https://github.com/reggie-pan/go-shorts-generator/issues" target="_blank" rel="noreferrer" className="about-link">
                  <i className="fab fa-github"></i> GitHub Issues
                </a>
              </div>
              <hr style={{ borderColor: 'var(--border-primary)', margin: '16px 0' }} />
              <div>
                <h4 style={{ margin: '0 0 8px 0', color: 'var(--text-primary)' }}>{t('appTitle')}</h4>
                <p style={{ color: 'var(--text-secondary)' }}>
                  {t('aboutDesc')}
                </p>
                <a href="https://github.com/reggie-pan/go-shorts-generator" target="_blank" rel="noreferrer" className="about-link">
                  <i className="fab fa-github"></i> {t('projectRepo')}
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
          <div className="header-group">
            <button 
              onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')} 
              className="btn-icon btn-theme-toggle"
              title={theme === 'dark' ? 'Switch to Light Mode' : 'Switch to Dark Mode'}
            >
              <i className={`fas ${theme === 'dark' ? 'fa-sun' : 'fa-moon'}`}></i>
            </button>
            <div className="lang-select-wrapper">
              <i className="fas fa-globe"></i>
              <SearchableSelect 
                options={[
                  { label: '繁體中文', value: 'zh-TW' },
                  { label: '简体中文', value: 'zh-CN' },
                  { label: 'English', value: 'en' }
                ]}
                value={lang}
                onChange={(val) => setLang(val)}
                searchable={false}
                className="header-lang-select"
              />
            </div>
          </div>
          <div className="header-divider"></div>
          <div className="header-group">
            <a href="/swagger.html" target="_blank" rel="noreferrer" className="btn-text">
              <i className="fas fa-file-code"></i> <span>{t('apiDocs')}</span>
            </a>
            <button 
              onClick={() => setShowAboutModal(true)} 
              className="btn-text" 
            >
              <i className="fas fa-info-circle"></i> <span>{t('about')}</span>
            </button>
          </div>
        </div>
      </div>

      <div className="card">
        <h2><i className="fas fa-plus-circle"></i> {t('createTask')}</h2>
        <label><i className="fas fa-scroll"></i> {t('script')}</label>
        <textarea rows="4" value={form.script} onChange={(e) => setForm({ ...form, script: e.target.value })} />

        <MaterialList 
          materials={form.materials}
          onUpdate={updateMaterial}
          onAdd={addMaterial}
          onRemove={removeMaterial}
          onMove={moveMaterial}
          t={t}
        />

        <h3><i className="fas fa-microphone"></i> {t('tts')}</h3>
        <div className="grid">
          <div>
            <label>{t('provider')}</label>
            <SearchableSelect 
              options={[
                { label: 'azure_v1', value: 'azure_v1' },
                { label: 'azure_v2', value: 'azure_v2' }
              ]}
              value={form.tts.provider}
              onChange={(val) => setForm({ ...form, tts: { ...form.tts, provider: val } })}
              searchable={false}
            />
          </div>
          <div>
            <label>{t('locale')}</label>
            <input 
              value={form.tts.locale} 
              disabled
              className="input-disabled"
            />
          </div>
          <div>
            <label>{t('voiceName')}</label>
            {voiceList.length > 0 ? (
              <SearchableSelect 
                options={voiceList.map(v => ({ label: v.name, value: v.name }))}
                value={form.tts.voice}
                placeholder={t('voicePlaceholder')}
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
            <label>{t('speed')}</label>
            <input type="number" step="0.1" value={form.tts.speed} onChange={(e) => setForm({ ...form, tts: { ...form.tts, speed: Number(e.target.value) } })} />
          </div>
          <div>
            <label>{t('pitch')}</label>
            <input type="number" step="0.1" value={form.tts.pitch} onChange={(e) => setForm({ ...form, tts: { ...form.tts, pitch: Number(e.target.value) } })} />
          </div>
        </div>

        <h3><i className="fas fa-film"></i> {t('videoSettings')}</h3>
        <div className="grid">
          <div>
            <label>{t('resolution')}</label>
            <SearchableSelect 
              options={[
                { label: '1080x1920 (9:16)', value: '1080x1920' },
                { label: '720x1280 (9:16)', value: '720x1280' },
                { label: '1080x1080 (1:1)', value: '1080x1080' },
                { label: '1920x1080 (16:9)', value: '1920x1080' }
              ]}
              value={form.video.resolution}
              onChange={(val) => setForm({ ...form, video: { ...form.video, resolution: val } })}
              searchable={false}
            />
          </div>
          <div>
            <label>{t('fps')}</label>
            <input type="number" value={form.video.fps} onChange={(e) => setForm({ ...form, video: { ...form.video, fps: Number(e.target.value) } })} />
          </div>
          <div>
            <label>{t('transition')}</label>
            <SearchableSelect 
              options={[
                { label: t('transitionNone'), value: 'none' },
                { label: t('transitionFade'), value: 'fade' },
                { label: t('transitionWipeLeft'), value: 'wipeleft' },
                { label: t('transitionWipeRight'), value: 'wiperight' },
                { label: t('transitionSlideLeft'), value: 'slideleft' },
                { label: t('transitionSlideRight'), value: 'slideright' },
                { label: t('transitionCircleOpen'), value: 'circleopen' },
                { label: t('transitionCircleClose'), value: 'circleclose' }
              ]}
              value={form.video.transition || 'none'}
              onChange={(val) => setForm({ ...form, video: { ...form.video, transition: val } })}
              searchable={false}
            />
          </div>
          <div>
            <label><i className="fas fa-palette"></i> {t('bgColor')}</label>
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
              <div className="blur-edge-wrapper">
                <label className="checkbox-label">
                  <input 
                    type="checkbox" 
                    checked={form.video.blur_background || false} 
                    onChange={(e) => setForm({ ...form, video: { ...form.video, blur_background: e.target.checked } })} 
                  />
                  {t('blurEdge')}
                </label>
              </div>
            </div>
          </div>
        </div>

        <h3><i className="fas fa-music"></i> {t('bgm')}</h3>
        <div className="grid">
          <div>
            <label>{t('source')}</label>
            <SearchableSelect 
              options={[
                { label: 'preset', value: 'preset' },
                { label: 'url', value: 'url' },
                { label: t('uploadFile'), value: 'upload' },
                { label: t('noneMusic'), value: 'none' }
              ]}
              value={form.bgm.source}
              onChange={(val) => setForm({ ...form, bgm: { ...form.bgm, source: val } })}
              searchable={false}
            />
          </div>
          {form.bgm.source !== 'none' && (
            <div>
              <label>{t('volume')}</label>
              <input type="number" step="0.05" value={form.bgm.volume} onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, volume: Number(e.target.value) } })} />
            </div>
          )}
        </div>
        {form.bgm.source !== 'none' && (
            <div style={{ marginTop: '12px' }}>
              <label>{t('pathPlaceholder')}</label>
              <div className="bgm-input-group">
                {form.bgm.source === 'preset' ? (
                  <SearchableSelect 
                    options={['random', form.bgm.path, ...bgmList]
                      .filter((v, i, arr) => v && arr.indexOf(v) === i)
                      .map((name) => ({ label: name === 'random' ? t('random') : name, value: name }))
                    }
                    value={form.bgm.path}
                    onChange={(val) => setForm({ ...form, bgm: { ...form.bgm, path: val } })}
                    searchable={true}
                  />
                ) : (
                  <input 
                    value={form.bgm.path} 
                    onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, path: e.target.value } })} 
                    placeholder={form.bgm.source === 'upload' ? t('uploadPlaceholder') : t('urlPlaceholder')}
                  />
                )}

                {form.bgm.source === 'preset' && form.bgm.path && (
                  <button 
                    className="btn-secondary btn-play"
                    onClick={() => toggleBgm(form.bgm.path)}
                    title={bgmPlaying ? t('stopPreview') : t('playPreview')}
                  >
                    <i className={bgmPlaying ? "fas fa-stop" : "fas fa-play"}></i>
                  </button>
                )}

                {form.bgm.source === 'upload' && (
                  <label className="btn btn-secondary btn-file-select">
                    <i className="fas fa-cloud-upload-alt"></i>
                    <span>{t('selectFile')}</span>
                    <input 
                      type="file" 
                      style={{ display: 'none' }} 
                      onChange={async (e) => {
                        if (e.target.files[0]) {
                          try {
                            const res = await api.uploadFile(e.target.files[0])
                            setForm({ ...form, bgm: { ...form.bgm, path: res.path } })
                          } catch (err) {
                            alert(t('uploadFail'))
                          }
                        }
                      }} 
                    />
                  </label>
                )}
              </div>
            </div>
        )}

        <h3><i className="fas fa-closed-captioning"></i> {t('subtitleStyle')}</h3>
        <div className="grid">
          <div>
            <label>{t('font')}</label>
            <SearchableSelect 
              options={fontList.length > 0 
                ? fontList.map((font) => ({ label: font.name, value: font.name }))
                : [{ label: form.subtitle_style.font, value: form.subtitle_style.font }]
              }
              value={form.subtitle_style.font}
              onChange={(val) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, font: val } })}
              searchable={true}
            />
          </div>
          <div>
            <label>{t('fontSize')}</label>
            <input type="number" value={form.subtitle_style.size} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, size: Number(e.target.value) } })} />
          </div>
          <div>
            <label>{t('yOffset')}</label>
            <input type="number" value={form.subtitle_style.y_offset} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, y_offset: Number(e.target.value) } })} />
          </div>
          <div>
            <label><i className="fas fa-palette"></i> {t('color')}</label>
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
            <label>{t('maxLineWidth')}</label>
            <input type="number" value={form.subtitle_style.max_line_width} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, max_line_width: Number(e.target.value) } })} />
          </div>
          <div>
            <label>{t('outlineWidth')}</label>
            <input type="number" step="0.1" value={form.subtitle_style.outline_width} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, outline_width: Number(e.target.value) } })} />
          </div>
          <div>
            <label><i className="fas fa-palette"></i> {t('outlineColor')}</label>
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
                {previewLoading ? <i className="fas fa-spinner fa-spin"></i> : <i className="fas fa-sync"></i>} {t('preview')}
              </button>
            </div>
          </div>
        </div>

        {previewImage && (
          <div className="preview-box" style={{ marginTop: '20px' }}>
            <div className="preview-label">
              <i className="fas fa-eye"></i> {t('subtitlePreview')}
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
            {isFormValid() ? t('submit') : t('fillRequired')}
          </button>
        </div>
      </div>

      <div className="card">
        <div className="task-list-header">
          <h2><i className="fas fa-list-check"></i> {t('taskList')}</h2>
          {jobs.length > 0 && (
            <button className="btn-danger btn-delete-all" onClick={removeAll}>
              <i className="fas fa-trash-alt"></i> {t('deleteAll')}
            </button>
          )}
        </div>
        <table>
          <thead>
            <tr>
              <th>{t('id')}</th>
              <th>{t('createdTime')}</th>
              <th>{t('status')}</th>
              <th>{t('progress')}</th>
              <th>{t('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {jobs.length === 0 ? (
              <tr>
                <td colSpan="5" className="empty-state">
                  <i className="fas fa-inbox"></i>
                  <div className="empty-title">{t('emptyStateTitle')}</div>
                  <div className="empty-desc">{t('emptyStateDesc')}</div>
                </td>
              </tr>
            ) : (
              currentJobs.map((j) => (
                <tr key={j.id}>
                  <td className="id-cell">
                    <div className="id-wrapper">
                      <span className="id-text" title={j.id}>#{j.id.substring(0, 8)}</span>
                      <button 
                        className="btn-icon-sm" 
                        onClick={() => {
                          navigator.clipboard.writeText(j.id)
                          addToast('success', t('toastCopyIdSuccess'))
                        }}
                        title={t('id')}
                      >
                        <i className="fas fa-copy"></i>
                      </button>
                    </div>
                  </td>
                  <td className="date-cell">
                    <div className="date-row">
                      <i className="fas fa-calendar-alt"></i>
                      {new Date(j.created_at).toLocaleDateString(lang, { year: 'numeric', month: '2-digit', day: '2-digit' })}
                    </div>
                    <div className="time-row">
                      {new Date(j.created_at).toLocaleTimeString(lang, { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
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
                    <button className="btn-secondary" onClick={() => handleCopyTask(j)} title={t('copyParams')}><i className="fas fa-copy"></i></button>
                    <button className="btn-secondary" onClick={() => handleDuplicateTask(j)} title={t('duplicateTask')}><i className="fas fa-redo"></i></button>
                    {!finished(j.status) && <button className="btn-secondary" onClick={() => cancel(j.id)} title={t('cancel')}><i className="fas fa-stop"></i></button>}
                    <button className="btn-danger" onClick={() => remove(j.id)} title={t('delete')}><i className="fas fa-trash"></i></button>
                    {j.status === 'success' && <a href={`/api/v1/jobs/${j.id}/result`} className="download-link" title={t('download')}><i className="fas fa-download"></i></a>}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
        
        {/* Pagination Controls */}
        {jobs.length > itemsPerPage && (
          <div className="pagination-controls" style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', marginTop: '16px', gap: '16px' }}>
            <button 
              className="btn-secondary" 
              onClick={() => handlePageChange(currentPage - 1)}
              disabled={currentPage === 1}
            >
              <i className="fas fa-chevron-left"></i> {t('prevPage')}
            </button>
            <span style={{ color: 'var(--text-secondary)' }}>
              {t('page')} {currentPage} / {totalPages} ({t('total')} {jobs.length} {t('items')})
            </span>
            <button 
              className="btn-secondary" 
              onClick={() => handlePageChange(currentPage + 1)}
              disabled={currentPage === totalPages}
            >
              {t('nextPage')} <i className="fas fa-chevron-right"></i>
            </button>
          </div>
        )}
      </div>

    </div>
  )
}
