import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { useApp } from '../store'
import ThemeSwitcher from './ThemeSwitcher'

export default function Welcome() {
  const { authenticated, register, login } = useApp()
  const navigate = useNavigate()
  const [mode, setMode] = useState<'register' | 'login'>('register')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  if (authenticated) return <Navigate to="/companion" replace />

  const submit = async () => {
    if (busy) return
    if (!username.trim() || password.length < 6) {
      setError('用户名不能为空，密码至少 6 位')
      return
    }
    setBusy(true)
    setError('')
    try {
      if (mode === 'register') await register(username.trim(), password)
      else await login(username.trim(), password)
      navigate('/companion')
    } catch (e) {
      setError(e instanceof Error ? e.message : '出错了')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="onboarding">
      <div className="theme-switcher" role="group" aria-label="主题切换">
        <ThemeSwitcher />
      </div>
      <div className="progress-dots">
        <span className="dot active" />
        <span className="dot" />
      </div>

      <div className="welcome-mascot">💬</div>

      <div className="brand">
        <div className="brand-logo">NiceToMeetU</div>
        <div className="brand-sub">从陌生开始，慢慢变熟悉</div>
      </div>

      <div className="welcome-card">
        <div className="welcome-label">{mode === 'register' ? '先注册一个账号' : '欢迎回来'}</div>
        <input
          className="welcome-input"
          value={username}
          placeholder="用户名"
          autoFocus
          onChange={(e) => setUsername(e.target.value)}
        />
        <input
          className="welcome-input"
          type="password"
          value={password}
          placeholder="密码（至少 6 位）"
          onChange={(e) => setPassword(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') submit()
          }}
        />
        {error && <div className="welcome-error">{error}</div>}
        <button className="welcome-btn" onClick={submit} disabled={busy}>
          {busy ? '…' : mode === 'register' ? '注册并开始 →' : '登录 →'}
        </button>
      </div>

      <button
        className="welcome-toggle"
        onClick={() => {
          setMode((m) => (m === 'register' ? 'login' : 'register'))
          setError('')
        }}
      >
        {mode === 'register' ? '已有账号？去登录' : '没有账号？去注册'}
      </button>
    </div>
  )
}
