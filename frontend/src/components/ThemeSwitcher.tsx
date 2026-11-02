import { useState } from 'react'
import { useApp } from '../store'
import type { ThemeId } from '../types'

const themes: { id: ThemeId; label: string; className: string }[] = [
  { id: 'system', label: '跟随系统', className: 'theme-dot--system' },
  { id: 'white', label: '白', className: 'theme-dot--white' },
  { id: 'black', label: '黑', className: 'theme-dot--black' },
  { id: 'brutalism', label: '新野兽主义', className: 'theme-dot--brutalism' },
  { id: 'cyberpunk', label: '赛博朋克', className: 'theme-dot--cyberpunk' },
  { id: 'anti-refinement', label: '反精致主义', className: 'theme-dot--anti-refinement' },
  { id: 'creative-process', label: '创意过程风', className: 'theme-dot--creative-process' },
  { id: 'organic-brutalism', label: '有机野兽派', className: 'theme-dot--organic-brutalism' },
]

export default function ThemeSwitcher({ direction = 'down' }: { direction?: 'up' | 'down' }) {
  const { theme, setTheme } = useApp()
  const [open, setOpen] = useState(false)

  return (
    <div className="theme-picker">
      <button
        className="theme-trigger"
        onClick={() => setOpen((o) => !o)}
        aria-label="切换主题"
        title="切换主题"
      >
        <span className="theme-trigger-icon">🎨</span>
        <span className="theme-trigger-label">切换主题</span>
        <span className="theme-caret">▾</span>
      </button>
      {open && (
        <div className={`theme-menu theme-menu--${direction}`}>
          {themes.map((t) => (
            <button
              key={t.id}
              className={`theme-option ${t.id === theme ? 'active' : ''}`}
              onClick={() => {
                setTheme(t.id)
                setOpen(false)
              }}
            >
              <span className={`theme-dot ${t.className}`} />
              <span className="theme-option-label">{t.label}</span>
              {t.id === theme && <span className="theme-check">✓</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
