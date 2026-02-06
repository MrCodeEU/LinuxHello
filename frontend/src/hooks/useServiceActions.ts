import { useState } from 'react'
import { pamAction, pamToggle, controlService } from '../wails'

export const useServiceActions = (fetchData: () => Promise<void>, fetchServiceStatus: () => Promise<void>) => {
  const [commandOutput, setCommandOutput] = useState('')
  const [isProcessing, setIsProcessing] = useState(false)

  const handlePAMAction = async (action: string, service?: string) => {
    setIsProcessing(true)
    setCommandOutput(`Running PAM action: ${action} ${service || ''}...`)

    try {
      const out = await pamAction(action, service || '')
      setCommandOutput(out)
      await fetchData()
    } catch (e) {
      setCommandOutput(`Error: ${e}`)
    } finally {
      setIsProcessing(false)
    }
  }

  const handlePAMToggle = async (enable: boolean) => {
    setIsProcessing(true)
    setCommandOutput(`${enable ? 'Enabling' : 'Disabling'} PAM module for sudo...`)

    try {
      const out = await pamToggle(enable)
      setCommandOutput(out || `PAM module ${enable ? 'enabled' : 'disabled'} successfully`)
      await fetchData()
    } catch (err) {
      setCommandOutput(`Error: ${err}`)
    } finally {
      setIsProcessing(false)
    }
  }

  const handleServiceAction = async (action: string) => {
    setIsProcessing(true)
    setCommandOutput(`Running service action: ${action}...`)

    try {
      const out = await controlService(action)
      setCommandOutput(out || "Action completed successfully.")
      await fetchServiceStatus()
    } catch (e) {
      setCommandOutput(`Error: ${e}`)
    } finally {
      setIsProcessing(false)
    }
  }

  return {
    commandOutput,
    isProcessing,
    handlePAMAction,
    handlePAMToggle,
    handleServiceAction
  }
}
