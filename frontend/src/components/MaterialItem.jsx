import React from 'react'
import api from '../services/api'
import SearchableSelect from './SearchableSelect'

const MaterialItem = ({ 
  material, 
  index, 
  total, 
  onUpdate, 
  onRemove, 
  onMove,
  t
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
        alert(t('uploadFail'))
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
            title={t('moveUp')}
          >
            <i className="fas fa-arrow-up"></i>
          </button>
          <button 
            className="btn-icon" 
            onClick={() => onMove(index, 1)} 
            disabled={index === total - 1}
            title={t('moveDown')}
          >
            <i className="fas fa-arrow-down"></i>
          </button>
          <button 
            className="btn-icon btn-icon-danger" 
            onClick={() => onRemove(index)}
            title={t('remove')}
          >
            <i className="fas fa-trash"></i>
          </button>
        </div>
      </div>

      <div className="grid">
        <div className="form-group">
          <label><i className="fas fa-layer-group"></i> {t('type')}</label>
          <SearchableSelect 
            options={[
              { label: 'image', value: 'image' },
              { label: 'video', value: 'video' }
            ]}
            value={material.type}
            onChange={(val) => onUpdate(index, 'type', val)}
            searchable={false}
          />
        </div>
        {material.type === 'image' && (
          <div className="form-group">
            <label><i className="fas fa-magic"></i> {t('effect')}</label>
            <SearchableSelect 
              options={[
                { label: t('effectNone'), value: 'none' },
                { label: t('effectZoomIn'), value: 'zoom_in' },
                { label: t('effectZoomOut'), value: 'zoom_out' },
                { label: t('effectPanLeft'), value: 'pan_left' },
                { label: t('effectPanRight'), value: 'pan_right' },
                { label: t('effectPanUp'), value: 'pan_up' },
                { label: t('effectPanDown'), value: 'pan_down' },
                { label: t('effectDiagonalPan'), value: 'diagonal_pan' },
                { label: t('effectRotate'), value: 'rotate' },
                { label: t('effectShake'), value: 'shake' },
              ]}
              value={material.effect || 'none'}
              onChange={(val) => onUpdate(index, 'effect', val)}
              searchable={false}
            />
          </div>
        )}
        <div className="form-group">
          <label><i className="fas fa-link"></i> {t('source')}</label>
          <SearchableSelect 
            options={[
              { label: 'url', value: 'url' },
              { label: t('uploadFile'), value: 'upload' }
            ]}
            value={material.source}
            onChange={(val) => onUpdate(index, 'source', val)}
            searchable={false}
          />
        </div>
        <div className="form-group">
          <label><i className="fas fa-clock"></i> {t('seconds')}</label>
          <input 
            type="number" 
            value={material.duration_sec} 
            onChange={(e) => onUpdate(index, 'duration_sec', Number(e.target.value))} 
          />
        </div>
      </div>

      <div className="material-path-row">
        <div className="form-group flex-grow">
          <label><i className="fas fa-globe"></i> {t('pathOrUrl')}</label>
          <div className="input-group">
            <input 
              value={material.path} 
              onChange={(e) => onUpdate(index, 'path', e.target.value)} 
              onBlur={(e) => onUpdate(index, 'path', e.target.value.trim())}
              placeholder={t('pathInputPlaceholder')}
            />
            
            {material.source === 'upload' && (
              <label className="btn btn-secondary btn-upload">
                <i className="fas fa-cloud-upload-alt"></i>
                <span>{t('uploadFileBtn')}</span>
                <input 
                  type="file" 
                  className="hidden-input"
                  onChange={handleFileUpload} 
                />
              </label>
            )}
          </div>
        </div>
      </div>

      {/* 影片音訊控制 - 獨立行 */}
      {material.type === 'video' && (
        <div className="video-audio-controls">
          <button 
            className={`btn-toggle ${material.mute ? 'active' : ''}`}
            onClick={() => onUpdate(index, 'mute', !material.mute)}
            title={material.mute ? t('unmute') : t('mute')}
          >
            <i className={material.mute ? "fas fa-volume-mute" : "fas fa-volume-up"}></i>
            <span>{material.mute ? t('muted') : t('unmuted')}</span>
          </button>

          {!material.mute && (
            <div className="volume-slider-group">
              <label><i className="fas fa-volume-up"></i> {t('materialVolume')}</label>
              <div className="volume-slider-container">
                <input 
                  type="range" 
                  min="0" 
                  max="1"
                  step="0.1"
                  value={material.volume ?? 1} 
                  onChange={(e) => onUpdate(index, 'volume', Number(e.target.value))} 
                  className="volume-slider"
                />
                <span className="volume-value">{(material.volume ?? 1).toFixed(1)}</span>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export default MaterialItem
