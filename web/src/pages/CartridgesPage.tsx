import React, { useEffect, useState, useRef } from 'react'
import { useCartridgeStore } from '../store/cartridgeStore'
import { CartridgeTable } from '../components/CartridgeTable'
import { Modal } from '../components/Modal'
import { RegisterForm } from '../components/RegisterForm'
import { OperationForm } from '../components/OperationForm'
import { HistoryModal } from '../components/HistoryModal'

export const CartridgesPage: React.FC = () => {
  const {
    cartridges,
    totalCount,
    loading,
    error,
    fetchCartridges,
    registerCartridge,
    sendToRefill,
    receiveFromRefill,
    retireCartridge,
    getHistory,
    clearError,
  } = useCartridgeStore()

  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [showRegisterModal, setShowRegisterModal] = useState(false)
  const [showOperationModal, setShowOperationModal] = useState(false)
  const [showHistoryModal, setShowHistoryModal] = useState(false)
  const [selectedCartridgeId, setSelectedCartridgeId] = useState('')
  const [operationType, setOperationType] = useState<'send' | 'receive' | 'retire'>('send')
  const [history, setHistory] = useState<any[]>([])

  const searchInputRef = useRef<HTMLInputElement>(null)
  const pageSize = 20

  // Авто-фокус на поле поиска для сканера штрих-кодов
  useEffect(() => {
    searchInputRef.current?.focus()
  }, [])

  useEffect(() => {
    fetchCartridges(page, pageSize, search, statusFilter)
  }, [page, search, statusFilter])

  useEffect(() => {
    if (error) {
      const timer = setTimeout(() => clearError(), 5000)
      return () => clearTimeout(timer)
    }
  }, [error])

  const handleRegister = async (id: string, model: string) => {
    try {
      await registerCartridge(id, model)
      setShowRegisterModal(false)
    } catch (err) {
      // Ошибка уже обработана в store
    }
  }

  const handleSendToRefill = (id: string) => {
    setSelectedCartridgeId(id)
    setOperationType('send')
    setShowOperationModal(true)
  }

  const handleReceiveFromRefill = (id: string) => {
    setSelectedCartridgeId(id)
    setOperationType('receive')
    setShowOperationModal(true)
  }

  const handleRetire = (id: string) => {
    setSelectedCartridgeId(id)
    setOperationType('retire')
    setShowOperationModal(true)
  }

  const handleOperationSubmit = async (cartridgeId: string, comment: string) => {
    try {
      if (operationType === 'send') {
        await sendToRefill(cartridgeId, comment)
      } else if (operationType === 'receive') {
        await receiveFromRefill(cartridgeId, comment)
      } else {
        await retireCartridge(cartridgeId, comment)
      }
      setShowOperationModal(false)
    } catch (err) {
      // Ошибка уже обработана в store
    }
  }

  const handleViewHistory = async (id: string) => {
    try {
      const data = await getHistory(id)
      setHistory(data.transactions || [])
      setSelectedCartridgeId(id)
      setShowHistoryModal(true)
    } catch (err) {
      console.error('Ошибка при загрузке истории:', err)
    }
  }

  const totalPages = Math.ceil(totalCount / pageSize)

  return (
    <div className="px-4 py-6 sm:px-0">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-2xl font-bold text-gray-900">Картриджи</h2>
        <button
          onClick={() => setShowRegisterModal(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 font-medium"
        >
          + Зарегистрировать
        </button>
      </div>

      {error && (
        <div className="mb-4 p-4 bg-red-50 border border-red-200 rounded-md">
          <p className="text-red-800">{error}</p>
        </div>
      )}

      <div className="bg-white shadow rounded-lg">
        <div className="p-4 border-b space-y-4">
          <div className="flex flex-col sm:flex-row sm:items-center sm:space-x-4 space-y-2 sm:space-y-0">
            <input
              ref={searchInputRef}
              type="text"
              placeholder="Поиск по ID или модели... (Ctrl+K для фокуса)"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value)
                setPage(1)
              }}
              className="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              onKeyDown={(e) => {
                // Поддержка Ctrl+K для фокуса
                if (e.ctrlKey && e.key === 'k') {
                  e.preventDefault()
                  searchInputRef.current?.focus()
                }
              }}
            />
            <select
              value={statusFilter}
              onChange={(e) => {
                setStatusFilter(e.target.value)
                setPage(1)
              }}
              className="px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="">Все статусы</option>
              <option value="CARTRIDGE_STATUS_IN_USE">В использовании</option>
              <option value="CARTRIDGE_STATUS_REFILLING">На заправке</option>
              <option value="CARTRIDGE_STATUS_RETIRED">Утилизирован</option>
            </select>
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-center">
            <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          </div>
        ) : (
          <CartridgeTable
            cartridges={cartridges}
            onSendToRefill={handleSendToRefill}
            onReceiveFromRefill={handleReceiveFromRefill}
            onRetire={handleRetire}
            onViewHistory={handleViewHistory}
          />
        )}

        <div className="p-4 border-t flex justify-between items-center">
          <p className="text-sm text-gray-600">
            Показано {cartridges.length} из {totalCount}
          </p>
          <div className="flex space-x-2">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-3 py-1 border border-gray-300 rounded-md disabled:opacity-50 hover:bg-gray-50"
            >
              ← Назад
            </button>
            <span className="px-3 py-1 text-gray-600">
              Страница {page} из {totalPages || 1}
            </span>
            <button
              onClick={() => setPage(p => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="px-3 py-1 border border-gray-300 rounded-md disabled:opacity-50 hover:bg-gray-50"
            >
              Вперед →
            </button>
          </div>
        </div>
      </div>

      <Modal
        isOpen={showRegisterModal}
        onClose={() => setShowRegisterModal(false)}
        title="Регистрация картриджа"
      >
        <RegisterForm
          onSubmit={handleRegister}
          onCancel={() => setShowRegisterModal(false)}
        />
      </Modal>

      <Modal
        isOpen={showOperationModal}
        onClose={() => setShowOperationModal(false)}
        title={
          operationType === 'send' ? 'Отправить на заправку' :
          operationType === 'receive' ? 'Принять с заправки' :
          'Утилизировать картридж'
        }
      >
        <OperationForm
          cartridgeId={selectedCartridgeId}
          operationType={operationType}
          onSubmit={handleOperationSubmit}
          onCancel={() => setShowOperationModal(false)}
        />
      </Modal>

      <Modal
        isOpen={showHistoryModal}
        onClose={() => setShowHistoryModal(false)}
        title="История операций"
      >
        <HistoryModal
          cartridgeId={selectedCartridgeId}
          transactions={history}
          onClose={() => setShowHistoryModal(false)}
        />
      </Modal>
    </div>
  )
}
