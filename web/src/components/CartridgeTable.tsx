import React from 'react'
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  SortingState,
  useReactTable,
} from '@tanstack/react-table'
import { Cartridge } from '../api/api'

const columnHelper = createColumnHelper<Cartridge>()

const statusLabels: Record<string, { label: string; color: string }> = {
  CARTRIDGE_STATUS_IN_USE: { label: 'В использовании', color: 'bg-green-100 text-green-800' },
  CARTRIDGE_STATUS_REFILLING: { label: 'На заправке', color: 'bg-yellow-100 text-yellow-800' },
  CARTRIDGE_STATUS_RETIRED: { label: 'Утилизирован', color: 'bg-red-100 text-red-800' },
}

interface CartridgeTableProps {
  cartridges: Cartridge[]
  onSendToRefill: (id: string) => void
  onReceiveFromRefill: (id: string) => void
  onRetire: (id: string) => void
  onViewHistory: (id: string) => void
}

export const CartridgeTable: React.FC<CartridgeTableProps> = ({
  cartridges,
  onSendToRefill,
  onReceiveFromRefill,
  onRetire,
  onViewHistory,
}) => {
  const [sorting, setSorting] = React.useState<SortingState>([])

  const columns = [
    columnHelper.accessor('id', {
      header: ({ column }) => (
        <button
          className="flex items-center space-x-1 hover:text-gray-700 font-medium"
          onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
        >
          <span>ID</span>
          {column.getIsSorted() === 'asc' && <span>↑</span>}
          {column.getIsSorted() === 'desc' && <span>↓</span>}
        </button>
      ),
      cell: info => info.getValue(),
    }),
    columnHelper.accessor('model', {
      header: ({ column }) => (
        <button
          className="flex items-center space-x-1 hover:text-gray-700 font-medium"
          onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
        >
          <span>Модель</span>
          {column.getIsSorted() === 'asc' && <span>↑</span>}
          {column.getIsSorted() === 'desc' && <span>↓</span>}
        </button>
      ),
      cell: info => info.getValue(),
    }),
    columnHelper.accessor('status', {
      header: 'Статус',
      cell: info => {
        const status = info.getValue()
        const config = statusLabels[status] || { label: status, color: 'bg-gray-100 text-gray-800' }
        return (
          <span className={`px-2 py-1 text-xs font-medium rounded-full ${config.color}`}>
            {config.label}
          </span>
        )
      },
    }),
    columnHelper.accessor('totalRefills', {
      header: ({ column }) => (
        <button
          className="flex items-center space-x-1 hover:text-gray-700 font-medium"
          onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
        >
          <span>Заправки</span>
          {column.getIsSorted() === 'asc' && <span>↑</span>}
          {column.getIsSorted() === 'desc' && <span>↓</span>}
        </button>
      ),
      cell: info => info.getValue(),
    }),
    columnHelper.accessor('createdAt', {
      header: ({ column }) => (
        <button
          className="flex items-center space-x-1 hover:text-gray-700 font-medium"
          onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
        >
          <span>Дата регистрации</span>
          {column.getIsSorted() === 'asc' && <span>↑</span>}
          {column.getIsSorted() === 'desc' && <span>↓</span>}
        </button>
      ),
      cell: info => {
        const date = new Date(info.getValue())
        return date.toLocaleDateString('ru-RU')
      },
    }),
    columnHelper.display({
      id: 'actions',
      header: 'Действия',
      cell: info => {
        const cartridge = info.row.original
        return (
          <div className="flex space-x-2">
            {cartridge.status !== 'CARTRIDGE_STATUS_RETIRED' && (
              <>
                {cartridge.status !== 'CARTRIDGE_STATUS_REFILLING' && (
                  <button
                    onClick={() => onSendToRefill(cartridge.id)}
                    className="text-yellow-600 hover:text-yellow-900 text-sm font-medium"
                    title="Отправить на заправку"
                  >
                    🔄
                  </button>
                )}
                {cartridge.status === 'CARTRIDGE_STATUS_REFILLING' && (
                  <button
                    onClick={() => onReceiveFromRefill(cartridge.id)}
                    className="text-green-600 hover:text-green-900 text-sm font-medium"
                    title="Принять с заправки"
                  >
                    ✅
                  </button>
                )}
                <button
                  onClick={() => onRetire(cartridge.id)}
                  className="text-red-600 hover:text-red-900 text-sm font-medium"
                  title="Утилизировать"
                >
                  🗑️
                </button>
              </>
            )}
            <button
              onClick={() => onViewHistory(cartridge.id)}
              className="text-blue-600 hover:text-blue-900 text-sm font-medium"
              title="История"
            >
              📋
            </button>
          </div>
        )
      },
    }),
  ]

  const table = useReactTable({
    data: cartridges,
    columns,
    state: {
      sorting,
    },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-gray-200">
        <thead className="bg-gray-50">
          {table.getHeaderGroups().map(headerGroup => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map(header => (
                <th
                  key={header.id}
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  {flexRender(header.column.columnDef.header, header.getContext())}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody className="bg-white divide-y divide-gray-200">
          {table.getRowModel().rows.map(row => (
            <tr key={row.id} className="hover:bg-gray-50">
              {row.getVisibleCells().map(cell => (
                <td key={cell.id} className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {cartridges.length === 0 && (
        <div className="text-center py-8 text-gray-500">
          Картриджи не найдены
        </div>
      )}
    </div>
  )
}
