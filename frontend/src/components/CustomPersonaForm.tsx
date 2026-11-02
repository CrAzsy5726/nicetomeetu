import { useEffect, useRef, useState } from 'react'
import { createCustomPersona, getPersonalities } from '../api'
import type { PersonalityOption, Persona } from '../types'

interface Props {
  onClose: () => void
  onCreated: (p: Persona) => void
}

export default function CustomPersonaForm({ onClose, onCreated }: Props) {
  const [name, setName] = useState('')
  const [avatarFile, setAvatarFile] = useState<File | null>(null)
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null)
  const [personalities, setPersonalities] = useState<PersonalityOption[]>([])
  const [personality, setPersonality] = useState('') // 预设 ID，或 'custom'
  const [customText, setCustomText] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    getPersonalities()
      .then((d) => setPersonalities(d.personalities ?? []))
      .catch(() => {})
  }, [])

  const onFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (!f) return
    setAvatarFile(f)
    setAvatarPreview(URL.createObjectURL(f))
  }

  const personalityValue = personality === 'custom' ? customText.trim() : personality

  const submit = async () => {
    if (!name.trim() || !personalityValue || !avatarFile) {
      setError('请填写名字、选择或填写性格并上传头像')
      return
    }
    setBusy(true)
    setError('')
    try {
      const form = new FormData()
      form.append('name', name.trim())
      form.append('personality', personalityValue)
      form.append('avatar', avatarFile)
      const r = await createCustomPersona(form)
      onCreated(r.persona)
    } catch (e) {
      setError(e instanceof Error ? e.message : '出错了')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="memory-overlay" onClick={onClose}>
      <div className="memory-panel" onClick={(e) => e.stopPropagation()}>
        <header className="memory-head">
          <div className="memory-title">自定义伙伴</div>
          <button className="memory-close" onClick={onClose} aria-label="关闭">
            ×
          </button>
        </header>

        <div className="custom-form">
          <button className="custom-avatar-upload" onClick={() => fileRef.current?.click()}>
            {avatarPreview ? (
              <img src={avatarPreview} alt="头像预览" />
            ) : (
              <span>+ 上传头像</span>
            )}
          </button>
          <input ref={fileRef} type="file" accept="image/*" onChange={onFile} hidden />

          <input
            className="welcome-input"
            placeholder="名字"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />

          <div className="custom-personality-label">选择性格</div>
          <div className="custom-personalities">
            {personalities.map((p) => (
              <button
                key={p.id}
                className={`personality-chip ${personality === p.id ? 'active' : ''}`}
                onClick={() => setPersonality(p.id)}
              >
                {p.label}
              </button>
            ))}
            <button
              className={`personality-chip ${personality === 'custom' ? 'active' : ''}`}
              onClick={() => setPersonality('custom')}
            >
              ＋ 添加性格
            </button>
          </div>

          {personality === 'custom' && (
            <textarea
              className="custom-personality-text"
              placeholder="描述你想要的性格，例如：成熟稳重、爱讲冷笑话、像个温柔的大哥哥…"
              value={customText}
              onChange={(e) => setCustomText(e.target.value)}
              rows={3}
            />
          )}

          {error && <div className="welcome-error">{error}</div>}
          <button className="welcome-btn" onClick={submit} disabled={busy}>
            {busy ? '…' : '创建伙伴 →'}
          </button>
        </div>
      </div>
    </div>
  )
}
