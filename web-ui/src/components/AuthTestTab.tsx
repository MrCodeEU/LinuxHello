import React from 'react'
import { Terminal, Play, Loader2, CheckCircle, AlertTriangle } from 'lucide-react'
import { CameraPreview } from './CameraPreview'

interface AuthTestTabProps {
  authTestResult: any
  isAuthTesting: boolean
  handleAuthTest: () => Promise<void>
}

export const AuthTestTab: React.FC<AuthTestTabProps> = ({
  authTestResult,
  isAuthTesting,
  handleAuthTest
}) => {
  const renderAuthResult = () => {
    if (!authTestResult) return null

    const { success, username, confidence, error, processing_time } = authTestResult

    if (error) {
      return (
        <div className="p-4 bg-red-900 text-red-300 border border-red-700 rounded-lg">
          <div className="flex items-center gap-2 mb-2">
            <AlertTriangle size={20} />
            <span className="font-medium">Authentication Failed</span>
          </div>
          <p className="text-sm">{error}</p>
        </div>
      )
    }

    if (success) {
      return (
        <div className="p-4 bg-green-900 text-green-300 border border-green-700 rounded-lg">
          <div className="flex items-center gap-2 mb-2">
            <CheckCircle size={20} />
            <span className="font-medium">Authentication Successful</span>
          </div>
          <div className="text-sm space-y-1">
            <p><strong>User:</strong> {username}</p>
            <p><strong>Confidence:</strong> {(confidence * 100).toFixed(1)}%</p>
            {processing_time && (
              <p><strong>Processing Time:</strong> {processing_time}ms</p>
            )}
          </div>
        </div>
      )
    }

    return (
      <div className="p-4 bg-yellow-900 text-yellow-300 border border-yellow-700 rounded-lg">
        <div className="flex items-center gap-2 mb-2">
          <AlertTriangle size={20} />
          <span className="font-medium">Authentication Failed</span>
        </div>
        <p className="text-sm">No face recognized or confidence too low</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3 mb-6">
        <Terminal className="text-purple-500" size={28} />
        <h2 className="text-2xl font-bold text-gray-100">Authentication Test</h2>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Camera Preview and Controls */}
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-4">Camera Preview</h3>
          <div className="space-y-4">
            <CameraPreview 
              isActive={true}
              className="w-full"
            />

            <p className="text-gray-300 text-sm">
              Test the face authentication system by looking at the camera. 
              The system will attempt to recognize your face and authenticate you.
            </p>

            <button
              onClick={handleAuthTest}
              disabled={isAuthTesting}
              className="w-full flex items-center justify-center gap-2 px-6 py-3 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {isAuthTesting ? (
                <>
                  <Loader2 className="animate-spin" size={20} />
                  Testing Authentication...
                </>
              ) : (
                <>
                  <Play size={20} />
                  Start Auth Test
                </>
              )}
            </button>
          </div>
        </div>

        {/* Test Results and Analysis */}
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-medium text-gray-200 mb-4">Test Results</h3>
          
          {/* Auth Result */}
          {renderAuthResult()}
          
          {/* Analysis Image with Bounding Boxes */}
          {authTestResult && (authTestResult.image_data || authTestResult.bounding_boxes) && (
            <div className="mt-4">
              <h4 className="text-md font-medium text-gray-300 mb-2">Analysis</h4>
              <CameraPreview
                isActive={false}
                showOverlay={true}
                overlayData={{
                  image_data: authTestResult.image_data,
                  bounding_boxes: authTestResult.bounding_boxes
                }}
                className="w-full"
              />
              {authTestResult.bounding_boxes && authTestResult.bounding_boxes.length > 0 && (
                <p className="text-gray-400 text-xs mt-2">
                  Detected {authTestResult.bounding_boxes.length} face(s) with bounding boxes
                </p>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
        <h3 className="text-lg font-medium text-gray-200 mb-3">Test Instructions</h3>
        <ul className="space-y-2 text-gray-400 text-sm">
          <li>• Ensure you have enrolled your face first</li>
          <li>• Look directly at the camera</li>
          <li>• Maintain good lighting conditions</li>
          <li>• Keep your face centered and steady</li>
          <li>• Wait for the test to complete</li>
          <li>• Check the analysis panel for detected faces and confidence scores</li>
        </ul>
      </div>
    </div>
  )
}