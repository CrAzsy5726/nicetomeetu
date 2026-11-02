import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import {
  chatStream,
  clearPersona,
  drawBlindBox,
  flashback,
  getPersonas,
  getState,
  login as apiLogin,
  logout as apiLogout,
  register as apiRegister,
  reset,
  selectPersona,
  setToken as setApiToken,
  subscribeEvents,
} from './api'
import { levelRank, relationLevel } from './relation'
import type { Message, Persona, ThemeId } from './types'

const THEME_KEY = 'ntmu-theme'
const TOKEN_KEY = 'ntmu-token'
const USERNAME_KEY = 'ntmu-username'

const THEME_IDS: ThemeId[] = [
  'brutalism',
  'cyberpunk',
  'black',
  'white',
  'system',
  'anti-refinement',
  'creative-process',
  'organic-brutalism',
]

interface AppState {
  theme: ThemeId
  setTheme: (t: ThemeId) => void
  authenticated: boolean
  username: string
  personas: Persona[]
  persona: Persona | null
  familiarity: number
  messages: Message[]
  streaming: boolean
  loading: boolean
  milestone: string | null
  revealScene: string | null
  register: (username: string, password: string) => Promise<void>
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  selectCompanion: (id: string) => Promise<void>
  selectBlindBox: () => Promise<Persona | undefined>
  switchCompanion: () => void
  resetMemory: () => Promise<void>
  sendMessage: (text: string, quote?: string) => Promise<void>
  refreshFamiliarity: () => void
  refreshPersonas: () => Promise<void>
}

const AppContext = createContext<AppState | null>(null)

