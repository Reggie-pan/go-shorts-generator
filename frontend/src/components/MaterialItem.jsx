import React from 'react'
import api from '../services/api'

const MaterialItem = ({ 
  material, 
  index, 
  total, 
  onUpdate, 
  onRemove, 
  onMove 
}) => {
  const handleFileUpload = async (e) => {
    if (e.target.files[0]) {
      try {
        const file = e.target.files[0]
        const ext = file.name.split('.').pop().toLowerCase()
        
        // Auto-detect type
        if (['mp4', 'mov', 'avi', 'mkv', 'webm'].includes(ext)) {
          onUpdate(index, 'type', 'video')
        } else if (['jpg', 'jpeg', 'png', 'gif', 'webp'].includes(ext)) {
          onUpdate(index, 'type', 'image')
        }
        
        const res = await api.uploadFile(file)
        onUpdate(index, 'path', res.path)
      } catch (err) {
        alert('上傳失敗')
      }
    }
  }

  return (
    <div className="material-block">
      <div className="material-header">
        <span className="material-index">#{index + 1}</span>
        <div className="material-controls">
          <button 
            className="btn-icon" 
            onClick={() => onMove(index, -1)} 
            disabled={index === 0}
            title="上移"
          >
            <i className="fas fa-arrow-up"></i>
          </button>
          <button 
            className="btn-icon" 
            onClick={() => onMove(index, 1)} 
            disabled={index === total - 1}
            title="下移"
          >
            <i className="fas fa-arrow-down"></i>
          </button>
          <button 
            className="btn-icon btn-icon-danger" 
            onClick={() => onRemove(index)}
            title="移除"
          >
            <i className="fas fa-trash"></i>
          </button>
        </div>
      </div>

      <div className="grid">
        <div className="form-group">
          <label><i className="fas fa-layer-group"></i> 類型</label>
          <select 
            value={material.type} 
            onChange={(e) => onUpdate(index, 'type', e.target.value)}
          >
            <option value="image">image</option>
            <option value="video">video</option>
          </select>
        </div>
        <div className="form-group">
          <label><i className="fas fa-link"></i> 來源</label>
          <select 
            value={material.source} 
            onChange={(e) => onUpdate(index, 'source', e.target.value)}
          >
            <option value="url">url</option>
            <option value="upload">upload(檔案)</option>
          </select>
        </div>
        <div className="form-group">
          <label><i className="fas fa-clock"></i> 秒數</label>
          <input 
            type="number" 
            value={material.duration_sec} 
            onChange={(e) => onUpdate(index, 'duration_sec', Number(e.target.value))} 
          />
        </div>
      </div>

      <div className="material-path-row">
        <div className="form-group flex-grow">
          <label><i className="fas fa-globe"></i> 網址或路徑</label>
          <div className="input-group">
            <input 
              value={material.path} 
              onChange={(e) => onUpdate(index, 'path', e.target.value)} 
              onBlur={(e) => onUpdate(index, 'path', e.target.value.trim())}
              placeholder="請輸入網址或上傳檔案..."
            />
            
            {material.source === 'upload' && (
              <label className="btn btn-secondary btn-upload">
                <i className="fas fa-cloud-upload-alt"></i>
                <span>選擇檔案</span>
                <input 
                  type="file" 
                  className="hidden-input"
                  onChange={handleFileUpload} 
                />
              </label>
            )}

            {material.type === 'video' && (
              <button 
                className={`btn-toggle ${material.mute ? 'active' : ''}`}
                onClick={() => onUpdate(index, 'mute', !material.mute)}
                title={material.mute ? "點擊開啟聲音" : "點擊靜音"}
              >
                <i className={material.mute ? "fas fa-volume-mute" : "fas fa-volume-up"}></i>
                <span>{material.mute ? '已靜音' : '有聲音'}</span>
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export default MaterialItem
