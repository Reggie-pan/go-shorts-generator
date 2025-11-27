import React, { useEffect } from 'react'

export default function SwaggerUI() {
  useEffect(() => {
    // 動態載入 Swagger UI CSS 和 JS
    const link = document.createElement('link')
    link.rel = 'stylesheet'
    link.href = 'https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.10.0/swagger-ui.css'
    document.head.appendChild(link)

    const script = document.createElement('script')
    script.src = 'https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.10.0/swagger-ui-bundle.js'
    script.onload = () => {
      window.SwaggerUIBundle({
        url: '/api/v1/swagger.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          window.SwaggerUIBundle.presets.apis,
          window.SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: 'BaseLayout',
        defaultModelsExpandDepth: 1,
        defaultModelExpandDepth: 1,
        docExpansion: 'list',
        filter: true,
        showRequestHeaders: true,
        tryItOutEnabled: true
      })
    }
    document.body.appendChild(script)

    return () => {
      document.head.removeChild(link)
      document.body.removeChild(script)
    }
  }, [])

  return (
    <div style={{ 
      minHeight: '100vh',
      background: 'linear-gradient(135deg, #0f172a, #1e293b)'
    }}>
      <div style={{ 
        maxWidth: '1400px', 
        margin: '0 auto', 
        padding: '24px 16px' 
      }}>
        <div style={{ marginBottom: '24px' }}>
          <a 
            href="/" 
            style={{ 
              color: '#0ea5e9', 
              textDecoration: 'none',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '8px',
              fontSize: '0.9375rem',
              fontWeight: 500
            }}
          >
            <i className="fas fa-arrow-left"></i> 返回主頁
          </a>
        </div>
        <div 
          id="swagger-ui" 
          style={{ 
            background: 'white',
            borderRadius: '12px',
            padding: '24px',
            boxShadow: '0 20px 60px rgba(0,0,0,0.4)'
          }}
        ></div>
      </div>
    </div>
  )
}
