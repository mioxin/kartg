import React from 'react'
import { useForm } from 'react-hook-form'

interface RegisterFormProps {
  onSubmit: (id: string, model: string) => Promise<void>
  onCancel: () => void
}

interface FormData {
  id: string
  model: string
}

export const RegisterForm: React.FC<RegisterFormProps> = ({ onSubmit, onCancel }) => {
  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm<FormData>({
    defaultValues: {
      id: '',
      model: '',
    },
  })

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

        <div>
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
            className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            placeholder="Например: HP 12A"
            autoComplete="off"
          />
          {errors.model && <p className="mt-1 text-sm text-red-600">{errors.model.message}</p>}
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
