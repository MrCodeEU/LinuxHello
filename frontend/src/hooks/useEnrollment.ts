import { useState, useCallback, useRef } from 'react'

interface EnrollmentStatus {
  is_enrolling: boolean
  username: string
  progress: number
  total: number
  message: string
}

export const useEnrollment = (fetchUsers: () => Promise<void>) => {
  const [enrollName, setEnrollName] = useState('')
  const [enrollStatus, setEnrollStatus] = useState('')
  const [isEnrolling, setIsEnrolling] = useState(false)
  const [enrollmentProgress, setEnrollmentProgress] = useState<EnrollmentStatus | null>(null)
  const pollIntervalRef = useRef<NodeJS.Timeout | null>(null)

  const pollEnrollmentStatus = useCallback(async () => {
    try {
      const res = await fetch('/api/enroll/status')
      if (res.ok) {
        const status: EnrollmentStatus = await res.json()
        setEnrollmentProgress(status)
        setIsEnrolling(status.is_enrolling)
        
        if (!status.is_enrolling && pollIntervalRef.current) {
          clearInterval(pollIntervalRef.current)
          pollIntervalRef.current = null
          
          // If enrollment was completed successfully
          if (status.message.includes('successfully')) {
            setEnrollStatus('Enrollment completed successfully!')
            setEnrollName('')
            await fetchUsers()
            setTimeout(() => setEnrollStatus(''), 3000)
          }
        }
      }
    } catch (error) {
      console.error('Failed to poll enrollment status:', error)
    }
  }, [fetchUsers])

  const handleEnroll = async () => {
    if (!enrollName.trim()) return
    
    setIsEnrolling(true)
    setEnrollStatus('Starting enrollment...')
    
    try {
      const res = await fetch('/api/enroll', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: enrollName })
      })
      
      if (res.ok) {
        // Start polling for status updates
        pollIntervalRef.current = setInterval(pollEnrollmentStatus, 500)
        pollEnrollmentStatus() // Initial poll
      } else {
        const err = await res.text()
        setEnrollStatus(`Error: ${err}`)
        setIsEnrolling(false)
      }
    } catch (error) {
      console.error('Enrollment failed:', error)
      setEnrollStatus(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`)
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