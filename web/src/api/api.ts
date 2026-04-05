import axios from 'axios'

const API_BASE_URL = '/api/v1'

export const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor для добавления токена в Authorization header
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Карtridge API
export interface Cartridge {
  id: string
  model: string
  status: 'CARTRIDGE_STATUS_IN_USE' | 'CARTRIDGE_STATUS_REFILLING' | 'CARTRIDGE_STATUS_RETIRED'
  totalRefills: number
  createdAt: string
  retiredAt?: string
}

export interface ListCartridgesResponse {
  cartridges: Cartridge[]
  totalCount: number
}

export const cartridgeApi = {
  list: async (page = 1, pageSize = 50, search = '', status = ''): Promise<ListCartridgesResponse> => {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
      ...(search && { search }),
      ...(status && { status }),
    })
    const response = await api.get(`/cartridges?${params}`)
    return response.data
  },

  get: async (id: string): Promise<Cartridge> => {
    const response = await api.get(`/cartridges/${id}`)
    return response.data
  },

  register: async (id: string, model: string): Promise<Cartridge> => {
    const response = await api.post('/cartridges', { id, model })
    return response.data
  },
}

// Operations API
export const operationApi = {
  sendToRefill: async (cartridgeId: string, comment = ''): Promise<Cartridge> => {
    const response = await api.post('/operations/send-to-refill', { cartridge_id: cartridgeId, comment })
    return response.data
  },

  receiveFromRefill: async (cartridgeId: string, comment = ''): Promise<Cartridge> => {
    const response = await api.post('/operations/receive-from-refill', { cartridge_id: cartridgeId, comment })
    return response.data
  },

  retire: async (cartridgeId: string, comment = ''): Promise<Cartridge> => {
    const response = await api.post('/operations/retire', { cartridge_id: cartridgeId, comment })
    return response.data
  },

  getHistory: async (cartridgeId: string) => {
    const response = await api.get(`/cartridges/${cartridgeId}/history`)
    return response.data
  },

  generateAct: async (cartridgeIds: string[] = []): Promise<string> => {
    const response = await api.post('/operations/generate-act', { cartridge_ids: cartridgeIds })
    return response.data as string
  },
}

// Analytics API
export interface GlobalStats {
  totalCartridges: number
  inUse: number
  refilling: number
  retired: number
  totalRefillsAllTime: number
}

export interface RefillsStats {
  totalRefills: number
  uniqueCartridges: number
}

export const analyticsApi = {
  getRefillsStats: async (periodStart: string, periodEnd: string): Promise<RefillsStats> => {
    const response = await api.get('/analytics/refills-stats', {
      params: { period_start: periodStart, period_end: periodEnd },
    })
    return response.data
  },

  getGlobalStats: async (): Promise<GlobalStats> => {
    const response = await api.get('/analytics/global-stats')
    return response.data
  },

  // Экспорт статистики заправок
  exportRefillsStats: async (periodStart: string, periodEnd: string, format: 'csv' | 'txt' = 'csv') => {
    const params = new URLSearchParams({
      period_start: periodStart,
      period_end: periodEnd,
      format,
    })
    const response = await api.get(`/export/refills?${params}`, {
      responseType: 'blob',
    })
    return response.data as Blob
  },

  // Экспорт истории картриджа
  exportCartridgeHistory: async (cartridgeId: string, format: 'csv' | 'txt' = 'csv') => {
    const params = new URLSearchParams({ format })
    const response = await api.get(`/export/cartridge/${cartridgeId}/history?${params}`, {
      responseType: 'blob',
    })
    return response.data as Blob
  },
}

// Health check
export const healthApi = {
  check: async () => {
    const response = await api.get('/health')
    return response.data
  },
}

// Утилита для скачивания файлов
export const downloadFile = (blob: Blob, filename: string) => {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

// Auth API
export interface UserInfo {
  id: number
  username: string
  fullName: string
  role: string
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  expiresIn: number
  user: UserInfo
}

export const authApi = {
  login: async (username: string, password: string): Promise<LoginResponse> => {
    const response = await api.post('/auth/login', { username, password })
    return response.data
  },

  register: async (username: string, password: string, fullName: string, role?: string) => {
    const response = await api.post('/auth/register', { username, password, full_name: fullName, role })
    return response.data
  },

  getCurrentUser: async (): Promise<UserInfo> => {
    const response = await api.get('/auth/me')
    return response.data
  },

  changePassword: async (oldPassword: string, newPassword: string) => {
    const response = await api.post('/auth/change-password', { old_password: oldPassword, new_password: newPassword })
    return response.data
  },

  logout: () => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  },
}

// Models API (справочник моделей картриджей)
export interface ModelItem {
  id: number
  name: string
  usageCount: number
  lastUsedAt: string
  createdAt: string
}

export interface ListModelsResponse {
  models: ModelItem[]
  totalCount: number
}

export const modelApi = {
  list: async (page = 1, pageSize = 50, search = ''): Promise<ListModelsResponse> => {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
      ...(search && { search }),
    })
    const response = await api.get(`/models?${params}`)
    return response.data
  },

  upsert: async (name: string): Promise<ModelItem> => {
    const response = await api.post('/models', { name })
    return response.data
  },
}
