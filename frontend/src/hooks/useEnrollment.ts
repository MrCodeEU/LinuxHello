import { useState, useCallback, useEffect, useRef } from 'react'
import { startEnrollment, EventsOn } from '../wails'
import type { EnrollmentStatus } from '../wails'

export const useEnrollment = (fetchUsers: () => Promise<void>) => {
  const [enrollName, setEnrollName] = useState('')
  const [enrollStatus, setEnrollStatus] = useState('')
  const [isEnrolling, setIsEnrolling] = useState(false)
  const [enrollmentProgress, setEnrollmentProgress] = useState<EnrollmentStatus | null>(null)
  const cleanupRef = useRef<(() => void)[]>([])

  const cleanupEvents = useCallback(() => {
    cleanupRef.current.forEach(fn => fn())
    cleanupRef.current = []
  }, [])

  // Clean up on unmount
  useEffect(() => {
    return () => cleanupEvents()
  }, [cleanupEvents])

  const handleEnroll = async () => {
    if (!enrollName.trim()) return

    setIsEnrolling(true)
    setEnrollStatus('Starting enrollment...')

    try {
      await startEnrollment(enrollName)

      // Subscribe to enrollment events from Go backend
      const cancelStatus = EventsOn('enrollment:status', (status: EnrollmentStatus) => {
        setEnrollmentProgress(status)
        setIsEnrolling(status.is_enrolling)
      })

      const cancelComplete = EventsOn('enrollment:complete', async (result: { success: boolean; error?: string }) => {
        cleanupEvents()

        if (result.success) {
          setEnrollStatus('Enrollment completed successfully!')
          setEnrollName('')
          setIsEnrolling(false)
          await fetchUsers()
          setTimeout(() => setEnrollStatus(''), 3000)
        } else {
          setEnrollStatus(`Error: ${result.error || 'Enrollment failed'}`)
          setIsEnrolling(false)
        }
      })

      cleanupRef.current = [cancelStatus, cancelComplete]
    } catch (error) {
      console.error('Enrollment failed:', error)
      setEnrollStatus(`Error: ${error instanceof Error ? error.message : 'Unknown error'}`)
      setIsEnrolling(false)
    }
  }

  return {
    enrollName,
    setEnrollName,
    enrollStatus,
    setEnrollStatus,
    isEnrolling,
    enrollmentProgress,
    handleEnroll
  }
}
