import React from 'react'

const ConfirmDialog = ({ show, message, onConfirm, onCancel }) => {
  if (!show) return null
  return (
    <div style={{
      position: 'fixed',
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      background: 'rgba(0, 0, 0, 0.7)',
      backdropFilter: 'blur(4px)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      zIndex: 10000,
      animation: 'fadeIn 0.2s ease-out'
    }}>
      <div style={{
        background: 'var(--bg-card)',
        border: '1px solid var(--border-primary)',
        borderRadius: '12px',
        padding: '24px',
        width: '90%',
        maxWidth: '400px',
        boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)',
        animation: 'scaleIn 0.2s ease-out'
      }}>
        <h3 style={{ marginTop: 0, marginBottom: '12px', display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--color-danger)' }}>
          <i className="fas fa-exclamation-triangle"></i> 確認刪除
        </h3>
        <p style={{ color: 'var(--text-secondary)', marginBottom: '24px', lineHeight: 1.5 }}>
          {message}
        </p>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
          <button 
            className="btn-secondary" 
            onClick={onCancel}
            style={{ padding: '8px 16px' }}
          >
            取消
          </button>
          <button 
            className="btn-danger" 
            onClick={onConfirm}
            style={{ padding: '8px 16px' }}
          >
            確認刪除
          </button>
        </div>
      </div>
    </div>
  )
}

export default ConfirmDialog
