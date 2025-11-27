import React, { useEffect, useState } from 'react'
import api from '../services/api'

const blankMaterial = { type: 'image', source: 'url', path_or_url: '', duration_sec: 3 }
const defaultRequest = {
  script: '這是一段示範腳本。\n第二句會跟字幕同步。',
  materials: [{ ...blankMaterial, path_or_url: 'https://picsum.photos/720/1280', duration_sec: 3 }],
  tts: { provider: 'free', voice: '', locale: 'en-US', speed: 1, pitch: 0 },
  video: { resolution: '1080x1920', fps: 30, speed: 1 },
  bgm: { source: 'preset', path_or_url_or_name: 'default.mp3', volume: 0.2 },
  subtitle_style: { font: 'Noto Sans TC', size: 36, color: 'FFFFFF', y_offset: 40, max_line_width: 24 }
}

export default function App() {
  const [form, setForm] = useState(defaultRequest)
  const [jobs, setJobs] = useState([])
  const [bgmList, setBgmList] = useState([])

  const loadJobs = async () => {
    const res = await api.listJobs()
    setJobs(res.data || [])
  }
  const loadBgm = async () => {
    const res = await api.listBGM()
    setBgmList(res.data || [])
  }

  useEffect(() => {
    loadJobs()
    loadBgm()
    const id = setInterval(loadJobs, 4000)
    return () => clearInterval(id)
  }, [])

  const updateMaterial = (index, key, value) => {
    const newMats = [...form.materials]
    newMats[index] = { ...newMats[index], [key]: value }
    setForm({ ...form, materials: newMats })
  }
  const addMaterial = () => setForm({ ...form, materials: [...form.materials, { ...blankMaterial }] })
  const removeMaterial = (index) => setForm({ ...form, materials: form.materials.filter((_, i) => i !== index) })

  const submit = async () => {
    await api.createJob(form)
    await loadJobs()
  }

  const cancel = async (id) => {
    await api.cancelJob(id)
    await loadJobs()
  }

  const remove = async (id) => {
    await api.deleteJob(id)
    await loadJobs()
  }

  const finished = (status) => ['success', 'failed', 'canceled'].includes(status)

  return (
    <div className="container">
      <h1><i className="fas fa-video"></i> Video Smith</h1>
      <div style={{ marginBottom: 24 }}>
        <a href="/swagger.html" target="_blank" rel="noreferrer" className="api-docs-link">
          <i className="fas fa-file-code"></i> 查看 API 文件
        </a>
      </div>

      <div className="card">
        <h2><i className="fas fa-plus-circle"></i> 建立任務</h2>
        <label><i className="fas fa-scroll"></i> 腳本</label>
        <textarea rows="4" value={form.script} onChange={(e) => setForm({ ...form, script: e.target.value })} />

        <h3><i className="fas fa-images"></i> 素材設定</h3>
        {form.materials.map((m, idx) => (
          <div key={idx} className="material-block">
            <div className="grid">
              <div>
                <label><i className="fas fa-layer-group"></i> 類型</label>
                <select value={m.type} onChange={(e) => updateMaterial(idx, 'type', e.target.value)}>
                  <option value="image">image</option>
                  <option value="video">video</option>
                </select>
              </div>
              <div>
                <label><i className="fas fa-link"></i> 來源</label>
                <select value={m.source} onChange={(e) => updateMaterial(idx, 'source', e.target.value)}>
                  <option value="url">url</option>
                  <option value="upload">upload(路徑)</option>
                </select>
              </div>
              <div>
                <label><i className="fas fa-clock"></i> 秒數</label>
                <input type="number" value={m.duration_sec} onChange={(e) => updateMaterial(idx, 'duration_sec', Number(e.target.value))} />
              </div>
            </div>
            <div className="material-block-path">
              <label><i className="fas fa-globe"></i> 網址或路徑</label>
              <input value={m.path_or_url} onChange={(e) => updateMaterial(idx, 'path_or_url', e.target.value)} />
            </div>
            <div className="material-block-actions">
              {form.materials.length > 1 && <button className="btn-danger" onClick={() => removeMaterial(idx)}><i className="fas fa-trash"></i> 移除</button>}
            </div>
          </div>
        ))}
        <button className="btn-primary" onClick={addMaterial}><i className="fas fa-plus"></i> 新增素材</button>

        <h3><i className="fas fa-microphone"></i> 語音合成 (TTS)</h3>
        <div className="grid">
          <div>
            <label>Provider</label>
            <select value={form.tts.provider} onChange={(e) => setForm({ ...form, tts: { ...form.tts, provider: e.target.value } })}>
              <option value="free">free_espeak</option>
              <option value="google">google</option>
              <option value="azure_v1">azure_v1</option>
              <option value="azure_v2">azure_v2</option>
            </select>
          </div>
          <div>
            <label>Locale</label>
            <input value={form.tts.locale} onChange={(e) => setForm({ ...form, tts: { ...form.tts, locale: e.target.value } })} />
          </div>
          <div>
            <label>Voice Name</label>
            <input value={form.tts.voice} onChange={(e) => setForm({ ...form, tts: { ...form.tts, voice: e.target.value } })} />
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
        </div>

        <h3><i className="fas fa-music"></i> 背景音樂</h3>
        <div className="grid">
          <div>
            <label>來源</label>
            <select value={form.bgm.source} onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, source: e.target.value } })}>
              <option value="preset">preset</option>
              <option value="url">url</option>
              <option value="upload">upload(路徑)</option>
            </select>
          </div>
          <div>
            <label>音量(0~1)</label>
            <input type="number" step="0.05" value={form.bgm.volume} onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, volume: Number(e.target.value) } })} />
          </div>
        </div>
        <label>檔名/網址/路徑</label>
        {form.bgm.source === 'preset' ? (
          <select value={form.bgm.path_or_url_or_name} onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, path_or_url_or_name: e.target.value } })}>
            {[form.bgm.path_or_url_or_name, ...bgmList].filter((v, i, arr) => v && arr.indexOf(v) === i).map((name) => (
              <option key={name} value={name}>{name}</option>
            ))}
          </select>
        ) : (
          <input value={form.bgm.path_or_url_or_name} onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, path_or_url_or_name: e.target.value } })} />
        )}

        <h3><i className="fas fa-closed-captioning"></i> 字幕樣式</h3>
        <div className="grid">
          <div>
            <label>字體</label>
            <input value={form.subtitle_style.font} onChange={(e) => setForm({ ...form, subtitle_style: { ...form.subtitle_style, font: e.target.value } })} />
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
        </div>
        <div className="preview-box">
          <div className="preview-label">
            <i className="fas fa-eye"></i> 字幕預覽
          </div>
          <div
            style={{
              fontFamily: form.subtitle_style.font || 'Noto Sans TC',
              fontSize: `${form.subtitle_style.size || 36}px`,
              color: `#${form.subtitle_style.color || 'FFFFFF'}`,
              textAlign: 'center',
              padding: '20px'
            }}
          >
            {form.script ? form.script.slice(0, 12) : '預覽文字'}
          </div>
        </div>

        <div style={{ marginTop: 24 }}>
          <button onClick={submit}><i className="fas fa-paper-plane"></i> 建立任務</button>
        </div>
      </div>

      <div className="card">
        <h2><i className="fas fa-list-check"></i> 任務列表</h2>
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>狀態</th>
              <th>進度</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {jobs.length === 0 ? (
              <tr>
                <td colSpan="4" style={{ textAlign: 'center', padding: '40px', color: 'var(--text-muted)' }}>
                  <i className="fas fa-inbox" style={{ fontSize: '3rem', marginBottom: '16px', display: 'block', opacity: 0.3 }}></i>
                  <div style={{ fontSize: '1.125rem' }}>目前沒有任務</div>
                  <div style={{ fontSize: '0.875rem', marginTop: '8px' }}>建立新任務後將顯示在這裡</div>
                </td>
              </tr>
            ) : (
              jobs.map((j) => (
                <tr key={j.id}>
                  <td className="id-cell">{j.id}</td>
                  <td>
                    <span className={`status-badge status-${j.status}`}>
                      {j.status === 'pending' && <i className="fas fa-clock"></i>}
                      {j.status === 'running' && <i className="fas fa-spinner fa-spin"></i>}
                      {j.status === 'success' && <i className="fas fa-check-circle"></i>}
                      {j.status === 'failed' && <i className="fas fa-times-circle"></i>}
                      {j.status === 'canceled' && <i className="fas fa-ban"></i>}
                      {' '}{j.status}
                    </span>
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
    </div>
  )
}
