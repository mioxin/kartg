import React, { useState } from 'react'
import { useForm } from 'react-hook-form'
import { authApi } from '../api/api'

interface RegisterUserModalProps {
  isOpen: boolean
  onClose: () => void
  onRegistered: () => void
}

interface FormData {
  username: string
  password: string
  fullName: string
  role: 'user' | 'admin'
}

export const RegisterUserModal: React.FC<RegisterUserModalProps> = ({ isOpen, onClose, onRegistered }) => {
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const { register, handleSubmit, formState: { errors }, reset } = useForm<FormData>({
    defaultValues: {
      username: '',
      password: '',
      fullName: '',
      role: 'user',
    },
  })

  if (!isOpen) return null

  const handleFormSubmit = async (data: FormData) => {
    setLoading(true)
    setMessage(null)

    try {
      await authApi.register(data.username, data.password, data.fullName, data.role)
      setMessage({ type: 'success', text: `Пользователь ${data.username} успешно зарегистрирован` })
      reset()
      setTimeout(() => {
        onRegistered()
        onClose()
      }, 1500)
    } catch (err: any) {
      setMessage({ type: 'error', text: err.response?.data?.message || 'Ошибка при регистрации' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
        <div className="px-6 py-4 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">Регистрация пользователя</h3>
          <p className="text-sm text-gray-500 mt-1">
            Создание нового пользователя системы
          </p>
        </div>

        <form onSubmit={handleSubmit(handleFormSubmit)}>
          <div className="px-6 py-4 space-y-4">
            {message && (
              <div className={`p-3 rounded-md text-sm ${
                message.type === 'success' 
                  ? 'bg-green-50 text-green-800 border border-green-200' 
                  : 'bg-red-50 text-red-800 border border-red-200'
              }`}>
                {message.text}
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Имя пользователя *
              </label>
              <input
                type="text"
                {...register('username', {
                  required: 'Имя пользователя обязательно',
                  minLength: {
                    value: 3,
                    message: 'Минимальная длина - 3 символа',
                  },
                  pattern: {
                    value: /^[a-zA-Z0-9_]+$/,
                    message: 'Только буквы, цифры и подчеркивание',
                  },
                })}
                className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                placeholder="Например: ivanov"
                autoComplete="off"
              />
              {errors.username && (
                <p className="mt-1 text-sm text-red-600">{errors.username.message}</p>
              )}
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Полное имя
              </label>
              <input
                type="text"
                {...register('fullName')}
                className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                placeholder="Например: Иван Иванов"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Пароль *
              </label>
              <input
                type="password"
                {...register('password', {
                  required: 'Пароль обязателен',
                  minLength: {
                    value: 6,
                    message: 'Минимальная длина пароля - 6 символов',
                  },
                })}
                className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                autoComplete="new-password"
              />
              {errors.password && (
                <p className="mt-1 text-sm text-red-600">{errors.password.message}</p>
              )}
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Роль
              </label>
              <select
                {...register('role')}
                className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              >
                <option value="user">Пользователь</option>
                <option value="admin">Администратор</option>
              </select>
            </div>

            <div className="bg-blue-50 border border-blue-200 rounded-md p-3">
              <p className="text-sm text-blue-800">
                ℹ️ Пользователь с пустым паролем сможет войти без ввода пароля.
              </p>
            </div>
          </div>

          <div className="px-6 py-4 bg-gray-50 rounded-b-lg flex justify-end space-x-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
            >
              Отмена
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 disabled:opacity-50"
            >
              {loading ? 'Регистрация...' : 'Зарегистрировать'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
