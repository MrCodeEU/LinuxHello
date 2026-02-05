import React, { useState, useEffect } from 'react'
import { FileText, Download, RefreshCw } from 'lucide-react'

interface LogEntry {
  timestamp: string
  level: string
  message: string
  component?: string
}

export const LogsTab: React.FC = () => {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [selectedLevel, setSelectedLevel] = useState('all')

  const fetchLogs = async () => {
    setIsRefreshing(true)
    try {
      const res = await fetch('/api/logs')
      if (res.ok) {
        const logData = await res.json()
        setLogs(logData)
      }
    } catch (error) {
      console.error('Failed to fetch logs:', error)
    } finally {
      setIsRefreshing(false)
    }
  }

  useEffect(() => {
    fetchLogs()
    const interval = setInterval(fetchLogs, 5000) // Refresh every 5 seconds
    return () => clearInterval(interval)
  }, [])

  const filteredLogs = logs.filter(log => 
    selectedLevel === 'all' || log.level.toLowerCase() === selectedLevel
  )

  const getLevelColor = (level: string) => {
    const colors = {
      error: 'text-red-400 bg-red-900/30',
      warn: 'text-yellow-400 bg-yellow-900/30',
      info: 'text-blue-400 bg-blue-900/30',
      debug: 'text-gray-400 bg-gray-900/30'
    }
    return colors[level.toLowerCase() as keyof typeof colors] || 'text-gray-400 bg-gray-900/30'
  }

  const downloadLogs = async () => {
    try {
      const res = await fetch('/api/logs/download')
      if (res.ok) {
        const blob = await res.blob()
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `linuxhello-logs-${new Date().toISOString().slice(0, 10)}.log`
        document.body.appendChild(a)
        a.click()
        window.URL.revokeObjectURL(url)
        document.body.removeChild(a)
      }
    } catch (error) {
      console.error('Failed to download logs:', error)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <FileText className="text-gray-500" size={28} />
          <h2 className="text-2xl font-bold text-gray-100">System Logs</h2>
        </div>
        <div className="flex items-center gap-3">
          <select
            value={selectedLevel}
            onChange={(e) => setSelectedLevel(e.target.value)}
            className="bg-neutral-800 border border-neutral-700 rounded-lg px-3 py-2 text-gray-300 focus:ring-2 focus:ring-blue-500 outline-none"
          >
            <option value="all">All Levels</option>
            <option value="error">Error</option>
            <option value="warn">Warning</option>
            <option value="info">Info</option>
            <option value="debug">Debug</option>
          </select>
          <button
            onClick={fetchLogs}
            disabled={isRefreshing}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 transition-colors"
          >
            <RefreshCw size={16} className={isRefreshing ? 'animate-spin' : ''} />
            Refresh
          </button>
          <button
            onClick={downloadLogs}
            className="flex items-center gap-2 px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors"
          >
            <Download size={16} />
            Download
          </button>
        </div>
      </div>

      <div className="bg-neutral-800 rounded-xl border border-neutral-700">
        <div className="p-4 border-b border-neutral-700 flex items-center justify-between">
          <h3 className="text-lg font-medium text-gray-200">Recent Activity</h3>
          <span className="text-sm text-gray-400">{filteredLogs.length} entries</span>
        </div>
        
        <div className="max-h-96 overflow-y-auto">
          {filteredLogs.length > 0 ? (
            <div className="divide-y divide-neutral-700">
              {filteredLogs.map((log, index) => (
                <div key={index} className="p-4 hover:bg-neutral-750 transition-colors">
                  <div className="flex items-start gap-3">
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getLevelColor(log.level)}`}>
                      {log.level.toUpperCase()}
                    </span>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="text-xs text-gray-400 font-mono">
                          {log.timestamp}
                        </span>
                        {log.component && (
                          <span className="text-xs text-blue-400 bg-blue-900/30 px-2 py-0.5 rounded">
                            {log.component}
                          </span>
                        )}
                      </div>
                      <p className="text-sm text-gray-300 font-mono break-all">
                        {log.message}
                      </p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="p-8 text-center text-gray-400">
              <FileText size={48} className="mx-auto mb-4 opacity-50" />
              <p>No logs available for the selected level.</p>
            </div>
          )}
        </div>
      </div>

      <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
        <h3 className="text-lg font-medium text-gray-200 mb-3">Log Information</h3>
        <div className="space-y-2 text-sm text-gray-400">
          <p>• Logs are automatically refreshed every 5 seconds</p>
          <p>• Use the level filter to focus on specific types of messages</p>
          <p>• Download logs for offline analysis or support requests</p>
          <p>• System logs include authentication attempts, enrollment activities, and errors</p>
        </div>
      </div>
    </div>
  )
}