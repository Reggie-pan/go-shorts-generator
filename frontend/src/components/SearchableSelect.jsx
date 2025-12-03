import React, { useState, useEffect, useRef } from 'react'

const SearchableSelect = ({ options, value, onChange, placeholder, searchable = true, className = '' }) => {
  const [isOpen, setIsOpen] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const wrapperRef = useRef(null)

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (wrapperRef.current && !wrapperRef.current.contains(event.target)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const filteredOptions = searchable 
    ? options.filter(option => option.label.toLowerCase().includes(searchTerm.toLowerCase()))
    : options

  const selectedOption = options.find(o => o.value === value)
  const displayValue = selectedOption ? selectedOption.label : value

  return (
    <div ref={wrapperRef} className={`searchable-select-wrapper ${className}`} style={{ position: 'relative', width: '100%' }}>
      <div 
        onClick={() => setIsOpen(!isOpen)}
        className="searchable-select-trigger"
      >
        <span style={{ color: value ? 'var(--text-primary)' : 'var(--text-muted)' }}>
          {displayValue || placeholder}
        </span>
        <i className={`fas fa-chevron-${isOpen ? 'up' : 'down'}`} style={{ fontSize: '0.8rem', opacity: 0.7 }}></i>
      </div>

      {isOpen && (
        <div className="searchable-select-dropdown">
          {searchable && (
            <div className="searchable-select-search-box">
              <input
                autoFocus
                type="text"
                placeholder="搜尋..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                onClick={(e) => e.stopPropagation()}
                className="searchable-select-input"
              />
            </div>
          )}
          <div className="searchable-select-list">
            {filteredOptions.length > 0 ? (
              filteredOptions.map(option => (
                <div
                  key={option.value}
                  onClick={() => {
                    onChange(option.value)
                    setIsOpen(false)
                    setSearchTerm('')
                  }}
                  className={`searchable-select-option ${value === option.value ? 'selected' : ''}`}
                >
                  {option.label}
                </div>
              ))
            ) : (
              <div className="searchable-select-empty">
                找不到符合的項目
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default SearchableSelect
