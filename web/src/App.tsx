import { BrowserRouter, Routes, Route, Link, Navigate, useNavigate } from 'react-router-dom'
import { CartridgesPage } from './pages/CartridgesPage'
import { OperationsPage } from './pages/OperationsPage'
import { AnalyticsPage } from './pages/AnalyticsPage'
import { LoginPage } from './pages/LoginPage'
import { ToastContainer } from './components/ToastContainer'
import { useCartridgeStore } from './store/cartridgeStore'
import { useAuthStore } from './store/authStore'

// Компонент для защиты роутов
interface ProtectedRouteProps {
  children: React.ReactNode
}

const ProtectedRoute: React.FC<ProtectedRouteProps> = ({ children }) => {
  const { isAuthenticated } = useAuthStore()
  
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  
  return <>{children}</>
}

// Компонент навигации с logout
const AppNav: React.FC = () => {
  const navigate = useNavigate()
  const { isAuthenticated, user, logout } = useAuthStore()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <nav className="bg-white shadow">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between h-16">
          <div className="flex items-center">
            <Link to="/" className="text-xl font-bold text-gray-800">
              kartg 🖨️
            </Link>
            {isAuthenticated && (
              <div className="ml-10 flex items-baseline space-x-4">
                <Link to="/cartridges" className="text-gray-600 hover:text-gray-900 px-3 py-2 rounded-md text-sm font-medium">
                  Картриджи
                </Link>
                <Link to="/operations" className="text-gray-600 hover:text-gray-900 px-3 py-2 rounded-md text-sm font-medium">
                  Операции
                </Link>
                <Link to="/analytics" className="text-gray-600 hover:text-gray-900 px-3 py-2 rounded-md text-sm font-medium">
                  Аналитика
                </Link>
              </div>
            )}
          </div>
          <div className="flex items-center space-x-4">
            {isAuthenticated && user && (
              <>
                <span className="text-sm text-gray-600">
                  👤 {user.fullName || user.username}
                </span>
                <button
                  onClick={handleLogout}
                  className="text-sm text-gray-600 hover:text-gray-900"
                >
                  Выход
                </button>
              </>
            )}
          </div>
        </div>
      </div>
    </nav>
  )
}

function App() {
  const { toasts, removeToast } = useCartridgeStore()

  return (
    <BrowserRouter>
      <div className="min-h-screen bg-gray-100">
        <AppNav />

        <main className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/" element={
              <ProtectedRoute>
                <HomePage />
              </ProtectedRoute>
            } />
            <Route path="/cartridges" element={
              <ProtectedRoute>
                <CartridgesPage />
              </ProtectedRoute>
            } />
            <Route path="/operations" element={
              <ProtectedRoute>
                <OperationsPage />
              </ProtectedRoute>
            } />
            <Route path="/analytics" element={
              <ProtectedRoute>
                <AnalyticsPage />
              </ProtectedRoute>
            } />
          </Routes>
        </main>

        <ToastContainer toasts={toasts} removeToast={removeToast} />
      </div>
    </BrowserRouter>
  )
}

function HomePage() {
  return (
    <div className="px-4 py-6 sm:px-0">
      <h1 className="text-3xl font-bold text-gray-900 mb-6">Добро пожаловать в kartg</h1>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <a href="/cartridges" className="block bg-white overflow-hidden shadow rounded-lg hover:shadow-md transition-shadow">
          <div className="px-4 py-5 sm:p-6">
            <h3 className="text-lg font-medium text-gray-900">📦 Картриджи</h3>
            <p className="mt-2 text-sm text-gray-500">
              Управление реестром картриджей: регистрация, поиск, просмотр статуса
            </p>
          </div>
        </a>
        <a href="/operations" className="block bg-white overflow-hidden shadow rounded-lg hover:shadow-md transition-shadow">
          <div className="px-4 py-5 sm:p-6">
            <h3 className="text-lg font-medium text-gray-900">🔄 Операции</h3>
            <p className="mt-2 text-sm text-gray-500">
              Отправка на заправку, прием с заправки, утилизация
            </p>
          </div>
        </a>
        <a href="/analytics" className="block bg-white overflow-hidden shadow rounded-lg hover:shadow-md transition-shadow">
          <div className="px-4 py-5 sm:p-6">
            <h3 className="text-lg font-medium text-gray-900">📊 Аналитика</h3>
            <p className="mt-2 text-sm text-gray-500">
              Статистика заправок, отчеты, экспорт данных
            </p>
          </div>
        </a>
      </div>
    </div>
  )
}

export default App
