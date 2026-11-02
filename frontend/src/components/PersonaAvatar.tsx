import { personaImage } from '../characters'
import type { Persona } from '../types'

interface Props {
  persona: Persona
  masked?: boolean
  className?: string
}

// 圆形头像：盲盒隐藏时显示「?」，揭示后按头像类型（上传图 / emoji）渲染。
export default function PersonaAvatar({ persona, masked, className = '' }: Props) {
  if (masked) {
    return (
      <div className={`persona-avatar-mask ${className}`} aria-label="神秘伙伴">
        ?
      </div>
    )
  }
  const src = personaImage(persona)
  if (src.startsWith('/')) {
    return <img className={`persona-avatar-img ${className}`} src={src} alt={persona.name} />
  }
  return (
    <div
      className={`persona-avatar-emoji ${className}`}
      style={{ background: persona.accent ? `${persona.accent}22` : undefined }}
      aria-label={persona.name}
    >
      {src}
    </div>
  )
}
