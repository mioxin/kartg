import React, { useState } from 'react'
import { useForm } from 'react-hook-form'
import { authApi } from '../api/api'
import { useAuthStore } from '../store/authStore'

interface ChangePasswordModalProps {
  isOpen: boolean
  onClose: () => void
}

interface FormData {
  oldPassword: string
  newPassword: string
  confirmPassword: string
}

export const ChangePasswordModal: React.FC<ChangePasswordModalProps> = ({ isOpen, onClose }) => {
  const { user } = useAuthStore()
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const { register, handleSubmit, formState: { errors }, watch } = useForm<FormData>({
    defaultValues: {
      oldPassword: '',
      newPassword: '',
      confirmPassword: '',
    },
  })

  const newPassword = watch('newPassword')

  if (!isOpen) return null

  const handleFormSubmit = async (data: FormData) => {
    setLoading(true)
    setMessage(null)

    try {
      await authApi.changePassword(data.oldPassword, data.newPassword)
      setMessage({ type: 'success', text: 'Пароль успешно изменен' })
      setTimeout(() => {
        onClose()
      }, 1500)
    } catch (err: any) {
      setMessage({ type: 'error', text: err.response?.data?.message || 'Ошибка при смене пароля' })
    } finally {
      setLoading(false)
    }
  }

  const hasPassword = user?.username === 'admin'

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
        <div className="px-6 py-4 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">Смена пароля</h3>
          <p className="text-sm text-gray-500 mt-1">
            Пользователь: <span className="font-medium">{user?.username}</span>
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

            {hasPassword && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Текущий пароль
                </label>
                <input
                  type="password"
                  {...register('oldPassword', {
                    required: 'Введите текущий пароль',
                  })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  autoComplete="current-password"
                />
                {errors.oldPassword && (
                  <p className="mt-1 text-sm text-red-600">{errors.oldPassword.message}</p>
                )}
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Новый пароль
              </label>
              <input
                type="password"
                {...register('newPassword', {
                  required: 'Введите новый пароль',
                  minLength: {
                    value: 6,
                    message: 'Минимальная длина пароля - 6 символов',
                  },
                })}
                className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                autoComplete="new-password"
              />
              {errors.newPassword && (
                <p className="mt-1 text-sm text-red-600">{errors.newPassword.message}</p>
              )}
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Подтверждение пароля
              </label>
              <input
                type="password"
                {...register('confirmPassword', {
                  required: 'Подтвердите новый пароль',
                  validate: value => value === newPassword || 'Пароли не совпадают',
                })}
                className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                autoComplete="new-password"
              />
              {errors.confirmPassword && (
                <p className="mt-1 text-sm text-red-600">{errors.confirmPassword.message}</p>
              )}
            </div>

            {!hasPassword && (
              <div className="bg-yellow-50 border border-yellow-200 rounded-md p-3">
                <p className="text-sm text-yellow-800">
                  ℹ️ У вас не установлен пароль. Просто задайте новый пароль.
                </p>
              </div>
            )}
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
              {loading ? 'Сохранение...' : 'Сохранить'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