export function AppProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<ThemeId>(() => {
    const saved = localStorage.getItem(THEME_KEY) as ThemeId | null
    return saved && THEME_IDS.includes(saved) ? saved : 'brutalism'
  })
  const [token, setToken] = useState<string>(() => localStorage.getItem(TOKEN_KEY) || '')
  const [username, setUsername] = useState<string>(() => localStorage.getItem(USERNAME_KEY) || '')
  const [personas, setPersonas] = useState<Persona[]>([])
  const [persona, setPersona] = useState<Persona | null>(null)
  const [familiarity, setFamiliarity] = useState(0)
  const [sessionId, setSessionId] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [streaming, setStreaming] = useState(false)
  const [loading, setLoading] = useState(true)
  const [milestone, setMilestone] = useState<string | null>(null)
  const [revealScene, setRevealScene] = useState<string | null>(null)

  const prevLevelRef = useRef<string | null>(null)
  const lastActivityRef = useRef<number>(Date.now())
  const autoFlashFiredRef = useRef<boolean>(false)

  // 主题应用：system 时跟随系统深浅色，并监听系统主题变化
  useEffect(() => {
    const apply = () => {
      let resolved = theme
      if (theme === 'system') {
        resolved = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'black' : 'white'
      }
      document.documentElement.dataset.theme = resolved
    }
    apply()
    localStorage.setItem(THEME_KEY, theme)

    if (theme === 'system') {
      const mq = window.matchMedia('(prefers-color-scheme: dark)')
      mq.addEventListener('change', apply)
      return () => mq.removeEventListener('change', apply)
    }
  }, [theme])

  // 关系升级检测
  useEffect(() => {
    const cur = relationLevel(familiarity)
    const prev = prevLevelRef.current
    prevLevelRef.current = cur
    if (prev !== null && levelRank(cur) > levelRank(prev)) {
      setMilestone(cur)
    }
  }, [familiarity])

  useEffect(() => {
    if (!milestone) return
    const t = setTimeout(() => setMilestone(null), 3200)
    return () => clearTimeout(t)
  }, [milestone])

  const loadUserState = useCallback(async () => {
    prevLevelRef.current = null
    try {
      const s = await getState()
      setPersona(s.persona)
      setFamiliarity(s.familiarity)
      setSessionId(s.session_id ?? '')
      setMessages(s.messages ?? [])
    } catch {
      setPersona(null)
      setFamiliarity(0)
      setSessionId('')
      setMessages([])
    }
  }, [])

  // 初始加载：登录后才拉人设与用户状态
  useEffect(() => {
    ;(async () => {
      if (token !== '') {
        setApiToken(token)
        try {
          const p = await getPersonas()
          setPersonas(p.personas ?? [])
        } catch (e) {
          console.error(e)
        }
        await loadUserState()
      } else {
        setPersona(null)
        setMessages([])
        setFamiliarity(0)
      }
      setLoading(false)
    })()
  }, [token, loadUserState])

  const refreshFamiliarity = useCallback(async () => {
    try {
      const s = await getState()
      setFamiliarity(s.familiarity)
    } catch {
      /* ignore */
    }
  }, [])

  const refreshPersonas = useCallback(async () => {
    try {
      const p = await getPersonas()
      setPersonas(p.personas)
    } catch {
      /* ignore */
    }
  }, [])

  // SSE 实时刷新了解度
  useEffect(() => {
    if (!persona) return
    return subscribeEvents((ev) => {
      if (ev.type === 'memory_updated' || ev.type === 'memory_reset') {
        setFamiliarity(ev.familiarity)
      }
    })
  }, [persona, token])

  const applyAuth = (tok: string, name: string) => {
    setToken(tok)
    setUsername(name)
    setApiToken(tok)
    localStorage.setItem(TOKEN_KEY, tok)
    localStorage.setItem(USERNAME_KEY, name)
    setPersona(null)
    setMessages([])
    setFamiliarity(0)
    setStreaming(false)
    autoFlashFiredRef.current = false
    lastActivityRef.current = Date.now()
    loadUserState()
  }

  const register = async (name: string, password: string) => {
    const r = await apiRegister(name, password)
    applyAuth(r.token, r.username)
  }

  const login = async (name: string, password: string) => {
    const r = await apiLogin(name, password)
    applyAuth(r.token, r.username)
  }

  const logout = async () => {
    try {
      await apiLogout()
    } catch {
      /* ignore */
    }
    setToken('')
    setUsername('')
    setApiToken('')
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USERNAME_KEY)
    setPersona(null)
    setMessages([])
    setFamiliarity(0)
    setStreaming(false)
  }

  const selectCompanion = async (id: string) => {
    const r = await selectPersona(id)
    setPersona(r.persona)
    setSessionId(r.session_id)
    setMessages(r.messages?.length ? r.messages : [{ role: 'assistant', content: r.opening }])
    refreshFamiliarity()
  }

  const selectBlindBox = async () => {
    const r = await drawBlindBox()
    setPersona(r.persona)
    setSessionId(r.session_id)
    setMessages([{ role: 'assistant', content: r.opening }])
    refreshFamiliarity()
    refreshPersonas()
    if (r.persona.scene) {
      setRevealScene(r.persona.scene)
      setTimeout(() => setRevealScene(null), 3600)
    }
    return r.persona
  }

  const switchCompanion = () => {
    setPersona(null)
    clearPersona().catch(() => {})
  }

  const resetMemory = async () => {
    const r = await reset()
    setSessionId(r.session_id)
    setMessages([{ role: 'assistant', content: r.opening }])
    setFamiliarity(0)
  }

  const sendMessage = async (text: string, quote?: string) => {
    if (!sessionId || streaming) return
    lastActivityRef.current = Date.now()
    setMessages((prev) => [...prev, { role: 'user', content: text, quote }, { role: 'assistant', content: '' }])
    setPersona((p) => (p && p.hidden ? { ...p, turns: (p.turns ?? 0) + 1 } : p))
    setStreaming(true)
    let acc = ''
    try {
      await chatStream(sessionId, text, quote, (delta) => {
        acc += delta
        setMessages((prev) => {
          const next = [...prev]
          next[next.length - 1] = { role: 'assistant', content: acc }
          return next
        })
      })
      refreshFamiliarity()
    } catch (e) {
      const msg = e instanceof Error ? e.message : '出错了'
      setMessages((prev) => {
        const next = [...prev]
        next[next.length - 1] = { role: 'assistant', content: `（${msg}）` }
        return next
      })
    } finally {
      setStreaming(false)
    }
  }

  const doFlashback = async () => {
    if (!sessionId) return
    const r = await flashback(sessionId)
    setMessages((prev) => [...prev, { role: 'assistant', content: r.message }])
  }

  // 自动闪回：静默一段时间后主动关心一次（每会话最多一次）。
  // 需先相识到一定程度（用户至少发言 30 次）才会主动提起往事，避免刚认识就套近乎。
  useEffect(() => {
    if (!persona || !sessionId) return
    const t = setInterval(() => {
      if (autoFlashFiredRef.current) return
      const userTurns = messages.filter((m) => m.role === 'user').length
      if (userTurns < 30) return
      if (Date.now() - lastActivityRef.current > 30000) {
        autoFlashFiredRef.current = true
        void doFlashback()
      }
    }, 10000)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [persona, sessionId, messages])

  const value: AppState = {
    theme,
    setTheme,
    authenticated: token !== '',
    username,
    personas,
    persona,
    familiarity,
    messages,
    streaming,
    loading,
    milestone,
    revealScene,
    register,
    login,
    logout,
    selectCompanion,
    selectBlindBox,
    switchCompanion,
    resetMemory,
    sendMessage,
    refreshFamiliarity,
    refreshPersonas,
  }

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>
}

export function useApp(): AppState {
  const ctx = useContext(AppContext)
  if (!ctx) throw new Error('useApp must be used within AppProvider')
  return ctx
}
