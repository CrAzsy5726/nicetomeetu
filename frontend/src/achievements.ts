import type { FactRow } from './types'

export interface Achievement {
  id: string
  icon: string
  title: string
  desc: string
  unlocked: boolean
}

export function computeAchievements(facts: FactRow[], familiarity: number): Achievement[] {
  const hasName = facts.some((f) => f.type === 'basic' && /叫|名字|称呼/.test(f.content))
  return [
    { id: 'first', icon: '🌱', title: '初次相识', desc: '记住你的第一件事', unlocked: familiarity >= 1 },
    { id: 'name', icon: '📛', title: '记住你的名字', desc: '记住了你的称呼', unlocked: hasName },
    { id: 'growing', icon: '🌿', title: '渐入佳境', desc: '了解你 5 件事', unlocked: familiarity >= 5 },
    { id: 'close', icon: '🌳', title: '无话不谈', desc: '了解你 15 件事', unlocked: familiarity >= 15 },
    { id: 'soul', icon: '💎', title: '知己', desc: '了解你 30 件事', unlocked: familiarity >= 30 },
  ]
}
