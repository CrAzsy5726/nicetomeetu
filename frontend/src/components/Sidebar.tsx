import { NavLink, useNavigate } from 'react-router-dom'
import { useApp } from '../store'
import ThemeSwitcher from './ThemeSwitcher'

export default function Sidebar() {
  const { username, logout } = useApp()
  const navigate = useNavigate()

  return (
    <aside className="sidebar">
      <div className="sidebar-brand">NiceToMeetU</div>

      <nav className="sidebar-nav">
        <NavLink to="/chat" className={({ isActive }) => `side-nav-item ${isActive ? 'active' : ''}`}>
          聊天
        </NavLink>
        <NavLink to="/memories" className={({ isActive }) => `side-nav-item ${isActive ? 'active' : ''}`}>
          记忆
        </NavLink>
      </nav>

      <div className="sidebar-foot">
        <div className="side-user">
          <span className="side-user-name">{username}</span>
          <button
            className="side-user-logout"
            onClick={() => {
              logout()
              navigate('/')
            }}
          >
            换账号
          </button>
        </div>
        <div className="side-themes" role="group" aria-label="主题切换">
          <ThemeSwitcher direction="up" />
        </div>
      </div>
    </aside>
  )
}
