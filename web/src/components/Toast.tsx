import React, { useEffect } from 'react'

export interface Toast {
  id: string
  message: string
  type: 'success' | 'error' | 'info'
}

interface ToastProps {
  toast: Toast
  onClose: (id: string) => void
}

export const Toast: React.FC<ToastProps> = ({ toast, onClose }) => {
  useEffect(() => {
    const timer = setTimeout(() => {
      onClose(toast.id)
    }, 5000)
    return () => clearTimeout(timer)
  }, [toast.id, onClose])

  const bgColor = {
    success: 'bg-green-50 border-green-200',
    error: 'bg-red-50 border-red-200',
    info: 'bg-blue-50 border-blue-200',
  }

  const textColor = {
    success: 'text-green-800',
    error: 'text-red-800',
    info: 'text-blue-800',
  }

  const icon = {
    success: '✅',
    error: '❌',
    info: 'ℹ️',
  }

  return (
    <div className={`mb-3 p-4 border rounded-md shadow-sm flex items-start justify-between ${bgColor[toast.type]} max-w-sm`}>
      <div className="flex items-start">
        <span className="text-xl mr-3">{icon[toast.type]}</span>
        <p className={`text-sm font-medium ${textColor[toast.type]}`}>{toast.message}</p>
      </div>
      <button
        onClick={() => onClose(toast.id)}
        className="ml-4 text-gray-400 hover:text-gray-600 focus:outline-none"
      >
        ✕
      </button>
    </div>
  )
}
