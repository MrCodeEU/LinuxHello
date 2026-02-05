import React from 'react'
import { Camera, UserPlus, Loader2 } from 'lucide-react'
import { CameraPreview } from './CameraPreview'

interface EnrollmentStatus {
  is_enrolling: boolean
  username: string
  progress: number
  total: number
  message: string
}

interface EnrollmentTabProps {
  enrollName: string
  setEnrollName: (name: string) => void
  enrollStatus: string
  isEnrolling: boolean
  enrollmentProgress: EnrollmentStatus | null
  handleEnroll: () => Promise<void>
}

export const EnrollmentTab: React.FC<EnrollmentTabProps> = ({
  enrollName,
  setEnrollName,
  enrollStatus,
  isEnrolling,
  enrollmentProgress,
  handleEnroll
}) => {
  const getEnrollmentStatusClass = (status: string) => {
    const baseClasses = 'p-3 rounded-lg text-sm'
    if (status.includes('Error') || status.includes('error')) {
      return `${baseClasses} bg-red-900 text-red-300 border border-red-700`
    }
    if (status.includes('Successful') || status.includes('✓')) {
      return `${baseClasses} bg-green-900 text-green-300 border border-green-700`
    }
    return `${baseClasses} bg-blue-900 text-blue-300 border border-blue-700`
  }
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3 mb-6">
        <Camera className="text-blue-500" size={28} />
        <h2 className="text-2xl font-bold text-gray-100">Enroll New User</h2>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Camera Preview */}
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-4">Camera Preview</h3>
          <CameraPreview 
            isActive={enrollName.trim().length > 0 || isEnrolling}
            className="w-full"
          />
        </div>

        {/* Enrollment Controls */}
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-4">User Enrollment</h3>
          <div className="space-y-4">
            <div>
              <label htmlFor="username" className="block text-sm font-medium text-gray-300 mb-2">
                Username
              </label>
              <input
                id="username"
                type="text"
                value={enrollName}
                onChange={(e) => setEnrollName(e.target.value)}
                placeholder="Enter username"
                className="w-full px-4 py-2 bg-neutral-700 border border-neutral-600 rounded-lg text-gray-100 placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                disabled={isEnrolling}
              />
            </div>

            <button
              onClick={handleEnroll}
              disabled={!enrollName.trim() || isEnrolling}
              className="w-full flex items-center justify-center gap-2 px-4 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {isEnrolling ? <Loader2 className="animate-spin" size={20} /> : <UserPlus size={20} />}
              {isEnrolling ? 'Enrolling...' : 'Start Enrollment'}
            </button>

            {/* Enrollment Progress */}
            {enrollmentProgress && enrollmentProgress.is_enrolling && (
              <div className="bg-neutral-700 p-4 rounded-lg border border-neutral-600 space-y-3">
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-300">Progress</span>
                  <span className="text-sm text-blue-400">{enrollmentProgress.progress}/{enrollmentProgress.total}</span>
                </div>
                
                <div className="w-full bg-neutral-600 rounded-full h-2.5">
                  <div 
                    className="bg-blue-600 h-2.5 rounded-full transition-all duration-300 ease-out"
                    style={{ width: `${(enrollmentProgress.progress / enrollmentProgress.total) * 100}%` }}
                  ></div>
                </div>
                
                <div className="text-sm text-gray-300 text-center">
                  {enrollmentProgress.message}
                </div>
              </div>
            )}

            {enrollStatus && (
              <div className={getEnrollmentStatusClass(enrollStatus)}>
                {enrollStatus}
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
        <h3 className="text-lg font-medium text-gray-200 mb-3">Instructions</h3>
        <ul className="space-y-2 text-gray-400 text-sm">
          <li>• Enter your username above to activate the camera preview</li>
          <li>• Look directly at the camera during enrollment</li>
          <li>• Ensure good lighting on your face</li>
          <li>• Hold steady position for a few seconds</li>
          <li>• Multiple samples will be captured automatically</li>
        </ul>
      </div>
    </div>
  )
}