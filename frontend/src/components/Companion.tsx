import { useState } from 'react'
import type { CSSProperties, MouseEvent } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { deletePersona } from '../api'
import { personaImage } from '../characters'
import { useApp } from '../store'
import type { Persona } from '../types'
import ConfirmDialog from './ConfirmDialog'
import CustomPersonaForm from './CustomPersonaForm'
import ThemeSwitcher from './ThemeSwitcher'

export default function Companion() {
  const {
    authenticated,
    persona,
    personas,
    selectCompanion,
    selectBlindBox,
    refreshPersonas,
    username,
    logout,
  } = useApp()
  const navigate = useNavigate()
  const [showCustom, setShowCustom] = useState(false)
  const [profileOpen, setProfileOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Persona | null>(null)
  const [deleting, setDeleting] = useState(false)

  if (!authenticated) return <Navigate to="/" replace />
  if (persona) return <Navigate to="/chat" replace />

  const pick = async (id: string) => {
    await selectCompanion(id)
    navigate('/chat')
  }

  const pickBlindBox = async () => {
    await selectBlindBox()
  }

  const handleCreated = async (id: string) => {
    setShowCustom(false)
    await refreshPersonas()
    await pick(id)
  }

  const handleDelete = (e: MouseEvent<HTMLButtonElement>, p: Persona) => {
    e.stopPropagation()
    setDeleteTarget(p)
  }

  const confirmDelete = async () => {
    if (!deleteTarget || deleting) return
    setDeleting(true)
    try {
      await deletePersona(deleteTarget.id)
      await refreshPersonas()
      setDeleteTarget(null)
    } catch (err) {
      console.error(err)
    } finally {
      setDeleting(false)
    }
  }

  const isDeletable = (id: string) => id.startsWith('custom_') || id.startsWith('blindbox_')

  return (
    <div className="onboarding">
      <div className="theme-switcher" role="group" aria-label="主题切换">
        <ThemeSwitcher />
      </div>
      <div className="progress-dots">
        <span className="dot active" />
        <span className="dot active" />
      </div>

      <div className="companion-head">
        <div className="companion-title">选一个伙伴陪你</div>
        <div className="companion-sub">TA 会记住你，慢慢懂你</div>
      </div>

      <div className="persona-grid">
        {personas.map((p) => (
          <div
            className="persona-card"
            key={p.id}
            style={{ '--card-accent': p.accent } as CSSProperties}
            onClick={() => pick(p.id)}
          >
            {isDeletable(p.id) && (
              <button
                className="persona-delete"
                onClick={(e) => handleDelete(e, p)}
                title="删除"
                aria-label="删除"
              >
                ×
              </button>
            )}
            {p.hidden ? (
              <div className="persona-body persona-body--mystery">
                <span className="mystery-mark">?</span>
              </div>
            ) : (
              <div className="persona-body">
                <img className="persona-img" src={personaImage(p)} alt={p.name} />
              </div>
            )}
            <div className="persona-name">{p.hidden ? '？？？' : p.name}</div>
            <div className="persona-tagline" style={{ color: p.accent }}>
              {p.hidden ? '神秘人' : p.tagline}
            </div>
            <div className="persona-pick">
              {p.hidden ? '再次相遇' : `认识 ${p.name}`}
            </div>
          </div>
        ))}

        <button className="persona-card persona-card--blindbox" onClick={pickBlindBox}>
          <div className="persona-body persona-body--blindbox">
            <span className="blindbox-mark">🎁</span>
          </div>
          <div className="persona-name">盲盒</div>
          <div className="persona-tagline">你一无所知的神秘邂逅</div>
          <div className="persona-pick">抽一个伙伴</div>
        </button>

        <button className="persona-card persona-card--custom" onClick={() => setShowCustom(true)}>
          <div className="persona-body persona-body--custom">
            <span className="custom-plus">＋</span>
          </div>
          <div className="persona-name">自定义</div>
          <div className="persona-tagline">上传头像 · 起名 · 选性格</div>
          <div className="persona-pick">创建一个伙伴</div>
        </button>
      </div>

      {showCustom && (
        <CustomPersonaForm onClose={() => setShowCustom(false)} onCreated={(p) => handleCreated(p.id)} />
      )}

      {deleteTarget && (
        <ConfirmDialog
          title="删除伙伴"
          message={`确定要删除${deleteTarget.hidden ? '这个神秘伙伴' : `「${deleteTarget.name}」`}吗？删除后无法恢复。`}
          confirmText="删除"
          cancelText="取消"
          busy={deleting}
          onConfirm={confirmDelete}
          onCancel={() => setDeleteTarget(null)}
        />
      )}

      <div className="profile-corner">
        {profileOpen && (
          <div className="profile-menu">
            <div className="profile-menu-user">👤 {username}</div>
            <button
              className="profile-menu-item"
              onClick={() => {
                logout()
                navigate('/')
              }}
            >
              换账号
            </button>
          </div>
        )}
        <button
          className="profile-trigger"
          onClick={() => setProfileOpen((o) => !o)}
          aria-label="个人中心"
          title="个人中心"
        >
          {username.charAt(0).toUpperCase()}
        </button>
      </div>
    </div>
  )
}
