import React, { useState } from 'react'
import { Shield, CheckCircle, AlertCircle, Loader2, Terminal, Key, Lock } from 'lucide-react'

interface PAMTabProps {
  pamStatus: string
  isProcessing: boolean
  commandOutput?: string
  handlePAMAction: (action: string) => void
  handlePAMToggle: (enable: boolean) => void
}

interface PAMService {
  id: string
  name: string
  description: string
  category: 'system' | 'desktop' | 'elevated'
  icon: React.ReactNode
  status: 'enabled' | 'disabled' | 'unknown'
}

export const PAMTab: React.FC<PAMTabProps> = ({
  pamStatus,
  isProcessing,
  commandOutput,
  handlePAMAction,
  handlePAMToggle
}) => {
  const [selectedServices, setSelectedServices] = useState<string[]>([])

  // PAM services configuration
  const pamServices: PAMService[] = [
    {
      id: 'sudo',
      name: 'Sudo',
      description: 'Administrative commands with sudo',
      category: 'elevated',
      icon: <Terminal size={20} className="text-red-400" />,
      status: 'unknown'
    },
    {
      id: 'polkit',
      name: 'PolicyKit',
      description: 'GUI authentication dialogs (KDE/GNOME)',
      category: 'desktop',
      icon: <Shield size={20} className="text-blue-400" />,
      status: 'unknown'
    },
    {
      id: 'login',
      name: 'System Login',
      description: 'Console and TTY login',
      category: 'system',
      icon: <Key size={20} className="text-green-400" />,
      status: 'unknown'
    },
    {
      id: 'su',
      name: 'Switch User (su)',
      description: 'User switching with su command',
      category: 'elevated',
      icon: <Lock size={20} className="text-orange-400" />,
      status: 'unknown'
    },
    {
      id: 'sddm',
      name: 'SDDM',
      description: 'Simple Desktop Display Manager (KDE)',
      category: 'desktop',
      icon: <CheckCircle size={20} className="text-purple-400" />,
      status: 'unknown'
    },
    {
      id: 'gdm',
      name: 'GDM',
      description: 'GNOME Display Manager',
      category: 'desktop',
      icon: <CheckCircle size={20} className="text-indigo-400" />,
      status: 'unknown'
    },
    {
      id: 'lightdm',
      name: 'LightDM',
      description: 'Lightweight Display Manager',
      category: 'desktop',
      icon: <CheckCircle size={20} className="text-cyan-400" />,
      status: 'unknown'
    }
  ]

  const handleServiceToggle = (serviceId: string, enable: boolean) => {
    const action = enable ? 'enable' : 'disable'
    // This would need to be updated to call a new API endpoint for service-specific PAM management
    handlePAMAction(`${action}-${serviceId}`)
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
    switch (status) {
      case 'enabled':
        return <span className={`${baseClasses} bg-green-900 text-green-300 border border-green-700`}>Enabled</span>
      case 'disabled':
        return <span className={`${baseClasses} bg-red-900 text-red-300 border border-red-700`}>Disabled</span>
      default:
        return <span className={`${baseClasses} bg-gray-900 text-gray-300 border border-gray-700`}>Unknown</span>
    }
  }

  const groupedServices = pamServices.reduce((acc, service) => {
    if (!acc[service.category]) acc[service.category] = []
    acc[service.category].push(service)
    return acc
  }, {} as Record<string, PAMService[]>)

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3 mb-6">
        <Shield className="text-red-500" size={28} />
        <h2 className="text-2xl font-bold text-gray-100">PAM Integration Manager</h2>
      </div>

      {/* Current Status */}
      <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
        <h3 className="text-lg font-medium text-gray-200 mb-4">Current PAM Status</h3>
        <div className="p-4 bg-neutral-900 rounded-lg border border-neutral-600">
          <pre className="text-sm text-gray-300 font-mono whitespace-pre-wrap">{pamStatus}</pre>
        </div>
      </div>

      {/* Service Selection */}
      {selectedServices.length > 0 && (
        <div className="bg-blue-900/20 border border-blue-700 p-4 rounded-xl">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <AlertCircle size={20} className="text-blue-400" />
              <span className="text-blue-300">
                {selectedServices.length} service{selectedServices.length !== 1 ? 's' : ''} selected
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

      {/* PAM Services by Category */}
      {Object.entries(groupedServices).map(([category, services]) => (
        <div key={category} className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-4 flex items-center gap-2">
            {getCategoryIcon(category)}
            {getCategoryTitle(category)}
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {services.map((service) => (
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
                        if (e.target.checked) {
                          setSelectedServices([...selectedServices, service.id])
                        } else {
                          setSelectedServices(selectedServices.filter(id => id !== service.id))
                        }
                      }}
                      className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
                    />
                    {service.icon}
                    <span className="font-medium text-gray-200">{service.name}</span>
                  </div>
                  {getStatusBadge(service.status)}
                </div>
                <p className="text-sm text-gray-400 mb-3">{service.description}</p>
                <div className="flex gap-2">
                  <button
                    onClick={() => handleServiceToggle(service.id, true)}
                    disabled={isProcessing}
                    className="flex-1 px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                  >
                    Enable
                  </button>
                  <button
                    onClick={() => handleServiceToggle(service.id, false)}
                    disabled={isProcessing}
                    className="flex-1 px-3 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                  >
                    Disable
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}

      {/* Legacy PAM Controls */}
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
            {['status', 'install', 'uninstall', 'backup', 'restore'].map(action => (
              <button
                key={action}
                onClick={() => handlePAMAction(action)}
                disabled={isProcessing}
                className="px-3 py-2 bg-orange-600 text-white rounded-lg hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm capitalize"
              >
                {action}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Command Output */}
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

      {/* Warning */}
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