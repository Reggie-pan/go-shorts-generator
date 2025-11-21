import React, { useEffect, useState } from 'react'
import api from '../services/api'

const blankMaterial = { type: 'image', source: 'url', path_or_url: '', duration_sec: 3 }
const defaultRequest = {
  script: '這是一段示範腳本。\n第二句會跟字幕同步。',
  materials: [{ ...blankMaterial, path_or_url: 'https://picsum.photos/720/1280', duration_sec: 3 }],
  tts: { provider: 'free', voice: '', locale: 'en-US', speed: 1, pitch: 0 },
  video: { resolution: '1080x1920', fps: 30, speed: 1 },
  bgm: { source: 'preset', path_or_url_or_name: 'default.mp3', volume: 0.2 },
  subtitle_style: { font: 'NotoSansTC', size: 36, color: 'FFFFFF', y_offset: 40, max_line_width: 24 }
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
      <h1>Video Smith</h1>
      <div style={{ marginBottom: 12 }}>
        <a href="/api/v1/swagger.json" style={{ color: '#93c5fd' }} target="_blank" rel="noreferrer">查看 Swagger JSON</a>
      </div>
      <div className="card">
        <h2>建立任務</h2>
        <label>腳本</label>
        <textarea rows="4" value={form.script} onChange={(e) => setForm({ ...form, script: e.target.value })} />

        <h3>素材設定</h3>
        {form.materials.map((m, idx) => (
          <div key={idx} style={{ border: '1px solid #1f2937', padding: 8, borderRadius: 8, marginBottom: 8 }}>
            <div style={{ display: 'flex', gap: 8 }}>
              <div style={{ flex: 1 }}>
                <label>類型</label>
                <select value={m.type} onChange={(e) => updateMaterial(idx, 'type', e.target.value)}>
                  <option value="image">image</option>
                  <option value="video">video</option>
                </select>
              </div>
              <div style={{ flex: 1 }}>
                <label>來源</label>
                <select value={m.source} onChange={(e) => updateMaterial(idx, 'source', e.target.value)}>
                  <option value="url">url</option>
                  <option value="upload">upload(路徑)</option>
                </select>
              </div>
              <div style={{ flex: 1 }}>
                <label>秒數</label>
                <input type="number" value={m.duration_sec} onChange={(e) => updateMaterial(idx, 'duration_sec', Number(e.target.value))} />
              </div>
            </div>
            <label>網址或路徑</label>
            <input value={m.path_or_url} onChange={(e) => updateMaterial(idx, 'path_or_url', e.target.value)} />
            <div style={{ textAlign: 'right' }}>
              {form.materials.length > 1 && <button style={{ background: '#ef4444', color: '#fff' }} onClick={() => removeMaterial(idx)}>移除</button>}
            </div>
          </div>
        ))}
        <button onClick={addMaterial} style={{ background: '#0ea5e9', color: '#083344' }}>新增素材</button>

        <h3>TTS</h3>
        <div style={{ display: 'flex', gap: 8 }}>
          <div style={{ flex: 1 }}>
            <label>Provider</label>
            <select value={form.tts.provider} onChange={(e) => setForm({ ...form, tts: { ...form.tts, provider: e.target.value } })}>
              <option value="free">free_espeak</option>
              <option value="google">google</option>
              <option value="azure_v1">azure_v1</option>
              <option value="azure_v2">azure_v2</option>
            </select>
          </div>
          <div style={{ flex: 1 }}>
            <label>Locale</label>
            <input value={form.tts.locale} onChange={(e) => setForm({ ...form, tts: { ...form.tts, locale: e.target.value } })} />
          </div>
          <div style={{ flex: 1 }}>
            <label>Voice Name</label>
            <input value={form.tts.voice} onChange={(e) => setForm({ ...form, tts: { ...form.tts, voice: e.target.value } })} />
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <div style={{ flex: 1 }}>
            <label>語速</label>
            <input type="number" step="0.1" value={form.tts.speed} onChange={(e) => setForm({ ...form, tts: { ...form.tts, speed: Number(e.target.value) } })} />
          </div>
          <div style={{ flex: 1 }}>
            <label>音高</label>
            <input type="number" step="0.1" value={form.tts.pitch} onChange={(e) => setForm({ ...form, tts: { ...form.tts, pitch: Number(e.target.value) } })} />
          </div>
        </div>

        <h3>影片</h3>
        <div style={{ display: 'flex', gap: 8 }}>
          <div style={{ flex: 1 }}>
            <label>解析度</label>
            <input value={form.video.resolution} onChange={(e) => setForm({ ...form, video: { ...form.video, resolution: e.target.value } })} />
          </div>
          <div style={{ flex: 1 }}>
            <label>FPS</label>
            <input type="number" value={form.video.fps} onChange={(e) => setForm({ ...form, video: { ...form.video, fps: Number(e.target.value) } })} />
          </div>
          <div style={{ flex: 1 }}>
            <label>速度倍率</label>
            <input type="number" step="0.1" value={form.video.speed} onChange={(e) => setForm({ ...form, video: { ...form.video, speed: Number(e.target.value) } })} />
          </div>
        </div>

        <h3>背景音樂</h3>
        <div style={{ display: 'flex', gap: 8 }}>
          <div style={{ flex: 1 }}>
            <label>來源</label>
            <select value={form.bgm.source} onChange={(e) => setForm({ ...form, bgm: { ...form.bgm, source: e.target.value } })}>
              <option value="preset">preset</option>
              <option value="url">url</option>
              <option value="upload">upload(路徑)</option>
            </select>
          </div>
          <div style={{ flex: 1 }}>
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

        <div style={{ marginTop: 12 }}>
          <button onClick={submit}>送出</button>
        </div>
      </div>

      <div className="card">
        <h2>任務列表</h2>
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
            {jobs.map((j) => (
              <tr key={j.id}>
                <td>{j.id}</td>
                <td>{j.status}</td>
                <td>{j.progress}%</td>
                <td>
                  {!finished(j.status) && <button onClick={() => cancel(j.id)}>取消</button>}
                  <button onClick={() => remove(j.id)} style={{ marginLeft: 8, background: '#ef4444', color: '#fff' }}>刪除</button>
                  {j.status === 'success' && <a href={`/api/v1/jobs/${j.id}/result`} style={{ marginLeft: 8, color: '#93c5fd' }}>下載</a>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
