import { create } from 'zustand'
import { Cartridge, cartridgeApi, operationApi, analyticsApi } from '../api/api'

export interface Toast {
  id: string
  message: string
  type: 'success' | 'error' | 'info'
}

interface CartridgeState {
  // Картриджи
  cartridges: Cartridge[]
  totalCount: number
  loading: boolean
  error: string | null

  // Уведомления
  toasts: Toast[]

  // Статистика
  globalStats: {
    totalCartridges: number
    inUse: number
    refilling: number
    retired: number
    totalRefillsAllTime: number
  } | null

  // Действия
  fetchCartridges: (page: number, pageSize: number, search: string, status: string) => Promise<void>
  getCartridge: (id: string) => Promise<Cartridge>
  registerCartridge: (id: string, model: string) => Promise<Cartridge>
  sendToRefill: (cartridgeId: string, comment: string) => Promise<Cartridge>
  receiveFromRefill: (cartridgeId: string, comment: string) => Promise<Cartridge>
  retireCartridge: (cartridgeId: string, comment: string) => Promise<Cartridge>
  getHistory: (cartridgeId: string) => Promise<any>
  fetchGlobalStats: () => Promise<void>
  
  // Уведомления
  addToast: (message: string, type: Toast['type']) => void
  removeToast: (id: string) => void
  clearError: () => void
}

export const useCartridgeStore = create<CartridgeState>((set, get) => ({
  cartridges: [],
  totalCount: 0,
  loading: false,
  error: null,
  toasts: [],
  globalStats: null,

  // Добавление уведомления
  addToast: (message, type) => {
    const id = Math.random().toString(36).substring(2, 9)
    const toast: Toast = { id, message, type }
    set(state => ({ toasts: [...state.toasts, toast] }))
    // Автоматическое удаление через 5 секунд
    setTimeout(() => {
      get().removeToast(id)
    }, 5000)
  },

  // Удаление уведомления
  removeToast: (id) => {
    set(state => ({ toasts: state.toasts.filter(t => t.id !== id) }))
  },

  fetchCartridges: async (page, pageSize, search, status) => {
    set({ loading: true, error: null })
    try {
      const data = await cartridgeApi.list(page, pageSize, search, status)
      set({ cartridges: data.cartridges, totalCount: data.totalCount, loading: false })
    } catch (err: any) {
      set({ error: err.message || 'Ошибка при загрузке картриджей', loading: false })
      get().addToast(err.message || 'Ошибка при загрузке картриджей', 'error')
    }
  },

  getCartridge: async (id) => {
    return await cartridgeApi.get(id)
  },

  registerCartridge: async (id, model) => {
    set({ loading: true, error: null })
    try {
      const cartridge = await cartridgeApi.register(id, model)
      set({ loading: false })
      get().addToast(`Картридж ${id} успешно зарегистрирован`, 'success')
      // Обновляем список
      get().fetchCartridges(1, 50, '', '')
      return cartridge
    } catch (err: any) {
      const message = err.message || 'Ошибка при регистрации'
      set({ error: message, loading: false })
      get().addToast(message, 'error')
      throw err
    }
  },

  sendToRefill: async (cartridgeId, comment) => {
    set({ loading: true, error: null })
    try {
      const cartridge = await operationApi.sendToRefill(cartridgeId, comment)
      set({ loading: false })
      get().addToast(`Картридж ${cartridgeId} отправлен на заправку`, 'success')
      get().fetchCartridges(1, 50, '', '')
      return cartridge
    } catch (err: any) {
      const message = err.message || 'Ошибка при отправке на заправку'
      set({ error: message, loading: false })
      get().addToast(message, 'error')
      throw err
    }
  },

  receiveFromRefill: async (cartridgeId, comment) => {
    set({ loading: true, error: null })
    try {
      const cartridge = await operationApi.receiveFromRefill(cartridgeId, comment)
      set({ loading: false })
      get().addToast(`Картридж ${cartridgeId} принят с заправки (всего заправок: ${cartridge.totalRefills})`, 'success')
      get().fetchCartridges(1, 50, '', '')
      return cartridge
    } catch (err: any) {
      const message = err.message || 'Ошибка при приеме с заправки'
      set({ error: message, loading: false })
      get().addToast(message, 'error')
      throw err
    }
  },

  retireCartridge: async (cartridgeId, comment) => {
    set({ loading: true, error: null })
    try {
      const cartridge = await operationApi.retire(cartridgeId, comment)
      set({ loading: false })
      get().addToast(`Картридж ${cartridgeId} утилизирован`, 'success')
      get().fetchCartridges(1, 50, '', '')
      return cartridge
    } catch (err: any) {
      const message = err.message || 'Ошибка при утилизации'
      set({ error: message, loading: false })
      get().addToast(message, 'error')
      throw err
    }
  },

  getHistory: async (cartridgeId) => {
    return await operationApi.getHistory(cartridgeId)
  },

  fetchGlobalStats: async () => {
    try {
      const stats = await analyticsApi.getGlobalStats()
      set({ globalStats: stats })
    } catch (err: any) {
      console.error('Ошибка при загрузке статистики:', err)
    }
  },

  clearError: () => set({ error: null }),
}))
