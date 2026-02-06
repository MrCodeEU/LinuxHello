import { useState } from 'react'
import { saveConfig, deleteUser } from '../wails'
import type { AppConfig } from './useAppData'

export const useConfiguration = (fetchConfig?: () => Promise<void>) => {
  const [saveStatus, setSaveStatus] = useState('')

  const handleSaveConfig = async (config: AppConfig | null) => {
    if (!config) return

    setSaveStatus('Saving...')

    try {
      await saveConfig(config)
      setSaveStatus('Settings saved successfully!')
      // Refresh config from server after successful save
      if (fetchConfig) {
        await fetchConfig()
      }
      setTimeout(() => setSaveStatus(''), 5000)
    } catch (error) {
      console.error('Failed to save config:', error)
      setSaveStatus(`Error saving settings: ${error instanceof Error ? error.message : 'Unknown error'}`)
    }
  }

  const handleDeleteUser = async (username: string, fetchData: () => Promise<void>) => {
    if (!confirm(`Delete user ${username}?`)) return

    try {
      await deleteUser(username)
      await fetchData()
    } catch (error) {
      console.error('Failed to delete user:', error)
      alert(`Failed to delete user: ${error instanceof Error ? error.message : 'Unknown error'}`)
    }
  }

  return {
    saveStatus,
    handleSaveConfig,
    handleDeleteUser
  }
}
