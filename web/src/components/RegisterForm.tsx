import React, { useState, useEffect, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { modelApi, ModelItem } from '../api/api'

interface RegisterFormProps {
  onSubmit: (id: string, model: string) => Promise<void>
  onCancel: () => void
}

interface FormData {
  id: string
  model: string
}

export const RegisterForm: React.FC<RegisterFormProps> = ({ onSubmit, onCancel }) => {
  const { register, handleSubmit, formState: { errors, isSubmitting }, setValue, watch } = useForm<FormData>({
    defaultValues: {
      id: '',
      model: '',
    },
  })

  const [models, setModels] = useState<ModelItem[]>([])
  const [showSuggestions, setShowSuggestions] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const modelInputRef = useRef<HTMLInputElement | null>(null)
  const suggestionsRef = useRef<HTMLUListElement>(null)
  const watchedModel = watch('model')

  // Загрузка моделей при монтировании
  useEffect(() => {
    loadModels()
  }, [])

  // Закрытие подсказок при клике вне
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        suggestionsRef.current &&
        !suggestionsRef.current.contains(event.target as Node) &&
        modelInputRef.current &&
        !modelInputRef.current.contains(event.target as Node)
      ) {
        setShowSuggestions(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const loadModels = async () => {
    try {
      const response = await modelApi.list(1, 100)
      setModels(response.models)
    } catch (err) {
      console.error('Ошибка при загрузке моделей:', err)
    }
  }

  const handleModelChange = (value: string) => {
    setValue('model', value)
    setSearchQuery(value)
    if (value) {
      setShowSuggestions(true)
    } else {
      setShowSuggestions(false)
    }
  }

  const handleModelFocus = () => {
    setShowSuggestions(true)
    setSearchQuery(watchedModel)
  }

  const selectModel = (modelName: string) => {
    setValue('model', modelName)
    setSearchQuery(modelName)
    setShowSuggestions(false)
    modelInputRef.current?.focus()
  }

  const filteredModels = models.filter(m =>
    m.name.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const handleFormSubmit = async (data: FormData) => {
    // Нормализация: trim + uppercase для ID
    const normalizedId = data.id.trim().toUpperCase()
    const normalizedModel = data.model.trim()
    await onSubmit(normalizedId, normalizedModel)
  }

  return (
    <form onSubmit={handleSubmit(handleFormSubmit)}>
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            ID картриджа (серийный номер) *
          </label>
          <input
            type="text"
            {...register('id', {
              required: 'ID обязателен',
              minLength: {
                value: 3,
                message: 'Минимальная длина ID - 3 символа',
              },
              maxLength: {
                value: 100,
                message: 'Максимальная длина ID - 100 символов',
              },
              pattern: {
                value: /^[a-zA-Z0-9\-_]+$/,
                message: 'ID может содержать только буквы, цифры, дефис и подчеркивание',
              },
            })}
            className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            placeholder="Например: CART-001"
            autoComplete="off"
          />
          {errors.id && <p className="mt-1 text-sm text-red-600">{errors.id.message}</p>}
        </div>

        <div className="relative">
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Модель *
          </label>
          <input
            type="text"
            {...register('model', {
              required: 'Модель обязательна',
              minLength: {
                value: 2,
                message: 'Минимальная длина модели - 2 символа',
              },
              maxLength: {
                value: 200,
                message: 'Максимальная длина модели - 200 символов',
              },
            })}
            ref={(e) => {
              register('model').ref(e)
              if (e) modelInputRef.current = e
            }}
            value={watchedModel}
            onChange={(e) => handleModelChange(e.target.value)}
            onFocus={handleModelFocus}
            className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            placeholder="Начните вводить название модели..."
            autoComplete="off"
          />
          {errors.model && <p className="mt-1 text-sm text-red-600">{errors.model.message}</p>}

          {/* Выпадающий список с подсказками */}
          {showSuggestions && filteredModels.length > 0 && (
            <ul
              ref={suggestionsRef}
              className="absolute z-10 w-full mt-1 bg-white border border-gray-300 rounded-md shadow-lg max-h-60 overflow-auto"
            >
              {filteredModels.map((model) => (
                <li
                  key={model.id}
                  onClick={() => selectModel(model.name)}
                  className="px-4 py-2 hover:bg-blue-50 cursor-pointer flex justify-between items-center"
                >
                  <span className="text-gray-900">{model.name}</span>
                  <span className="text-xs text-gray-500">
                    {model.usageCount} шт.
                  </span>
                </li>
              ))}
            </ul>
          )}

          {showSuggestions && filteredModels.length === 0 && searchQuery && (
            <div className="absolute z-10 w-full mt-1 bg-white border border-gray-300 rounded-md shadow-lg px-4 py-2 text-sm text-gray-500">
              Нет совпадений. Введите новое название модели.
            </div>
          )}
        </div>

        <div className="flex justify-end space-x-3 pt-4">
          <button
            type="button"
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
          >
            Отмена
          </button>
          <button
            type="submit"
            disabled={isSubmitting}
            className="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 disabled:opacity-50"
          >
            {isSubmitting ? 'Регистрация...' : 'Зарегистрировать'}
          </button>
        </div>
      </div>
    </form>
  )
}
