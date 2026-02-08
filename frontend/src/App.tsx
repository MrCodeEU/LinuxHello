import { useState, useEffect } from 'react'
import { Camera, Users, Settings, Activity, Shield, HardDrive, Terminal, Loader2 } from 'lucide-react'
import { useAppData } from './hooks/useAppData'
import { useEnrollment } from './hooks/useEnrollment'
import { useAuthTesting } from './hooks/useAuthTesting'
import { useServiceActions } from './hooks/useServiceActions'
import { useConfiguration } from './hooks/useConfiguration'
import { EnrollmentTab } from './components/EnrollmentTab'
import { UsersTab } from './components/UsersTab'
import { AuthTestTab } from './components/AuthTestTab'
import { SettingsTab } from './components/SettingsTab'
import { PAMTab } from './components/PAMTab'
import { LogsTab } from './components/LogsTab'
import { ErrorBoundary } from './components/ErrorBoundary'
import ModelDownloadModal from './components/ModelDownloadModal'
import { checkModels } from './wails'

function App() {
  const [activeTab, setActiveTab] = useState('enroll')
  const [showModelModal, setShowModelModal] = useState(false)
  const [modelCheckDone, setModelCheckDone] = useState(false)
  
  const { users, config, setConfig, pamStatus, pamServices, serviceInfo, fetchData, fetchUsers, fetchConfig, fetchServiceStatus } = useAppData()
  const { enrollName, setEnrollName, enrollStatus, isEnrolling, enrollmentProgress, handleEnroll } = useEnrollment(fetchUsers)
  const { authTestResult, isAuthTesting, handleAuthTest } = useAuthTesting()
  const { commandOutput, isProcessing, handlePAMAction, handlePAMToggle, handleServiceAction } = useServiceActions(fetchData, fetchServiceStatus)
  const { saveStatus, handleSaveConfig, handleDeleteUser } = useConfiguration(fetchConfig)

  // Check for models on first load
  useEffect(() => {
    const checkForModels = async () => {
      try {
        const modelStatus = await checkModels()
        if (!modelStatus.allModelsPresent) {
          setShowModelModal(true)
        }
      } catch (err) {
        console.error('Failed to check models:', err)
      } finally {
        setModelCheckDone(true)
      }
    }
    
    checkForModels()
  }, [])

  const getServiceStatusClass = (status: string) => {
    const baseClasses = 'px-3 py-1 rounded-full text-sm'
    if (status === 'active') {
      return `${baseClasses} bg-green-900 text-green-300 border border-green-700`
    }
    if (status === 'inactive') {
      return `${baseClasses} bg-red-900 text-red-300 border border-red-700`
    }
    return `${baseClasses} bg-yellow-900 text-yellow-300 border border-yellow-700`
  }

  useEffect(() => {
    // Only fetch all data on initial load
    if (activeTab === 'settings') {
      // For settings tab, only fetch non-config data
      fetchUsers()
    } else {
      fetchData()
    }
    
    const interval = setInterval(() => {
        if (activeTab === 'service') fetchServiceStatus()
    }, 5000)
    return () => clearInterval(interval)
  }, [activeTab, fetchData, fetchUsers, fetchServiceStatus])

  // Initial load
  useEffect(() => {
    fetchData()
  }, [])

  const renderTabContent = () => {
    switch (activeTab) {
      case 'enroll':
        return (
          <EnrollmentTab
            enrollName={enrollName}
            setEnrollName={setEnrollName}
            enrollStatus={enrollStatus}
            isEnrolling={isEnrolling}
            enrollmentProgress={enrollmentProgress}
            handleEnroll={handleEnroll}
          />
        )
      
      case 'users':
        return (
          <UsersTab
            users={users}
            handleDeleteUser={handleDeleteUser}
            fetchData={fetchData}
          />
        )
      
      case 'authtest':
        return (
          <AuthTestTab
            authTestResult={authTestResult}
            isAuthTesting={isAuthTesting}
            handleAuthTest={handleAuthTest}
          />
        )
      
      case 'pam':
        return (
          <ErrorBoundary>
            <PAMTab
              pamStatus={pamStatus}
              pamServices={pamServices}
              isProcessing={isProcessing}
              commandOutput={commandOutput}
              handlePAMAction={handlePAMAction}
              handlePAMToggle={handlePAMToggle}
            />
          </ErrorBoundary>
        )
      
      case 'service':
        return renderServiceTab()
      
      case 'settings':
        return (
          <SettingsTab
            config={config}
            setConfig={setConfig}
            saveStatus={saveStatus}
            handleSaveConfig={handleSaveConfig}
          />
        )
      
      case 'logs':
        return <LogsTab />
      
      default:
        return null
    }
  }

  const renderServiceTab = () => (
    <div className="space-y-6">
      <div className="flex items-center gap-3 mb-6">
        <HardDrive className="text-orange-500" size={28} />
        <h2 className="text-2xl font-bold text-gray-100">Service Manager</h2>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-4">Service Status</h3>
          <div className="space-y-3">
            <div className="flex justify-between items-center">
              <span className="text-gray-300">Status:</span>
              <span className={getServiceStatusClass(serviceInfo.status)}>
                {serviceInfo.status}
              </span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-gray-300">Enabled:</span>
              <span className={`px-3 py-1 rounded-full text-sm ${
                serviceInfo.enabled === 'enabled' 
                  ? 'bg-green-900 text-green-300 border border-green-700' 
                  : 'bg-red-900 text-red-300 border border-red-700'
              }`}>
                {serviceInfo.enabled}
              </span>
            </div>
          </div>
        </div>

        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-4">Service Actions</h3>
          <div className="grid grid-cols-2 gap-3">
            {['start', 'stop', 'restart', 'enable', 'disable'].map(action => (
              <button
                key={action}
                onClick={() => handleServiceAction(action)}
                disabled={isProcessing}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm capitalize"
              >
                {action}
              </button>
            ))}
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
    </div>
  )

  return (
    <div className="min-h-screen bg-neutral-900 text-gray-100 flex font-sans w-full overflow-hidden">
      {/* Sidebar */}
      <div className="w-64 bg-neutral-800 border-r border-neutral-700 flex flex-col shrink-0">
        <div className="p-6 border-b border-neutral-700">
          <h1 className="text-xl font-bold flex items-center gap-2">
            <Activity className="text-green-500" />
            LinuxHello
          </h1>
        </div>
        
        <nav className="flex-1 p-4 space-y-1">
          <button 
            onClick={() => setActiveTab('enroll')}
            className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${activeTab === 'enroll' ? 'bg-blue-600 text-white' : 'hover:bg-neutral-700 text-neutral-400'}`}
          >
            <Camera size={20} /> Enroll Face
          </button>
          <button 
            onClick={() => setActiveTab('users')}
            className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${activeTab === 'users' ? 'bg-blue-600 text-white' : 'hover:bg-neutral-700 text-neutral-400'}`}
          >
            <Users size={20} /> User Manager
          </button>
          <button 
            onClick={() => setActiveTab('authtest')}
            className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${activeTab === 'authtest' ? 'bg-blue-600 text-white' : 'hover:bg-neutral-700 text-neutral-400'}`}
          >
            <Terminal size={20} /> Auth Test
          </button>
          <button 
            onClick={() => setActiveTab('pam')}
            className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${activeTab === 'pam' ? 'bg-blue-600 text-white' : 'hover:bg-neutral-700 text-neutral-400'}`}
          >
            <Shield size={20} /> PAM Manager
          </button>
          <button 
            onClick={() => setActiveTab('service')}
            className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${activeTab === 'service' ? 'bg-blue-600 text-white' : 'hover:bg-neutral-700 text-neutral-400'}`}
          >
            <Activity size={20} /> System Services
          </button>
          <button
            onClick={() => setActiveTab('settings')}
            className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${activeTab === 'settings' ? 'bg-blue-600 text-white' : 'hover:bg-neutral-700 text-neutral-400'}`}
          >
            <Settings size={20} /> Configuration
          </button>
          <button
            onClick={() => setActiveTab('logs')}
            className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${activeTab === 'logs' ? 'bg-blue-600 text-white' : 'hover:bg-neutral-700 text-neutral-400'}`}
          >
            <Terminal size={20} /> System Logs
          </button>
        </nav>
      </div>

      {/* Main Content */}
      <div className="flex-1 p-8 overflow-y-auto w-full">
        {renderTabContent()}
      </div>

      {/* Model Download Modal */}
      {showModelModal && (
        <ModelDownloadModal
          onClose={() => setShowModelModal(false)}
          onComplete={() => setShowModelModal(false)}
        />
      )}
    </div>
  )
}

export default App
