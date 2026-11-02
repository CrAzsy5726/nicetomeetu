import { useEffect, useState } from 'react'
import { correctFact, deleteFact, getDigest, getMemory } from '../api'
import { useApp } from '../store'
import type { FactRow } from '../types'

const typeLabels: Record<string, { label: string; icon: string }> = {
  basic: { label: '基本信息', icon: '👤' },
  interest: { label: '兴趣爱好', icon: '🎯' },
  personality: { label: '性格风格', icon: '✨' },
  emotion: { label: '情绪状态', icon: '💭' },
  episodic: { label: '近期事件', icon: '📅' },
}

const factTypeOrder = ['basic', 'interest', 'personality', 'emotion', 'episodic']

export default function MemoryPanel() {
  const { refreshFamiliarity, resetMemory } = useApp()
  const [facts, setFacts] = useState<FactRow[]>([])
  const [digest, setDigest] = useState('')
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<string | null>(null)
  const [editText, setEditText] = useState('')

  useEffect(() => {
    ;(async () => {
      try {
        const m = await getMemory()
        setFacts(m.facts ?? [])
      } catch (e) {
        console.error(e)
      } finally {
        setLoading(false)
      }
      try {
        const d = await getDigest()
        setDigest(d.digest)
      } catch (e) {
        console.error(e)
      }
    })()
  }, [])

  const handleDelete = async (id: string) => {
    try {
      await deleteFact(id)
      setFacts((f) => f.filter((x) => x.id !== id))
      refreshFamiliarity()
    } catch (e) {
      console.error(e)
    }
  }

  const startEdit = (f: FactRow) => {
    setEditing(f.id)
    setEditText(f.content)
  }

  const saveEdit = async (f: FactRow) => {
    const text = editText.trim()
    if (!text) return
    try {
      await correctFact(f.id, f.type, text)
      setFacts((prev) => prev.map((x) => (x.id === f.id ? { ...x, content: text } : x)))
      setEditing(null)
      refreshFamiliarity()
    } catch (e) {
      console.error(e)
    }
  }

  const grouped: Record<string, FactRow[]> = {}
  for (const f of facts) {
    ;(grouped[f.type] ??= []).push(f)
  }

  const typeCounts: Record<string, number> = {}
  for (const f of facts) typeCounts[f.type] = (typeCounts[f.type] ?? 0) + 1
  const maxCount = Math.max(1, ...Object.values(typeCounts))

  return (
    <div className="memory-view">
      <div className="memory-view-head">
        <span className="memory-view-title">我们认识 {relationshipAge(facts)} 天了</span>
        <span className="memory-view-count">
          {facts.length === 0 ? '还不熟' : `已了解你 ${facts.length} 件事`}
        </span>
      </div>

      {digest && <div className="memory-digest">{digest}</div>}

      {facts.length > 0 && (
        <div className="memory-coverage">
          <div className="coverage-title">我对你的了解</div>
          {factTypeOrder.map((t) => {
            const c = typeCounts[t] ?? 0
            return (
              <div className="coverage-row" key={t}>
                <span className="coverage-label">
                  {typeLabels[t]?.icon} {typeLabels[t]?.label ?? t}
                </span>
                <div className="coverage-bar">
                  <div className="coverage-fill" style={{ width: `${(c / maxCount) * 100}%` }} />
                </div>
                <span className="coverage-count">{c > 0 ? c : '空'}</span>
              </div>
            )
          })}
        </div>
      )}

      <div className="memory-groups">
        {loading ? (
          <div className="memory-empty">加载中…</div>
        ) : facts.length === 0 ? (
          <div className="memory-empty">我们还不熟，多聊聊让我认识你吧。</div>
        ) : (
          Object.entries(grouped).map(([type, items]) => (
            <div className="memory-group" key={type}>
              <div className="memory-group-title">
                {typeLabels[type]?.icon} {typeLabels[type]?.label ?? type}
              </div>
              {items.map((f) => (
                <div className="memory-fact" key={f.id}>
                  {editing === f.id ? (
                    <>
                      <input
                        className="memory-edit"
                        value={editText}
                        onChange={(e) => setEditText(e.target.value)}
                        autoFocus
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') saveEdit(f)
                          if (e.key === 'Escape') setEditing(null)
                        }}
                      />
                      <button className="memory-save" onClick={() => saveEdit(f)}>
                        保存
                      </button>
                      <button className="memory-cancel" onClick={() => setEditing(null)}>
                        取消
                      </button>
                    </>
                  ) : (
                    <>
                      <div className="memory-fact-main">
                        <span className="memory-content" onClick={() => startEdit(f)} title="点击编辑">
                          {f.content}
                        </span>
                        <span className="memory-time">
                          {relativeTime(f.created_at)}
                          <span
                            className={`strength-dot strength-dot--${strengthInfo(f).cls}`}
                            title={`记忆强度：${strengthInfo(f).label}`}
                          />
                        </span>
                      </div>
                      <button className="memory-del" onClick={() => handleDelete(f.id)} title="删除">
                        ×
                      </button>
                    </>
                  )}
                </div>
              ))}
            </div>
          ))
        )}
      </div>

      {facts.length > 0 && (
        <button className="memory-reset-btn" onClick={() => void resetMemory()}>
          🔄 重新认识
        </button>
      )}
    </div>
  )
}

function strengthInfo(f: FactRow): { label: string; cls: string } {
  const ageDays = (Date.now() / 1000 - f.created_at) / 86400
  const score = f.importance - ageDays * 0.25
  if (score >= 4) return { label: '强', cls: 'strong' }
  if (score >= 2.5) return { label: '中', cls: 'normal' }
  return { label: '淡', cls: 'fading' }
}

function relativeTime(ts: number): string {
  if (!ts) return ''
  const diff = Date.now() / 1000 - ts
  const day = 86400
  if (diff < day) return '今天'
  if (diff < 2 * day) return '昨天'
  if (diff < 7 * day) return `${Math.floor(diff / day)} 天前`
  return `${Math.floor(diff / (7 * day))} 周前`
}

function relationshipAge(facts: FactRow[]): number {
  if (facts.length === 0) return 0
  let min = Infinity
  for (const f of facts) min = Math.min(min, f.created_at)
  if (!isFinite(min)) return 0
  return Math.max(1, Math.ceil((Date.now() / 1000 - min) / 86400))
}
