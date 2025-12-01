import React from 'react'
import MaterialItem from './MaterialItem'

const MaterialList = ({ materials, onUpdate, onAdd, onRemove, onMove }) => {
  return (
    <div className="section-container">
      <h3><i className="fas fa-images"></i> 素材設定</h3>
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
          />
        ))}
      </div>
      <button className="btn btn-primary btn-block" onClick={onAdd}>
        <i className="fas fa-plus"></i> 新增素材
      </button>
    </div>
  )
}

export default MaterialList
