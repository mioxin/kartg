import React, { useState } from 'react'

export interface CartridgeToRegister {
  id: string
  model: string
  isNew?: boolean
}

interface BulkRegisterModalProps {
  cartridgesToRegister: CartridgeToRegister[]
  materialStyles?: any
  onClose: () => void
  onRegisterAll: (cartridges: CartridgeToRegister[]) => Promise<void>
  isLoading?: boolean
}

export const BulkRegisterModal: React.FC<BulkRegisterModalProps> = ({
  cartridgesToRegister,
  onClose,
  onRegisterAll,
  isLoading = false,
}) => {
  const [cartridgeList, setCartridgeList] = useState<CartridgeToRegister[]>(cartridgesToRegister)

  const handleUpdateCartridge = (id: string, model: string) => {
    setCartridgeList(
      cartridgeList.map(c =>
        c.id === id ? { ...c, model } : c
      )
    )
  }

  const isValid = cartridgeList.every(c => c.model.trim() !== '')

  const handleRegister = async () => {
    try {
      await onRegisterAll(cartridgeList)
    } catch (error) {
      console.error('Error registering cartridges:', error)
    }
  }

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* Overlay */}
      <div className="fixed inset-0 bg-black bg-opacity-50 transition-opacity"></div>

      {/* Modal */}
      <div className="flex items-center justify-center min-h-screen px-4 pt-4 pb-20 text-center sm:p-0">
        <div className="relative inline-block align-bottom bg-white rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:w-full sm:max-w-2xl">
          {/* Header */}
          <div className="bg-white px-6 py-4 border-b border-gray-200">
            <div className="flex items-center">
              <span className="text-2xl mr-3">📋</span>
              <div>
                <h3 className="text-lg font-medium text-gray-900">Регистрация картриджей</h3>
                <p className="text-sm text-gray-500">Для каждого картриджа укажите тип</p>
              </div>
            </div>
          </div>

          {/* Content */}
          <div className="bg-white px-6 py-4 max-h-96 overflow-y-auto">
            <div className="space-y-3">
              {cartridgeList.map((cartridge) => (
                <div key={cartridge.id} className="border border-gray-200 rounded-md p-3 bg-gray-50">
                  <div className="mb-2">
                    <div className="flex items-center justify-between">
                      <p className="text-sm font-medium text-gray-900">{cartridge.id}</p>
                      {cartridge.isNew && (
                        <span className="text-xs bg-orange-100 text-orange-700 px-2 py-1 rounded">Новый</span>
                      )}
                    </div>
                  </div>

                  <input
                    type="text"
                    value={cartridge.model}
                    onChange={(e) => handleUpdateCartridge(cartridge.id, e.target.value)}
                    placeholder="Введите тип картриджа"
                    className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent uppercase"
                    autoComplete="off"
                    disabled={isLoading}
                  />
                </div>
              ))}
            </div>
          </div>

          {/* Footer */}
          <div className="bg-gray-50 px-6 py-4 border-t border-gray-200 flex justify-end space-x-3">
            <button
              type="button"
              onClick={onClose}
              disabled={isLoading}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50"
            >
              Отмена
            </button>
            <button
              type="button"
              onClick={handleRegister}
              disabled={isLoading || !isValid}
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 disabled:opacity-50"
            >
              {isLoading ? 'Регистрация...' : `Зарегистрировать (${cartridgeList.length})`}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default BulkRegisterModal
