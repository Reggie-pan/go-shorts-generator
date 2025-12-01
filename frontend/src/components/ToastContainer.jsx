import React, { useEffect, useState } from 'react'

const ToastContainer = ({ toasts }) => {
  return (
    <div style={{
      position: 'fixed',
      bottom: '24px',
      right: '24px',
      display: 'flex',
      flexDirection: 'column',
      gap: '12px',
      zIndex: 9999
    }}>
      {toasts.map(t => (
        <div key={t.id} style={{
          padding: '12px 24px',
          borderRadius: '8px',
          background: t.type === 'success' ? 'rgba(16, 185, 129, 0.9)' : 'rgba(239, 68, 68, 0.9)',
          backdropFilter: 'blur(10px)',
          color: 'white',
          boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          animation: 'slideIn 0.3s ease-out',
          minWidth: '200px'
        }}>
          <i className={t.type === 'success' ? 'fas fa-check-circle' : 'fas fa-exclamation-circle'}></i>
          <span style={{ fontWeight: 500 }}>{t.message}</span>
        </div>
      ))}
    </div>
  )
}

export default ToastContainer
