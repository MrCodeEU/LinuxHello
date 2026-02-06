import { useState } from 'react'
import { startCamera as wailsStartCamera, stopCamera as wailsStopCamera, runAuthTest } from '../wails'
import type { AuthTestResult } from '../wails'

export const useAuthTesting = () => {
  const [authTestResult, setAuthTestResult] = useState<AuthTestResult | null>(null)
  const [isAuthTesting, setIsAuthTesting] = useState(false)
  const [cameraRunning, setCameraRunning] = useState(false)

  const stopCamera = async () => {
    if (!cameraRunning) return

    try {
      await wailsStopCamera()
      setCameraRunning(false)
    } catch (e) {
      console.error('Failed to stop camera:', e)
    }
  }

  const startCamera = async () => {
    if (cameraRunning) return true

    try {
      await wailsStartCamera()
      setCameraRunning(true)
      return true
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
        setAuthTestResult({ error: 'Failed to start camera', success: false, liveness_passed: false, faces_detected: 0 })
        return
      }

      const result = await runAuthTest()
      setAuthTestResult(result)
    } catch (error) {
      console.error('Auth test failed:', error)
      setAuthTestResult({
        error: `Error: ${error instanceof Error ? error.message : 'Unknown error'}`,
        success: false,
        liveness_passed: false,
        faces_detected: 0
      })
    } finally {
      setIsAuthTesting(false)
      await stopCamera()
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
