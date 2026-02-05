import { useState } from 'react'
import type { AppConfig } from './useAppData'

export const useConfiguration = (fetchConfig?: () => Promise<void>) => {
  const [showPreview, setShowPreview] = useState(false)
  const [saveStatus, setSaveStatus] = useState('')

  const handleSaveConfig = async (config: AppConfig | null) => {
    if (!config) return
    
    setSaveStatus('Saving...')
    
    try {
      const res = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      })
      
      if (res.ok) {
        setSaveStatus('Settings saved successfully!')
        // Refresh config from server after successful save
        if (fetchConfig) {
          await fetchConfig()
        }
        setTimeout(() => setSaveStatus(''), 5000)
      } else {
        const err = await res.text()
        setSaveStatus(`Error saving settings: ${err}`)
      }
    } catch (error) {
      console.error('Failed to save config:', error)
      setSaveStatus(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`)
    }
  }

  const handleDeleteUser = async (username: string, fetchData: () => Promise<void>) => {
    if (!confirm(`Delete user ${username}?`)) return
    
    try {
      await fetch(`/api/users/${username}`, { method: 'DELETE' })
      await fetchData()
    } catch (error) {
      console.error('Failed to delete user:', error)
      alert(`Failed to delete user: ${error instanceof Error ? error.message : 'Unknown error'}`)
    }
  }

  return {
    showPreview,
    setShowPreview,
    saveStatus,
    handleSaveConfig,
    handleDeleteUser
  }
}