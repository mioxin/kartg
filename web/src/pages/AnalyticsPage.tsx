import React, { useState } from 'react'
import { useCartridgeStore } from '../store/cartridgeStore'
import { analyticsApi, downloadFile } from '../api/api'

export const AnalyticsPage: React.FC = () => {
  const { globalStats, fetchGlobalStats } = useCartridgeStore()
  const [refillsStats, setRefillsStats] = useState<{ totalRefills: number; uniqueCartridges: number } | null>(null)
  const [periodStart, setPeriodStart] = useState('')
  const [periodEnd, setPeriodEnd] = useState('')
  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState<'csv' | 'txt' | null>(null)

  React.useEffect(() => {
    fetchGlobalStats()
  }, [fetchGlobalStats])

  const handleLoadStats = async () => {
    if (!periodStart || !periodEnd) return
    setLoading(true)
    try {
      const stats = await analyticsApi.getRefillsStats(
        new Date(periodStart).toISOString(),
        new Date(periodEnd).toISOString()
      )
      setRefillsStats(stats)
    } catch (err) {
      console.error('Ошибка при загрузке статистики:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleExport = async (format: 'csv' | 'txt') => {
    if (!periodStart || !periodEnd) return
    setExporting(format)
    try {
      const blob = await analyticsApi.exportRefillsStats(periodStart, periodEnd, format)
      const filename = `refills_stats_${periodStart}_${periodEnd}.${format}`
      downloadFile(blob, filename)
    } catch (err) {
      console.error('Ошибка при экспорте:', err)
    } finally {
      setExporting(null)
    }
  }

  return (
    <div className="px-4 py-6 sm:px-0">
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Аналитика</h2>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-6 mb-8">
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <span className="text-3xl">📦</span>
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">
                    Всего картриджей
                  </dt>
                  <dd className="text-2xl font-semibold text-gray-900">
                    {globalStats?.totalCartridges ?? 0}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <span className="text-3xl">✅</span>
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">
                    В использовании
                  </dt>
                  <dd className="text-2xl font-semibold text-green-600">
                    {globalStats?.inUse ?? 0}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <span className="text-3xl">🔄</span>
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">
                    На заправке
                  </dt>
                  <dd className="text-2xl font-semibold text-yellow-600">
                    {globalStats?.refilling ?? 0}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <span className="text-3xl">🗑️</span>
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">
                    Утилизировано
                  </dt>
                  <dd className="text-2xl font-semibold text-red-600">
                    {globalStats?.retired ?? 0}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <span className="text-3xl">🔁</span>
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">
                    Всего заправок
                  </dt>
                  <dd className="text-2xl font-semibold text-blue-600">
                    {globalStats?.totalRefillsAllTime ?? 0}
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 mb-4">
            Статистика заправок за период
          </h3>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Начало периода
              </label>
              <input
                type="date"
                value={periodStart}
                onChange={(e) => setPeriodStart(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Конец периода
              </label>
              <input
                type="date"
                value={periodEnd}
                onChange={(e) => setPeriodEnd(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <button
              onClick={handleLoadStats}
              disabled={loading || !periodStart || !periodEnd}
              className="w-full px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 font-medium"
            >
              {loading ? 'Загрузка...' : 'Показать статистику'}
            </button>

            {refillsStats && (
              <div className="mt-4 p-4 bg-gray-50 rounded-md">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm text-gray-500">Всего заправок</p>
                    <p className="text-2xl font-semibold text-gray-900">
                      {refillsStats.totalRefills}
                    </p>
                  </div>
                  <div>
                    <p className="text-sm text-gray-500">Уникальных картриджей</p>
                    <p className="text-2xl font-semibold text-gray-900">
                      {refillsStats.uniqueCartridges}
                    </p>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        <div className="bg-white shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 mb-4">Экспорт данных</h3>
          <p className="text-gray-600 mb-4">
            Выгрузите статистику заправок за выбранный период
          </p>
          <div className="space-y-3">
            <button
              onClick={() => handleExport('csv')}
              disabled={exporting !== null || !periodStart || !periodEnd}
              className="w-full px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 font-medium disabled:opacity-50 flex items-center justify-center"
            >
              {exporting === 'csv' ? (
                <span className="flex items-center">
                  <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  Экспорт...
                </span>
              ) : (
                '📥 Экспорт в CSV'
              )}
            </button>
            <button
              onClick={() => handleExport('txt')}
              disabled={exporting !== null || !periodStart || !periodEnd}
              className="w-full px-4 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700 font-medium disabled:opacity-50 flex items-center justify-center"
            >
              {exporting === 'txt' ? (
                <span className="flex items-center">
                  <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  Экспорт...
                </span>
              ) : (
                '📄 Экспорт в TXT'
              )}
            </button>
          </div>
          {!periodStart || !periodEnd ? (
            <p className="mt-3 text-sm text-gray-500">
              Выберите период для экспорта данных
            </p>
          ) : (
            <p className="mt-3 text-sm text-gray-500">
              Период: {periodStart} — {periodEnd}
            </p>
          )}

          <div className="mt-6 pt-6 border-t">
            <h4 className="text-sm font-medium text-gray-700 mb-2">О системе</h4>
            <dl className="space-y-2 text-sm">
              <div className="flex justify-between">
                <dt className="text-gray-500">Версия:</dt>
                <dd className="text-gray-900">0.2.0</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">База данных:</dt>
                <dd className="text-gray-900">SQLite (WAL)</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">API:</dt>
                <dd className="text-gray-900">gRPC + REST</dd>
              </div>
            </dl>
          </div>
        </div>
      </div>
    </div>
  )
}
