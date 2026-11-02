import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import BottomNav from './components/BottomNav'
import Chat from './components/Chat'
import Companion from './components/Companion'
import Memories from './components/Memories'
import MobileTopBar from './components/MobileTopBar'
import Sidebar from './components/Sidebar'
import Welcome from './components/Welcome'
import { AppProvider, useApp } from './store'

export default function App() {
  return (
    <AppProvider>
      <AppShell />
    </AppProvider>
  )
}

function AppShell() {
  const { loading, milestone, revealScene } = useApp()
  const location = useLocation()
  const isMain = location.pathname === '/chat' || location.pathname === '/memories'

  if (loading) return <div className="loading">加载中…</div>

  return (
    <div className="app">
      {isMain && <Sidebar />}
      <div className="main">
        {isMain && <MobileTopBar />}
        <div className="page-content" key={location.pathname}>
          <Routes>
            <Route path="/" element={<Welcome />} />
            <Route path="/companion" element={<Companion />} />
            <Route path="/chat" element={<Chat />} />
            <Route path="/memories" element={<Memories />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </div>
      </div>
      {isMain && <BottomNav />}
      {milestone && (
        <div className="milestone-toast">
          <span className="milestone-emoji">🎉</span>
          你们的关系升级为「{milestone}」了
        </div>
      )}
      {revealScene && (
        <div className="scene-reveal">
          <div className="scene-reveal-card">
            <div className="scene-reveal-label">初次邂逅</div>
            <div className="scene-reveal-text">{revealScene}</div>
          </div>
        </div>
      )}
    </div>
  )
}
