import { NavLink } from 'react-router-dom'

export default function BottomNav() {
  return (
    <nav className="bottom-nav">
      <NavLink to="/chat" className={({ isActive }) => `bottom-nav-item ${isActive ? 'active' : ''}`}>
        💬 聊天
      </NavLink>
      <NavLink to="/memories" className={({ isActive }) => `bottom-nav-item ${isActive ? 'active' : ''}`}>
        📖 记忆
      </NavLink>
    </nav>
  )
}
