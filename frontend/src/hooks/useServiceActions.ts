import { useState } from 'react'

export const useServiceActions = (fetchData: () => Promise<void>, fetchServiceStatus: () => Promise<void>) => {
  const [commandOutput, setCommandOutput] = useState('')
  const [isProcessing, setIsProcessing] = useState(false)

  const handlePAMAction = async (action: string, service?: string) => {
    setIsProcessing(true)
    setCommandOutput(`Running PAM action: ${action} ${service || ''}...`)
    
    try {
      const res = await fetch('/api/pam', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action, service })
      })
      const out = await res.text()
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
      const res = await fetch('/api/pam/manage', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: enable ? 'enable' : 'disable' })
      })
      
      if (res.ok) {
        const data = await res.json()
        setCommandOutput(data.message)
        await fetchData()
      } else {
        const error = await res.text()
        setCommandOutput(`Error: ${error}`)
      }
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
      const res = await fetch('/api/service', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action })
      })
      const out = await res.text()
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