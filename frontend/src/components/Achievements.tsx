import { useEffect, useState } from 'react'
import { computeAchievements } from '../achievements'
import { getMemory } from '../api'
import type { FactRow } from '../types'

export default function Achievements() {
  const [facts, setFacts] = useState<FactRow[]>([])
  useEffect(() => {
    getMemory()
      .then((d) => setFacts(d.facts ?? []))
      .catch(() => {})
  }, [])

  const list = computeAchievements(facts, facts.length)
  const unlocked = list.filter((a) => a.unlocked).length

  return (
    <div className="achievements-page">
      <div className="achievements-head">
        <div className="achievements-title">🏅 关系成就</div>
        <div className="achievements-sub">
          已解锁 {unlocked} / {list.length}
        </div>
      </div>
      <div className="achievements-grid">
        {list.map((a) => (
          <div key={a.id} className={`achievement-card ${a.unlocked ? 'on' : 'off'}`}>
            <div className="achievement-card-icon">{a.icon}</div>
            <div className="achievement-card-title">{a.title}</div>
            <div className="achievement-card-desc">{a.desc}</div>
          </div>
        ))}
      </div>
    </div>
  )
}
