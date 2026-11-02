import { useEffect, useState } from 'react'
import { getDiary, writeDiary } from '../api'
import type { DiaryEntry } from '../types'

export default function DiaryPanel() {
  const [entries, setEntries] = useState<DiaryEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [writing, setWriting] = useState(false)

  useEffect(() => {
    ;(async () => {
      try {
        const d = await getDiary()
        setEntries(d.entries ?? [])
      } catch (e) {
        console.error(e)
      } finally {
        setLoading(false)
      }
    })()
  }, [])

  const handleWrite = async () => {
    if (writing) return
    setWriting(true)
    try {
      const r = await writeDiary()
      setEntries((prev) => [
        { id: Date.now(), content: r.entry, created_at: Math.floor(Date.now() / 1000) },
        ...prev,
      ])
    } catch (e) {
      console.error(e)
    } finally {
      setWriting(false)
    }
  }

  return (
    <div className="diary-view">
      <button className="diary-btn" onClick={handleWrite} disabled={writing}>
        {writing ? '写着呢…' : '✍️ 写一篇今天的日记'}
      </button>

      <div className="diary-list">
        {loading ? (
          <div className="memory-empty">加载中…</div>
        ) : entries.length === 0 ? (
          <div className="memory-empty">还没有日记，点上面的按钮写第一篇吧～</div>
        ) : (
          entries.map((e) => (
            <div className="diary-entry" key={e.id}>
              <div className="diary-date">{formatDate(e.created_at)}</div>
              <div className="diary-content">{e.content}</div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function formatDate(ts: number): string {
  const d = new Date(ts * 1000)
  return `${d.getMonth() + 1}月${d.getDate()}日`
}
