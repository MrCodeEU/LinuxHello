import React, { useState, useMemo } from 'react'
import { Shield, CheckCircle, AlertCircle, Loader2, Terminal, Key, Lock } from 'lucide-react'
import type { PAMServiceStatus } from '../wails'

interface PAMTabProps {
  pamStatus: string
  pamServices: PAMServiceStatus[]
  isProcessing: boolean
  commandOutput?: string
  handlePAMAction: (action: string, service?: string) => void
  handlePAMToggle: (enable: boolean) => void
}

export const PAMTab: React.FC<PAMTabProps> = ({
  pamStatus,
  pamServices,
  isProcessing,
  commandOutput,
  handlePAMAction,
  handlePAMToggle
}) => {
  const [selectedServices, setSelectedServices] = useState<string[]>([])

  const getServiceIcon = (name: string) => {
    if (!name) return <Shield size={20} className="text-gray-400" />
    const lowerName = name.toLowerCase()
    if (lowerName.includes('sudo')) return <Terminal size={20} className="text-red-400" />
    if (lowerName.includes('polkit')) return <Shield size={20} className="text-blue-400" />
    if (lowerName.includes('login')) return <Key size={20} className="text-green-400" />
    if (lowerName.includes('su') && !lowerName.includes('sudo')) return <Lock size={20} className="text-orange-400" />
    if (lowerName.includes('sddm')) return <CheckCircle size={20} className="text-purple-400" />
    if (lowerName.includes('gdm')) return <CheckCircle size={20} className="text-indigo-400" />
    if (lowerName.includes('lightdm')) return <CheckCircle size={20} className="text-cyan-400" />
    if (lowerName.includes('kde')) return <CheckCircle size={20} className="text-pink-400" />
    return <Shield size={20} className="text-gray-400" />
  }

  const getServiceCategory = (name: string): 'system' | 'desktop' | 'elevated' => {
    if (!name) return 'system'
    const lowerName = name.toLowerCase()
    if (lowerName.includes('sudo') || lowerName.includes('su')) return 'elevated'
    if (lowerName.includes('dm') || lowerName.includes('polkit') || lowerName.includes('kde')) return 'desktop'
    return 'system'
  }

  const handleServiceToggle = (serviceId: string, enable: boolean) => {
    const action = enable ? 'enable' : 'disable'
    handlePAMAction(action, serviceId)
  }

  const handleBulkToggle = (enable: boolean) => {
    if (selectedServices.length === 0) return
    selectedServices.forEach(serviceId => {
      handleServiceToggle(serviceId, enable)
    })
    setSelectedServices([])
  }

  const getCategoryIcon = (category: string) => {
    switch (category) {
      case 'elevated': return <Terminal size={16} className="text-red-400" />
      case 'desktop': return <Shield size={16} className="text-blue-400" />
      case 'system': return <Key size={16} className="text-green-400" />
      default: return <Shield size={16} className="text-gray-400" />
    }
  }

  const getCategoryTitle = (category: string) => {
    switch (category) {
      case 'elevated': return 'Elevated Access'
      case 'desktop': return 'Desktop Managers'
      case 'system': return 'System Authentication'
      default: return 'Other'
    }
  }

  const getStatusBadge = (status: string) => {
    const baseClasses = 'px-2 py-1 rounded-full text-xs font-medium'
    if (!status) return <span className={`${baseClasses} bg-gray-900 text-gray-300 border border-gray-700`}>Unknown</span>
    switch (status.toLowerCase()) {
      case 'enabled':
        return <span className={`${baseClasses} bg-green-900 text-green-300 border border-green-700`}>Enabled</span>
      case 'disabled':
        return <span className={`${baseClasses} bg-red-900 text-red-300 border border-red-700`}>Disabled</span>
      case 'not installed':
        return <span className={`${baseClasses} bg-yellow-900 text-yellow-300 border border-yellow-700`}>Not Installed</span>
      default:
        return <span className={`${baseClasses} bg-gray-900 text-gray-300 border border-gray-700`}>{status}</span>
    }
  }

  const groupedServices = useMemo(() => {
    if (!pamServices || pamServices.length === 0) {
      return {}
    }
    // Use lowercase property names as they come from JSON
    const validServices = pamServices.filter((s: any) => s && s.name && s.id)
    console.log('PAM Services received:', pamServices.length, 'Valid:', validServices.length, pamServices)
    return validServices.reduce((acc: any, service: any) => {
      const category = getServiceCategory(service.name)
      if (!acc[category]) acc[category] = []
      acc[category].push(service)
      return acc
    }, {} as Record<string, any[]>)
  }, [pamServices])

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3 mb-6">
        <Shield className="text-red-500" size={28} />
        <h2 className="text-2xl font-bold text-gray-100">PAM Integration Manager</h2>
      </div>

      <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
        <h3 className="text-lg font-medium text-gray-200 mb-4">Current PAM Status</h3>
        <div className="p-4 bg-neutral-900 rounded-lg border border-neutral-600">
          <pre className="text-sm text-gray-300 font-mono whitespace-pre-wrap">{pamStatus}</pre>
        </div>
      </div>

      {/* Debug info */}
      <div className="bg-blue-900/20 border border-blue-600 p-4 rounded-xl">
        <h4 className="text-blue-300 font-medium mb-2">Debug Info</h4>
        <p className="text-blue-200 text-sm">Services count: {pamServices?.length || 0}</p>
        <p className="text-blue-200 text-sm">Grouped categories: {Object.keys(groupedServices).length}</p>
        {pamServices && pamServices.length > 0 && (
          <div className="mt-2">
            <p className="text-blue-200 text-sm font-semibold">First service data:</p>
            <pre className="text-xs text-blue-100 mt-1 overflow-x-auto">{JSON.stringify(pamServices[0], null, 2)}</pre>
          </div>
        )}
      </div>

      {selectedServices.length > 0 && (
        <div className="bg-blue-900/20 border border-blue-700 p-4 rounded-xl">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <AlertCircle size={20} className="text-blue-400" />
              <span className="text-blue-300">
                {selectedServices.length} service{selectedServices.length === 1 ? '' : 's'} selected
              </span>
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => handleBulkToggle(true)}
                disabled={isProcessing}
                className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
              >
                Enable Selected
              </button>
              <button
                onClick={() => handleBulkToggle(false)}
                disabled={isProcessing}
                className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
              >
                Disable Selected
              </button>
              <button
                onClick={() => setSelectedServices([])}
                className="px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors text-sm"
              >
                Clear
              </button>
            </div>
          </div>
        </div>
      )}

      {(!pamServices || pamServices.length === 0) && (
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <div className="flex items-center gap-3 text-gray-400">
            <AlertCircle size={24} />
            <div>
              <h3 className="text-lg font-medium text-gray-200 mb-2">No PAM Services Found</h3>
              <p className="text-sm">
                The PAM management script may not be installed or accessible. 
                Try clicking "Refresh Status" or "List Services" to reload the service list.
              </p>
            </div>
          </div>
        </div>
      )}

      {pamServices && pamServices.length > 0 && Object.entries(groupedServices).map(([category, services]) => (
          <div key={category} className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
            <h3 className="text-lg font-medium text-gray-200 mb-4 flex items-center gap-2">
              {getCategoryIcon(category)}
              {getCategoryTitle(category)}
            </h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {services.map((service: any) => (
                <div
                  key={service.id}
                  className={`p-4 rounded-lg border transition-colors ${
                    selectedServices.includes(service.id)
                      ? 'bg-blue-900/20 border-blue-600'
                      : 'bg-neutral-900 border-neutral-700 hover:border-neutral-600'
                  }`}
                >
                  <div className="flex items-start justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={selectedServices.includes(service.id)}
                        onChange={(e) => {
                          const newSelection = e.target.checked
                            ? [...selectedServices, service.id]
                            : selectedServices.filter(id => id !== service.id)
                          setSelectedServices(newSelection)
                        }}
                        className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
                      />
                      {getServiceIcon(service.name)}
                      <span className="font-medium text-gray-200">{service.name}</span>
                    </div>
                    {getStatusBadge(service.status)}
                  </div>
                  <p className="text-sm text-gray-400 mb-3">{service.pamFile}</p>
                  <div className="flex gap-2">
                    <button
                      onClick={() => handleServiceToggle(service.id, true)}
                      disabled={isProcessing || service.status === 'not installed'}
                      className="flex-1 px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                    >
                      Enable
                    </button>
                    <button
                      onClick={() => handleServiceToggle(service.id, false)}
                      disabled={isProcessing || service.status === 'not installed'}
                      className="flex-1 px-3 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                    >
                      Disable
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))
      }

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-4">Legacy PAM Controls</h3>
          <div className="space-y-3">
            <button
              onClick={() => handlePAMToggle(true)}
              disabled={isProcessing}
              className="w-full px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Enable PAM Module
            </button>
            <button
              onClick={() => handlePAMToggle(false)}
              disabled={isProcessing}
              className="w-full px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Disable PAM Module
            </button>
          </div>
        </div>

        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-4">System Actions</h3>
          <div className="grid grid-cols-2 gap-3">
            <button
              onClick={() => handlePAMAction('status', '')}
              disabled={isProcessing}
              className="px-3 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
            >
              Refresh Status
            </button>
            <button
              onClick={() => handlePAMAction('list', '')}
              disabled={isProcessing}
              className="px-3 py-2 bg-orange-600 text-white rounded-lg hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
            >
              List Services
            </button>
          </div>
        </div>
      </div>

      {commandOutput && (
        <div className="bg-neutral-800 p-4 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-2 flex items-center gap-2">
            {isProcessing && <Loader2 className="animate-spin" size={16} />}
            Command Output
          </h3>
          <pre className="text-sm text-gray-300 font-mono bg-black p-3 rounded overflow-x-auto whitespace-pre-wrap">
            {commandOutput}
          </pre>
        </div>
      )}

      <div className="bg-yellow-900/20 border border-yellow-700 p-4 rounded-xl">
        <div className="flex items-start gap-3">
          <AlertCircle size={20} className="text-yellow-400 mt-0.5" />
          <div>
            <h4 className="text-yellow-300 font-medium mb-1">Security Warning</h4>
            <p className="text-yellow-200 text-sm">
              Modifying PAM configuration can affect system security and access. Always ensure you have an alternative way to access your system (SSH, physical console) before making changes. 
              Test face authentication thoroughly before enabling it for critical services like login or sudo.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
