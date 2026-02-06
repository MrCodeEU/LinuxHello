import { useState } from 'react'

export const useAuthTesting = () => {
  const [authTestResult, setAuthTestResult] = useState<any>(null)
  const [isAuthTesting, setIsAuthTesting] = useState(false)
  const [cameraRunning, setCameraRunning] = useState(false)

  const stopCamera = async () => {
    if (!cameraRunning) return
    
    try {
      const stopRes = await fetch('/api/camera/stop', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      })
      if (stopRes.ok) {
        setCameraRunning(false)
      }
    } catch (e) {
      console.error('Failed to stop camera:', e)
    }
  }

  const startCamera = async () => {
    if (cameraRunning) return true
    
    try {
      const startRes = await fetch('/api/camera/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      })
      
      if (startRes.ok) {
        setCameraRunning(true)
        // Give camera time to warm up and clear buffers
        await new Promise(resolve => setTimeout(resolve, 800))
        return true
      }
    } catch (e) {
      console.error('Failed to start camera:', e)
    }
    
    return false
  }

  const handleAuthTest = async () => {
    setIsAuthTesting(true)
    setAuthTestResult(null)
    
    try {
      // Start camera first
      const cameraStarted = await startCamera()
      if (!cameraStarted) {
        setAuthTestResult({ error: 'Failed to start camera', success: false })
        return
      }
      
      const res = await fetch('/api/authtest', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      })
      
      if (res.ok) {
        const result = await res.json()
        setAuthTestResult(result)
      } else {
        const error = await res.text()
        setAuthTestResult({ error, success: false })
      }
      
      // Stop camera after test to prevent buffering
      await stopCamera()
    } catch (error) {
      console.error('Auth test failed:', error)
      setAuthTestResult({ 
        error: `Network error: ${error instanceof Error ? error.message : 'Unknown error'}`, 
        success: false 
      })
      // Ensure camera is stopped on error
      await stopCamera()
    } finally {
      setIsAuthTesting(false)
    }
  }

  return { 
    authTestResult, 
    setAuthTestResult, 
    isAuthTesting, 
    cameraRunning, 
    handleAuthTest 
  }
}