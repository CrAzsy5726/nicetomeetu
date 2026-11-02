export interface Persona {
  id: string
  name: string
  avatar: string
  tagline: string
  personality: string
  style: string
  opening: string
  scene?: string
  accent: string
  hidden?: boolean
  turns?: number
}

export interface Message {
  role: 'user' | 'assistant'
  content: string
  quote?: string
}

export interface FactRow {
  id: string
  type: string
  content: string
  importance: number
  created_at: number
  updated_at: number
}

export interface UserInfo {
  id: string
  name: string
  persona_id: string
}

export interface DiaryEntry {
  id: number
  content: string
  created_at: number
}

export interface State {
  persona_id?: string
  persona: Persona | null
  familiarity: number
  session_id?: string
  messages: Message[]
}

export interface SelectPersonaResponse {
  session_id: string
  opening: string
  persona: Persona
  messages?: Message[]
}

export interface ResetResponse {
  session_id: string
  opening: string
}

export type ThemeId =
  | 'brutalism'
  | 'cyberpunk'
  | 'black'
  | 'white'
  | 'system'
  | 'anti-refinement'
  | 'creative-process'
  | 'organic-brutalism'

export interface PersonalityOption {
  id: string
  label: string
  personality: string
  style: string
  accent: string
}
