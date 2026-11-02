import type { Persona } from './types'

// 盲盒解锁档位：按累计交互轮数（持久化在 persona.turns）。
// 0=完全神秘 1=已知名字 2=已知头像 3=完全揭晓（含性格 tagline）
export function revealLevel(persona: Persona): number {
  if (!persona.hidden) return 3
  const turns = persona.turns ?? 0
  if (turns >= 10) return 3
  if (turns >= 5) return 2
  if (turns >= 2) return 1
  return 0
}

export function displayName(persona: Persona, level: number): string {
  return level >= 1 ? persona.name : '？？？'
}

export function displayTagline(persona: Persona, level: number): string {
  return level >= 3 ? persona.tagline : '神秘人'
}

export function avatarMasked(persona: Persona, level: number): boolean {
  return persona.hidden === true && level < 2
}
