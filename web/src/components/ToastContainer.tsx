import React from 'react'
import { Toast, Toast as ToastType } from './Toast'

interface ToastContainerProps {
  toasts: ToastType[]
  removeToast: (id: string) => void
}

export const ToastContainer: React.FC<ToastContainerProps> = ({ toasts, removeToast }) => {
  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col items-end">
      {toasts.map(toast => (
        <Toast key={toast.id} toast={toast} onClose={removeToast} />
      ))}
    </div>
  )
}
