import type {
  DiaryEntry,
  FactRow,
  Persona,
  PersonalityOption,
  SelectPersonaResponse,
  ResetResponse,
  State,
} from './types'

const BASE = '/api'

let currentToken = ''

export function setToken(t: string) {
  currentToken = t
}
export function getToken() {
  return currentToken
}

async function json<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${currentToken}` },
    ...init,
  })
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}))
    throw new Error((body as { error?: string }).error ?? `请求失败 (${resp.status})`)
  }
  return (await resp.json()) as T
}

export interface AuthResponse {
  token: string
  user_id: string
  username: string
}

export function register(username: string, password: string): Promise<AuthResponse> {
  return json('/auth/register', { method: 'POST', body: JSON.stringify({ username, password }) })
}

export function login(username: string, password: string): Promise<AuthResponse> {
  return json('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) })
}

export function logout(): Promise<{ ok: boolean }> {
  return json('/auth/logout', { method: 'POST' })
}

export function getPersonas(): Promise<{ personas: Persona[] }> {
  return json('/personas')
}

export function getPersonalities(): Promise<{ personalities: PersonalityOption[] }> {
  return json('/personalities')
}

export function deletePersona(id: string): Promise<{ ok: boolean }> {
  return json(`/personas/${id}`, { method: 'DELETE' })
}

// 抽盲盒：随机隐藏角色 + 邂逅场景
export function drawBlindBox(): Promise<SelectPersonaResponse> {
  return json('/personas/blindbox', { method: 'POST' })
}

// 创建自定义人设（multipart 上传头像）
export async function createCustomPersona(form: FormData): Promise<{ persona: Persona }> {
  const resp = await fetch(`${BASE}/personas/custom`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${currentToken}` },
    body: form,
  })
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}))
    throw new Error((body as { error?: string }).error ?? `请求失败 (${resp.status})`)
  }
  return (await resp.json()) as { persona: Persona }
}

export function getState(): Promise<State> {
  return json('/state')
}

export function selectPersona(personaId: string): Promise<SelectPersonaResponse> {
  return json('/persona', { method: 'POST', body: JSON.stringify({ persona_id: personaId }) })
}

export function clearPersona(): Promise<{ ok: boolean }> {
  return json('/persona/clear', { method: 'POST' })
}

export function reset(): Promise<ResetResponse> {
  return json('/reset', { method: 'POST' })
}

export function getMemory(): Promise<{ facts: FactRow[] }> {
  return json('/memory')
}

export function deleteFact(id: string): Promise<{ ok: boolean }> {
  return json(`/memory/${id}`, { method: 'DELETE' })
}

export function correctFact(id: string, type: string, content: string): Promise<{ ok: boolean }> {
  return json('/memory/correct', { method: 'POST', body: JSON.stringify({ id, type, content }) })
}

export function getDigest(): Promise<{ digest: string }> {
  return json('/memory/digest')
}

export function flashback(sessionId: string): Promise<{ message: string }> {
  return json('/flashback', { method: 'POST', body: JSON.stringify({ session_id: sessionId }) })
}

export function getDiary(): Promise<{ entries: DiaryEntry[] }> {
  return json('/diary')
}

export function writeDiary(): Promise<{ entry: string }> {
  return json('/diary', { method: 'POST' })
}

// 流式聊天：逐段回调 delta，结束时返回。
export async function chatStream(
  sessionId: string,
  message: string,
  quote: string | undefined,
  onDelta: (delta: string) => void,
): Promise<void> {
  const resp = await fetch(`${BASE}/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${currentToken}` },
    body: JSON.stringify({ session_id: sessionId, message, quote }),
  })
  if (!resp.ok || !resp.body) {
    const body = await resp.json().catch(() => ({}))
    throw new Error((body as { error?: string }).error ?? `请求失败 (${resp.status})`)
  }

  const reader = resp.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''

    for (const line of lines) {
      const t = line.trim()
      if (!t.startsWith('data:')) continue
      const data = t.slice(5).trim()
      if (!data) continue
      let obj: { delta?: string; error?: string }
      try {
        obj = JSON.parse(data)
      } catch {
        continue
      }
      if (obj.error) throw new Error(obj.error)
      if (obj.delta) onDelta(obj.delta)
    }
  }
}

// 订阅实时事件流（SSE）。返回取消函数。
export function subscribeEvents(
  onEvent: (ev: { type: string; familiarity: number }) => void,
): () => void {
  const controller = new AbortController()
  ;(async () => {
    try {
      const resp = await fetch(`${BASE}/events`, {
        headers: { Authorization: `Bearer ${currentToken}` },
        signal: controller.signal,
      })
      if (!resp.body) return
      const reader = resp.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      for (;;) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''
        for (const line of lines) {
          const t = line.trim()
          if (!t.startsWith('data:')) continue
          const data = t.slice(5).trim()
          if (!data) continue
          try {
            onEvent(JSON.parse(data))
          } catch {
            /* ignore */
          }
        }
      }
    } catch {
      /* 连接中断，静默退出 */
    }
  })()
  return () => controller.abort()
}
