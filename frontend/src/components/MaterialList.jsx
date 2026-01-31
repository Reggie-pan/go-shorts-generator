import React from 'react'
import MaterialItem from './MaterialItem'

const MaterialList = ({ materials, onUpdate, onAdd, onRemove, onMove, autoDistribute, onAutoDistributeChange, t }) => {
  return (
    <div className="section-container">
      <h3><i className="fas fa-images"></i> {t('materialSettings')}</h3>
      
      {/* 自動分配時長開關 */}
      <div className="auto-distribute-toggle" style={{ marginBottom: '16px' }}>
        <label className="checkbox-label">
          <input 
            type="checkbox" 
            checked={autoDistribute || false} 
            onChange={(e) => onAutoDistributeChange(e.target.checked)} 
          />
          {t('autoDistributeDuration')}
        </label>
        {autoDistribute && (
          <div className="hint-text" style={{ marginTop: '4px', fontSize: '12px', color: 'var(--text-secondary)' }}>
            <i className="fas fa-info-circle"></i> {t('autoDistributeDurationHint')}
          </div>
        )}
      </div>

      <div className="material-list">
        {materials.map((m, idx) => (
          <MaterialItem 
            key={idx} 
            material={m} 
            index={idx} 
            total={materials.length}
            onUpdate={onUpdate} 
            onRemove={onRemove}
            onMove={onMove}
            autoDistribute={autoDistribute}
            t={t}
          />
        ))}
      </div>
      <button className="btn btn-primary btn-block" onClick={onAdd}>
        <i className="fas fa-plus"></i> {t('addMaterial')}
      </button>
    </div>
  )
}

export default MaterialList
