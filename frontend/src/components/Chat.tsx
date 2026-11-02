import { useEffect, useRef, useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { relationLevel, relationStars } from '../relation'
import { avatarMasked, displayName, displayTagline, revealLevel } from '../reveal'
import { useApp } from '../store'
import Markdown from './Markdown'
import PersonaAvatar from './PersonaAvatar'

const EMOJIS = [
  '😀', '😄', '😂', '🤣', '😊', '😍', '😘', '😎', '🤔', '😅',
  '😭', '😢', '😡', '😱', '🤯', '😴', '🥰', '😇', '🙃', '😜',
  '❤️', '💔', '💕', '✨', '🔥', '💯', '👍', '👎', '🙏', '👏',
  '🤝', '💪', '🎉', '🎊', '🌈', '☀️', '🌙', '⭐', '🌧️', '❄️',
  '🍕', '🍰', '🎂', '🍜', '☕', '🍵', '🎮', '🎵', '⚽', '🏀',
  '📚', '🎨', '🐱', '🐶', '🌻', '🍀',
]

export default function Chat() {
  const { persona, familiarity, messages, streaming, sendMessage, switchCompanion } = useApp()
  const navigate = useNavigate()
  const [draft, setDraft] = useState('')
  const [showEmoji, setShowEmoji] = useState(false)
  const [quoteText, setQuoteText] = useState<string | null>(null)
  const [menu, setMenu] = useState<{ x: number; y: number; content: string } | null>(null)
  const endRef = useRef<HTMLDivElement>(null)
  const taRef = useRef<HTMLTextAreaElement>(null)
  const touchStartX = useRef<number | null>(null)

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streaming])

  // 点击任意处关闭右键菜单
  useEffect(() => {
    const close = () => setMenu(null)
    document.addEventListener('click', close)
    return () => document.removeEventListener('click', close)
  }, [])

  if (!persona) return <Navigate to="/" replace />

  const showMenu = (x: number, y: number, content: string) => {
    setMenu({
      x: Math.min(x, window.innerWidth - 150),
      y: Math.min(y, window.innerHeight - 100),
      content,
    })
  }

  const doQuote = (content: string) => {
    setQuoteText(content)
    setMenu(null)
  }

  const doCopy = async (content: string) => {
    try {
      await navigator.clipboard.writeText(content)
    } catch {
      /* ignore */
    }
    setMenu(null)
  }

  const submit = () => {
    const text = draft.trim()
    if (!text || streaming) return
    void sendMessage(text, quoteText ?? undefined)
    setDraft('')
    setQuoteText(null)
    if (taRef.current) taRef.current.style.height = 'auto'
  }

  const stars = relationStars(familiarity)
  const level = revealLevel(persona)
  const nameShown = displayName(persona, level)
  const avatarHidden = avatarMasked(persona, level)
  const taglineShown = displayTagline(persona, level)

  return (
    <div className="chat">
      <header className="chat-header">
        <div className="chat-avatar">
          <PersonaAvatar persona={persona} masked={avatarHidden} />
        </div>
        <div className="chat-id">
          <div className="chat-name">{nameShown}</div>
          {persona.hidden && <div className="chat-tagline">{taglineShown}</div>}
          <div className="familiarity">
            <span className="familiarity-label">
              {familiarity === 0
                ? '你们还不认识'
                : `已了解你 ${familiarity} 件事 · ${relationLevel(familiarity)}`}
            </span>
            <div className="constellation" title={`关系等级：${relationLevel(familiarity)}`}>
              {[0, 1, 2, 3, 4].map((i) => (
                <span key={i} className={`star ${i < stars ? 'star--lit' : 'star--dim'}`}>
                  ★
                </span>
              ))}
            </div>
          </div>
        </div>
        <div className="chat-actions">
          <button
            className="chat-action"
            onClick={() => {
              switchCompanion()
              navigate('/')
            }}
          >
            换伙伴
          </button>
        </div>
      </header>

      {persona.scene && (
        <div className="chat-scene">
          <div className="chat-scene-text">{persona.scene}</div>
        </div>
      )}

      <div className="messages">
        {messages.map((m, i) => {
          const isAssistantWithContent = m.role === 'assistant' && m.content !== ''
          return (
            <div key={i} className={`message message--${m.role}`}>
              {m.role === 'assistant' && (
                <div className="message-avatar">
                  <PersonaAvatar persona={persona} masked={avatarHidden} />
                </div>
              )}
              <div
                className={`bubble ${m.role === 'assistant' && m.content === '' ? 'empty' : ''}`}
                onContextMenu={
                  isAssistantWithContent
                    ? (e) => {
                        e.preventDefault()
                        showMenu(e.clientX, e.clientY, m.content)
                      }
                    : undefined
                }
                onTouchStart={
                  isAssistantWithContent
                    ? (e) => {
                        touchStartX.current = e.touches[0].clientX
                      }
                    : undefined
                }
                onTouchEnd={
                  isAssistantWithContent
                    ? (e) => {
                        if (touchStartX.current === null) return
                        const dx = e.changedTouches[0].clientX - touchStartX.current
                        touchStartX.current = null
                        if (dx < -40) {
                          showMenu(e.changedTouches[0].clientX, e.changedTouches[0].clientY, m.content)
                        }
                      }
                    : undefined
                }
              >
                {m.role === 'assistant' && m.content === '' ? (
                  <span className="typing">
                    <span />
                    <span />
                    <span />
                  </span>
                ) : m.role === 'assistant' ? (
                  <Markdown text={m.content} />
                ) : (
                  <>
                    {m.quote && <div className="message-quote">{m.quote}</div>}
                    {m.content}
                  </>
                )}
              </div>
            </div>
          )
        })}
        <div ref={endRef} />
      </div>

      <div className="chat-input-area">
        {showEmoji && (
          <div className="emoji-panel">
            {EMOJIS.map((e) => (
              <button
                key={e}
                className="emoji-item"
                onClick={() => {
                  setDraft((d) => d + e)
                  taRef.current?.focus()
                }}
              >
                {e}
              </button>
            ))}
          </div>
        )}
        {quoteText && (
          <div className="quote-bar">
            <div className="quote-bar-text">
              <span className="quote-bar-label">引用</span>
              <span className="quote-bar-content">{quoteText}</span>
            </div>
            <button className="quote-bar-close" onClick={() => setQuoteText(null)} aria-label="取消引用">
              ×
            </button>
          </div>
        )}
        <div className="chat-input">
          <textarea
            ref={taRef}
            rows={1}
            value={draft}
            placeholder={`和 ${nameShown} 说点什么…`}
            onChange={(e) => {
              setDraft(e.target.value)
              e.target.style.height = 'auto'
              e.target.style.height = Math.min(e.target.scrollHeight, 140) + 'px'
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                submit()
              }
            }}
          />
          <button
            className={`emoji-btn ${showEmoji ? 'active' : ''}`}
            onClick={() => setShowEmoji((v) => !v)}
            aria-label="表情"
            title="表情"
          >
            😊
          </button>
          <button className="send-btn" onClick={submit} disabled={streaming || !draft.trim()} aria-label="发送">
            ➤
          </button>
        </div>
      </div>

      {menu && (
        <div
          className="context-menu"
          style={{ left: menu.x, top: menu.y }}
          onClick={(e) => e.stopPropagation()}
        >
          <button className="context-menu-item" onClick={() => doQuote(menu.content)}>
            💬 引用
          </button>
          <button className="context-menu-item" onClick={() => doCopy(menu.content)}>
            📋 复制
          </button>
        </div>
      )}
    </div>
  )
}
