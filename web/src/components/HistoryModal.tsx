import React from 'react'

interface Transaction {
  id: string
  cartridgeId: string
  type: string
  timestamp: string
  comment: string
}

interface HistoryModalProps {
  cartridgeId: string
  transactions: Transaction[]
  onClose: () => void
}

const typeLabels: Record<string, { label: string; icon: string }> = {
  OPERATION_TYPE_REGISTRATION: { label: 'Регистрация', icon: '📝' },
  OPERATION_TYPE_SEND_TO_REFILL: { label: 'Отправка на заправку', icon: '🔄' },
  OPERATION_TYPE_RECEIVE_FROM_REFILL: { label: 'Прием с заправки', icon: '✅' },
  OPERATION_TYPE_RETIREMENT: { label: 'Утилизация', icon: '🗑️' },
}

export const HistoryModal: React.FC<HistoryModalProps> = ({
  cartridgeId,
  transactions,
  onClose,
}) => {
  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="flex items-center justify-center min-h-screen px-4 pt-4 pb-20 text-center sm:block sm:p-0">
        <div className="fixed inset-0 transition-opacity bg-gray-500 bg-opacity-75" onClick={onClose}></div>

        <div className="inline-block w-full max-w-2xl my-8 overflow-hidden text-left align-middle transition-all transform bg-white shadow-xl rounded-lg">
          <div className="flex justify-between items-center px-6 py-4 border-b">
            <h3 className="text-lg font-medium text-gray-900">
              История картриджа: {cartridgeId}
            </h3>
            <button
              onClick={onClose}
              className="text-gray-400 hover:text-gray-500 focus:outline-none"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <div className="px-6 py-4 max-h-96 overflow-y-auto">
            {transactions.length === 0 ? (
              <p className="text-gray-500 text-center py-4">История пуста</p>
            ) : (
              <div className="space-y-3">
                {transactions.map((tx) => {
                  const config = typeLabels[tx.type] || { label: tx.type, icon: '📄' }
                  const date = new Date(tx.timestamp)
                  return (
                    <div
                      key={tx.id}
                      className="flex items-start space-x-3 p-3 bg-gray-50 rounded-md"
                    >
                      <span className="text-xl">{config.icon}</span>
                      <div className="flex-1">
                        <div className="flex justify-between">
                          <p className="font-medium text-gray-900">{config.label}</p>
                          <p className="text-sm text-gray-500">
                            {date.toLocaleDateString('ru-RU')} {date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}
                          </p>
                        </div>
                        {tx.comment && (
                          <p className="text-sm text-gray-600 mt-1">{tx.comment}</p>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          <div className="px-6 py-4 border-t bg-gray-50 flex justify-end">
            <button
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
            >
              Закрыть
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
