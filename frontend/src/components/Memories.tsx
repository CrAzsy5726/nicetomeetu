import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useApp } from '../store'
import Achievements from './Achievements'
import DiaryPanel from './DiaryPanel'
import MemoryPanel from './MemoryPanel'
import ThemeSwitcher from './ThemeSwitcher'

type Tab = 'facts' | 'diary' | 'achievements'

export default function Memories() {
  const { username, logout } = useApp()
  const navigate = useNavigate()
  const [tab, setTab] = useState<Tab>('facts')

  return (
    <div className="memories">
      <div className="memories-account">
        <span className="memories-account-user">👤 {username}</span>
        <div className="memories-account-actions">
          <ThemeSwitcher direction="down" />
          <button
            className="memories-account-logout"
            onClick={() => {
              logout()
              navigate('/')
            }}
          >
            换账号
          </button>
        </div>
      </div>
      <div className="memories-tabs">
        <button className={`tab ${tab === 'facts' ? 'active' : ''}`} onClick={() => setTab('facts')}>
          画像
        </button>
        <button className={`tab ${tab === 'diary' ? 'active' : ''}`} onClick={() => setTab('diary')}>
          日记
        </button>
        <button
          className={`tab ${tab === 'achievements' ? 'active' : ''}`}
          onClick={() => setTab('achievements')}
        >
          成就
        </button>
      </div>
      <div className="memories-body">
        {tab === 'facts' && <MemoryPanel />}
        {tab === 'diary' && <DiaryPanel />}
        {tab === 'achievements' && <Achievements />}
      </div>
    </div>
  )
}
