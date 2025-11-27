import React from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import App from './pages/App'
import SwaggerUI from './pages/SwaggerUI'
import './style.css'

createRoot(document.getElementById('root')).render(
  <BrowserRouter>
    <Routes>
      <Route path="/" element={<App />} />
      <Route path="/swagger" element={<SwaggerUI />} />
    </Routes>
  </BrowserRouter>
)
