import type { Persona } from './types'

const map: Record<string, string> = {
  xiaoman: '/characters/xiaoman.png',
  aye: '/characters/aye.png',
  zhou: '/characters/zhou.png',
}

export function characterImage(personaId: string): string {
  return map[personaId] ?? ''
}

// 统一取人物图片：预设用本地图，自定义用上传路径，盲盒用 emoji
export function personaImage(persona: Persona): string {
  if (persona.avatar.startsWith('/')) return persona.avatar
  return characterImage(persona.id) || persona.avatar
}
