import { create } from 'zustand'
import { UserInfo } from '../api/api'

interface AuthState {
  isAuthenticated: boolean
  user: UserInfo | null
  token: string | null
  
  login: (token: string, user: UserInfo) => void
  logout: () => void
  setUser: (user: UserInfo) => void
}

export const useAuthStore = create<AuthState>((set) => ({
  isAuthenticated: false,
  user: null,
  token: null,

  login: (token, user) => {
    localStorage.setItem('token', token)
    localStorage.setItem('user', JSON.stringify(user))
    set({ isAuthenticated: true, token, user })
  },

  logout: () => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    set({ isAuthenticated: false, token: null, user: null })
  },

  setUser: (user) => {
    localStorage.setItem('user', JSON.stringify(user))
    set({ user })
  },
}))

// Инициализация из localStorage
const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null
const userStr = typeof window !== 'undefined' ? localStorage.getItem('user') : null

if (token && userStr) {
  try {
    const user = JSON.parse(userStr) as UserInfo
    useAuthStore.setState({ isAuthenticated: true, token, user })
  } catch (e) {
    // Ошибка парсинга - очищаем localStorage
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }
}
