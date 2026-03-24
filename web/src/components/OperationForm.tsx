import React from 'react'
import { useForm } from 'react-hook-form'

interface OperationFormProps {
  cartridgeId: string
  operationType: 'send' | 'receive' | 'retire'
  onSubmit: (cartridgeId: string, comment: string) => Promise<void>
  onCancel: () => void
}

interface FormData {
  comment: string
}

const operationConfig = {
  send: {
    title: 'Отправить на заправку',
    submitText: 'Отправить',
    submitColor: 'bg-yellow-600 hover:bg-yellow-700',
    commentRequired: false,
  },
  receive: {
    title: 'Принять с заправки',
    submitText: 'Принять',
    submitColor: 'bg-green-600 hover:bg-green-700',
    commentRequired: false,
  },
  retire: {
    title: 'Утилизировать картридж',
    submitText: 'Утилизировать',
    submitColor: 'bg-red-600 hover:bg-red-700',
    commentRequired: true, // Для утилизации комментарий обязателен
  },
}

export const OperationForm: React.FC<OperationFormProps> = ({
  cartridgeId,
  operationType,
  onSubmit,
  onCancel,
}) => {
  const config = operationConfig[operationType]
  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm<FormData>({
    defaultValues: {
      comment: '',
    },
  })

  const handleFormSubmit = async (data: FormData) => {
    await onSubmit(cartridgeId, data.comment)
  }

  return (
    <form onSubmit={handleSubmit(handleFormSubmit)}>
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            ID картриджа
          </label>
          <input
            type="text"
            value={cartridgeId}
            disabled
            className="w-full px-3 py-2 border border-gray-300 rounded-md bg-gray-50 text-gray-500"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Комментарий {config.commentRequired && <span className="text-red-500">*</span>}
          </label>
          <textarea
            {...register('comment', {
              required: config.commentRequired ? 'Комментарий обязателен для утилизации' : false,
              minLength: config.commentRequired ? {
                value: 10,
                message: 'Минимальная длина комментария - 10 символов',
              } : undefined,
            })}
            rows={3}
            className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            placeholder={config.commentRequired ? 'Укажите причину утилизации' : 'Необязательно'}
          />
          {errors.comment && <p className="mt-1 text-sm text-red-600">{errors.comment.message}</p>}
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
            className={`px-4 py-2 text-sm font-medium text-white border border-transparent rounded-md disabled:opacity-50 ${config.submitColor}`}
          >
            {isSubmitting ? 'Обработка...' : config.submitText}
          </button>
        </div>
      </div>
    </form>
  )
}
