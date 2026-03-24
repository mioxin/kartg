import React, { useState, useRef, useEffect } from 'react'
import { useCartridgeStore } from '../store/cartridgeStore'

type OperationType = 'send' | 'receive' | 'retire'

interface CartridgeItem {
  id: string
  status?: string
  model?: string
}

export const OperationsPage: React.FC = () => {
  const { sendToRefill, receiveFromRefill, retireCartridge, addToast, getCartridge, registerCartridge } = useCartridgeStore()

  const [operationType, setOperationType] = useState<OperationType>('send')
  const [inputId, setInputId] = useState('')
  const [inputModel, setInputModel] = useState('')
  const [showModelInput, setShowModelInput] = useState(false)
  const [pendingId, setPendingId] = useState('')
  const [loading, setLoading] = useState(false)
  const [processingId, setProcessingId] = useState<string | null>(null)
  
  // Отдельные списки для каждой операции
  const [cartridgeLists, setCartridgeLists] = useState<Record<OperationType, CartridgeItem[]>>({
    send: [],
    receive: [],
    retire: [],
  })

  const inputRef = useRef<HTMLInputElement>(null)
  const modelInputRef = useRef<HTMLInputElement>(null)

  // Текущий список в зависимости от типа операции
  const currentList = cartridgeLists[operationType]
  
  // Установка текущего списка
  const setCurrentList = (list: CartridgeItem[]) => {
    setCartridgeLists(prev => ({
      ...prev,
      [operationType]: list,
    }))
  }

  // Автофокус на поле ввода при загрузке
  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  // Сброс временных состояний при смене типа операции (списки сохраняем!)
  useEffect(() => {
    setInputId('')
    setInputModel('')
    setShowModelInput(false)
    setPendingId('')
  }, [operationType])

  // Фокус на поле модели при показе
  useEffect(() => {
    if (showModelInput) {
      modelInputRef.current?.focus()
    }
  }, [showModelInput])

  // Проверка корректности операции для картриджа
  const checkOperationValidity = (cartridge: CartridgeItem, opType: OperationType): { valid: boolean; message?: string } => {
    const status = cartridge.status
    
    if (opType === 'send') {
      // Отправить на заправку можно только картридж со статусом "В использовании"
      if (status === 'CARTRIDGE_STATUS_REFILLING') {
        return { valid: false, message: `Картридж ${cartridge.id} уже на заправке` }
      }
      if (status === 'CARTRIDGE_STATUS_RETIRED') {
        return { valid: false, message: `Картридж ${cartridge.id} утилизирован` }
      }
    }
    
    if (opType === 'receive') {
      // Принять с заправки можно только картридж со статусом "На заправке"
      if (status !== 'CARTRIDGE_STATUS_REFILLING') {
        return { valid: false, message: `Картридж ${cartridge.id} не находится на заправке (статус: ${getStatusLabel(status)})` }
      }
    }
    
    if (opType === 'retire') {
      // Утилизировать можно только картридж со статусом "В использовании" или "На заправке"
      if (status === 'CARTRIDGE_STATUS_RETIRED') {
        return { valid: false, message: `Картридж ${cartridge.id} уже утилизирован` }
      }
    }
    
    return { valid: true }
  }

  // Добавление картриджа в список
  const handleAddCartridge = async () => {
    const id = inputId.trim().toUpperCase()
    
    if (!id) {
      addToast('Введите ID картриджа', 'error')
      return
    }

    // Проверяем, нет ли уже такого в текущем списке
    if (currentList.some(c => c.id === id)) {
      addToast(`Картридж ${id} уже в списке`, 'error')
      return
    }

    try {
      // Пробуем получить информацию о картридже
      const cartridge = await getCartridge(id)
      
      // Проверяем корректность операции для этого картриджа
      const validity = checkOperationValidity(cartridge, operationType)
      if (!validity.valid) {
        addToast(validity.message || 'Операция невозможна', 'error')
        return
      }
      
      setCurrentList([...currentList, { 
        id: cartridge.id, 
        status: cartridge.status,
        model: cartridge.model 
      }])
      setInputId('') // Очищаем поле ввода
      inputRef.current?.focus() // Возвращаем фокус
    } catch (err: any) {
      // Картридж не найден - предлагаем зарегистрировать
      setPendingId(id)
      setInputModel('')
      setShowModelInput(true)
    }
  }

  // Регистрация нового картриджа
  const handleRegisterCartridge = async () => {
    if (!inputModel.trim()) {
      addToast('Введите модель картриджа', 'error')
      return
    }

    try {
      const cartridge = await registerCartridge(pendingId, inputModel.trim())
      setCurrentList([...currentList, { 
        id: cartridge.id, 
        status: cartridge.status,
        model: cartridge.model 
      }])
      setShowModelInput(false)
      setPendingId('')
      setInputModel('')
      setInputId('')
      inputRef.current?.focus()
      addToast(`Картридж ${pendingId} зарегистрирован`, 'success')
    } catch (err: any) {
      // Ошибка уже обработана в store
    }
  }

  // Отмена регистрации
  const handleCancelRegister = () => {
    setShowModelInput(false)
    setPendingId('')
    setInputModel('')
    inputRef.current?.focus()
  }

  // Обработка Enter в поле ввода ID
  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleAddCartridge()
    }
  }

  // Обработка Enter в поле модели
  const handleModelKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleRegisterCartridge()
    }
  }

  // Удаление картриджа из списка
  const handleRemoveCartridge = (id: string) => {
    setCurrentList(currentList.filter(c => c.id !== id))
  }

  // Очистка текущего списка
  const handleClearList = () => {
    setCurrentList([])
  }

  // Выполнение операции для всех картриджей в списке
  const handleExecuteOperation = async () => {
    if (currentList.length === 0) {
      addToast('Добавьте хотя бы один картридж', 'error')
      return
    }

    // Сначала проверяем все картриджи
    const invalidCartridges: string[] = []
    for (const cartridge of currentList) {
      const validity = checkOperationValidity(cartridge, operationType)
      if (!validity.valid) {
        invalidCartridges.push(validity.message || cartridge.id)
      }
    }

    if (invalidCartridges.length > 0) {
      addToast(`Невозможно выполнить операцию:\n${invalidCartridges.join('\n')}`, 'error')
      return
    }

    setLoading(true)
    let successCount = 0
    let errorCount = 0
    const processedIds: string[] = []

    for (const cartridge of currentList) {
      setProcessingId(cartridge.id)
      try {
        // Получаем актуальный статус перед операцией
        const currentCartridge = await getCartridge(cartridge.id)
        const validity = checkOperationValidity(currentCartridge, operationType)
        if (!validity.valid) {
          addToast(validity.message || `Операция для ${cartridge.id} невозможна`, 'error')
          errorCount++
          continue
        }

        if (operationType === 'send') {
          await sendToRefill(cartridge.id, '')
        } else if (operationType === 'receive') {
          await receiveFromRefill(cartridge.id, '')
        } else {
          await retireCartridge(cartridge.id, '')
        }
        successCount++
        processedIds.push(cartridge.id)
      } catch (err: any) {
        errorCount++
      }
      setProcessingId(null)
    }

    // Удаляем обработанные картриджи из списка
    setCurrentList(currentList.filter(c => !processedIds.includes(c.id)))

    setLoading(false)
    
    if (successCount > 0) {
      addToast(`${currentOp.label}: выполнено ${successCount}, ошибок ${errorCount}`, errorCount > 0 ? 'error' : 'success')
    }
  }

  const operationLabels = {
    send: { label: 'На заправку', icon: '🔄', color: 'yellow', description: 'Отправка картриджей в сервисный центр' },
    receive: { label: 'С заправки', icon: '✅', color: 'green', description: 'Прием картриджей после заправки' },
    retire: { label: 'На утилизацию', icon: '🗑️', color: 'red', description: 'Списание картриджей' },
  }

  const currentOp = operationLabels[operationType]
  const colorClasses = {
    yellow: {
      border: 'border-yellow-500',
      ring: 'focus:ring-yellow-500',
      bg: 'bg-yellow-600',
      hover: 'hover:bg-yellow-700',
      light: 'bg-yellow-50',
      text: 'text-yellow-700',
    },
    green: {
      border: 'border-green-500',
      ring: 'focus:ring-green-500',
      bg: 'bg-green-600',
      hover: 'hover:bg-green-700',
      light: 'bg-green-50',
      text: 'text-green-700',
    },
    red: {
      border: 'border-red-500',
      ring: 'focus:ring-red-500',
      bg: 'bg-red-600',
      hover: 'hover:bg-red-700',
      light: 'bg-red-50',
      text: 'text-red-700',
    },
  }

  const currentColors = colorClasses[currentOp.color as keyof typeof colorClasses]

  const getStatusLabel = (status: string | undefined) => {
    if (!status) return ''
    switch (status) {
      case 'CARTRIDGE_STATUS_IN_USE': return 'В использовании'
      case 'CARTRIDGE_STATUS_REFILLING': return 'На заправке'
      case 'CARTRIDGE_STATUS_RETIRED': return 'Утилизирован'
      default: return status
    }
  }

  const getStatusColor = (status: string | undefined) => {
    if (!status) return 'bg-gray-100 text-gray-800'
    switch (status) {
      case 'CARTRIDGE_STATUS_IN_USE': return 'bg-green-100 text-green-800'
      case 'CARTRIDGE_STATUS_REFILLING': return 'bg-yellow-100 text-yellow-800'
      case 'CARTRIDGE_STATUS_RETIRED': return 'bg-red-100 text-red-800'
      default: return 'bg-gray-100 text-gray-800'
    }
  }

  return (
    <div className="px-4 py-6 sm:px-0">
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Операции</h2>

      <div className="max-w-3xl mx-auto">
        <div className="bg-white shadow rounded-lg overflow-hidden">
          {/* Выбор типа операции */}
          <div className="grid grid-cols-3 border-b border-gray-200">
            {Object.entries(operationLabels).map(([key, op]) => (
              <button
                key={key}
                onClick={() => setOperationType(key as OperationType)}
                className={`py-4 px-2 text-center transition-colors ${
                  operationType === key
                    ? `${currentColors.light} border-b-2 border-${op.color}-500 ${currentColors.text}`
                    : 'text-gray-600 hover:bg-gray-50'
                }`}
              >
                <span className="text-xl block mb-1">{op.icon}</span>
                <span className="text-sm font-medium">{op.label}</span>
              </button>
            ))}
          </div>

          {/* Форма */}
          <div className="p-6">
            <div className="mb-4">
              <p className="text-sm text-gray-600">{currentOp.description}</p>
            </div>

            {/* Поле ввода ID */}
            <div className="mb-4">
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Введите ID картриджа
              </label>
              <div className="flex space-x-2">
                <input
                  ref={inputRef}
                  type="text"
                  value={inputId}
                  onChange={(e) => setInputId(e.target.value)}
                  onKeyPress={handleKeyPress}
                  placeholder="Например: CART-001"
                  className={`flex-1 px-4 py-3 border border-gray-300 rounded-md focus:outline-none focus:ring-2 ${currentColors.ring} uppercase`}
                  autoComplete="off"
                  disabled={loading || showModelInput}
                />
                <button
                  type="button"
                  onClick={handleAddCartridge}
                  disabled={loading || !inputId.trim() || showModelInput}
                  className={`px-6 py-3 ${currentColors.bg} text-white rounded-md ${currentColors.hover} font-medium disabled:opacity-50 transition-colors`}
                >
                  Добавить
                </button>
              </div>
              <p className="mt-1 text-xs text-gray-500">Нажмите Enter или кнопку "Добавить"</p>
            </div>

            {/* Форма регистрации нового картриджа */}
            {showModelInput && (
              <div className={`mb-4 p-4 border-2 border-${currentOp.color}-300 ${currentColors.light} rounded-md`}>
                <div className="flex items-start mb-3">
                  <span className="text-xl mr-2">ℹ️</span>
                  <div>
                    <p className="text-sm font-medium text-gray-900">Картридж не найден</p>
                    <p className="text-xs text-gray-600">Зарегистрировать новый картридж?</p>
                  </div>
                </div>
                <div className="flex items-center space-x-2 mb-2">
                  <span className="text-sm font-medium text-gray-700">ID:</span>
                  <span className={`px-3 py-1 bg-white border border-gray-300 rounded-md font-mono text-sm`}>{pendingId}</span>
                </div>
                <div className="flex space-x-2">
                  <input
                    ref={modelInputRef}
                    type="text"
                    value={inputModel}
                    onChange={(e) => setInputModel(e.target.value)}
                    onKeyPress={handleModelKeyPress}
                    placeholder="Модель картриджа (например: HP 12A)"
                    className={`flex-1 px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 ${currentColors.ring} text-sm`}
                    autoComplete="off"
                  />
                  <button
                    type="button"
                    onClick={handleRegisterCartridge}
                    className={`px-4 py-2 ${currentColors.bg} text-white rounded-md ${currentColors.hover} text-sm font-medium transition-colors`}
                  >
                    Зарегистрировать
                  </button>
                  <button
                    type="button"
                    onClick={handleCancelRegister}
                    className="px-4 py-2 bg-gray-200 text-gray-700 rounded-md hover:bg-gray-300 text-sm font-medium transition-colors"
                  >
                    Отмена
                  </button>
                </div>
              </div>
            )}

            {/* Список картриджей */}
            <div className="mb-6">
              <div className="flex justify-between items-center mb-2">
                <label className="block text-sm font-medium text-gray-700">
                  Картриджи для операции ({currentList.length})
                </label>
                {currentList.length > 0 && (
                  <button
                    type="button"
                    onClick={handleClearList}
                    className="text-xs text-red-600 hover:text-red-800 font-medium flex items-center"
                    disabled={loading}
                  >
                    <span className="mr-1">🗑️</span>
                    Очистить список
                  </button>
                )}
              </div>
              
              <div className={`border border-gray-200 rounded-md min-h-[200px] max-h-[400px] overflow-y-auto ${currentColors.light}`}>
                {currentList.length === 0 ? (
                  <div className="flex items-center justify-center h-[200px] text-gray-400">
                    <div className="text-center">
                      <span className="text-4xl block mb-2">📦</span>
                      <p>Список пуст</p>
                      <p className="text-sm">Добавьте картриджи для операции</p>
                    </div>
                  </div>
                ) : (
                  <ul className="divide-y divide-gray-200">
                    {currentList.map((cartridge) => (
                      <li
                        key={cartridge.id}
                        className={`flex items-center justify-between p-3 hover:bg-white transition-colors ${
                          processingId === cartridge.id ? 'bg-white animate-pulse' : ''
                        }`}
                      >
                        <div className="flex items-center space-x-3">
                          <span className="text-lg">{currentOp.icon}</span>
                          <div>
                            <p className="font-medium text-gray-900">{cartridge.id}</p>
                            <p className="text-xs text-gray-500">{cartridge.model || 'Модель не указана'}</p>
                          </div>
                        </div>
                        <div className="flex items-center space-x-2">
                          <span className={`px-2 py-1 text-xs font-medium rounded-full ${getStatusLabel(cartridge.status) ? getStatusColor(cartridge.status) : 'bg-gray-100 text-gray-800'}`}>
                            {getStatusLabel(cartridge.status) || 'Неизвестно'}
                          </span>
                          <button
                            type="button"
                            onClick={() => handleRemoveCartridge(cartridge.id)}
                            className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                            disabled={loading || processingId === cartridge.id}
                          >
                            ✕
                          </button>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>

            {/* Кнопка выполнения */}
            <button
              type="button"
              onClick={handleExecuteOperation}
              disabled={loading || currentList.length === 0}
              className={`w-full py-3 px-4 ${currentColors.bg} text-white rounded-md ${currentColors.hover} font-medium disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center justify-center`}
            >
              {loading ? (
                <span className="flex items-center">
                  <svg className="animate-spin -ml-1 mr-2 h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  Обработка...
                </span>
              ) : (
                <span className="flex items-center">
                  <span className="mr-2">{currentOp.icon}</span>
                  {operationType === 'send' && `Выдать на заправку (${currentList.length})`}
                  {operationType === 'receive' && `Принять с заправки (${currentList.length})`}
                  {operationType === 'retire' && `Списать (${currentList.length})`}
                </span>
              )}
            </button>
          </div>
        </div>

        {/* Подсказка */}
        <div className="mt-6 bg-blue-50 border border-blue-200 rounded-lg p-4">
          <h4 className="text-sm font-medium text-blue-900 mb-2">💡 Подсказка</h4>
          <ul className="text-sm text-blue-700 space-y-1">
            <li>• Вводите ID картриджей по одному и нажимайте Enter для добавления в список</li>
            <li>• Можно добавить несколько картриджей для массовой операции</li>
            <li>• При приеме с заправки счетчик заправок увеличивается автоматически</li>
          </ul>
        </div>
      </div>
    </div>
  )
}
