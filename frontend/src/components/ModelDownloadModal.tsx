import { useState, useEffect } from 'react'
import { Download, CheckCircle, XCircle, Loader } from 'lucide-react'
import { checkModels, downloadModels, EventsOn, EventsOff } from '../wails'

interface ModelDownloadModalProps {
  onClose: () => void
  onComplete: () => void
}

interface DownloadProgress {
  detection: { progress: number; status: string; message: string }
  recognition: { progress: number; status: string; message: string }
}

export default function ModelDownloadModal({ onClose, onComplete }: ModelDownloadModalProps) {
  const [isDownloading, setIsDownloading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [modelStatus, setModelStatus] = useState<any>(null)
  const [downloadProgress, setDownloadProgress] = useState<DownloadProgress>({
    detection: { progress: 0, status: 'pending', message: '' },
    recognition: { progress: 0, status: 'pending', message: '' }
  })
  const [downloadComplete, setDownloadComplete] = useState(false)

  useEffect(() => {
    checkModelStatus()

    // Listen for download start events
    EventsOn('model:download:start', (data: any) => {
      console.log('Download started:', data)
      setDownloadProgress(prev => ({
        ...prev,
        [data.model]: {
          progress: 0,
          status: 'downloading',
          message: data.message || 'Starting download...'
        }
      }))
    })

    // Listen for progress events
    EventsOn('model:download:progress', (data: any) => {
      console.log('Download progress:', data)
      setDownloadProgress(prev => ({
        ...prev,
        [data.model]: {
          progress: data.progress || 0,
          status: data.status || 'downloading',
          message: data.message || ''
        }
      }))
    })

    EventsOn('model:download:complete', (data: any) => {
      console.log('Download complete:', data)
      setDownloadProgress(prev => ({
        ...prev,
        [data.model]: {
          progress: 100,
          status: 'complete',
          message: data.message || 'Downloaded'
        }
      }))
    })

    EventsOn('model:download:error', (data: any) => {
      console.error('Download error:', data)
      setError(data.message || data.error || 'Download failed')
      setIsDownloading(false)
    })

    return () => {
      EventsOff('model:download:start')
      EventsOff('model:download:progress')
      EventsOff('model:download:complete')
      EventsOff('model:download:error')
    }
  }, [])

  const checkModelStatus = async () => {
    try {
      const status = await checkModels()
      setModelStatus(status)
      
      // If all models are present, close automatically
      if (status.allModelsPresent) {
        setTimeout(() => onComplete(), 1000)
      }
    } catch (err) {
      setError(`Failed to check models: ${err}`)
    }
  }

  const handleDownload = async () => {
    setIsDownloading(true)
    setError(null)
    setDownloadComplete(false)
    
    try {
      await downloadModels()
      
      // Wait a bit for final events to arrive
      setTimeout(async () => {
        await checkModelStatus()
        setDownloadComplete(true)
        setIsDownloading(false)
      }, 500)
    } catch (err) {
      setError(`Download failed: ${err}`)
      setIsDownloading(false)
    }
  }

  const handleFinish = () => {
    onComplete()
  }

  const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i]
  }

  return (
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div className="bg-neutral-900 border border-neutral-700 rounded-2xl shadow-2xl max-w-2xl w-full p-8">
        <div className="flex items-center gap-3 mb-6">
          <Download className="text-blue-500" size={32} />
          <h2 className="text-2xl font-bold text-gray-100">ONNX Models Required</h2>
        </div>

        <p className="text-gray-300 mb-6">
          LinuxHello requires AI models for face detection and recognition. 
          These models are downloaded separately to keep installation packages small.
        </p>

        {modelStatus && !isDownloading && !downloadComplete && (
          <div className="space-y-4 mb-6">
            {/* Detection Model */}
            <div className="bg-neutral-800 p-4 rounded-lg border border-neutral-700">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-medium text-gray-200">Face Detection Model</h3>
                  <p className="text-sm text-gray-400">{modelStatus.detectionModel.name}</p>
                  {modelStatus.detectionModel.exists && (
                    <p className="text-xs text-gray-500 mt-1">
                      {formatSize(modelStatus.detectionModel.size)}
                    </p>
                  )}
                </div>
                {modelStatus.detectionModel.exists ? (
                  <CheckCircle className="text-green-500" size={24} />
                ) : (
                  <XCircle className="text-red-500" size={24} />
                )}
              </div>
            </div>

            {/* Recognition Model */}
            <div className="bg-neutral-800 p-4 rounded-lg border border-neutral-700">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-medium text-gray-200">Face Recognition Model</h3>
                  <p className="text-sm text-gray-400">{modelStatus.recognitionModel.name}</p>
                  {modelStatus.recognitionModel.exists && (
                    <p className="text-xs text-gray-500 mt-1">
                      {formatSize(modelStatus.recognitionModel.size)}
                    </p>
                  )}
                </div>
                {modelStatus.recognitionModel.exists ? (
                  <CheckCircle className="text-green-500" size={24} />
                ) : (
                  <XCircle className="text-red-500" size={24} />
                )}
              </div>
            </div>
          </div>
        )}

        {/* Download Progress */}
        {isDownloading && (
          <div className="space-y-4 mb-6">
            {/* Detection Model Progress - only show if downloading */}
            {downloadProgress.detection.status === 'downloading' && (
              <div className="bg-neutral-800 p-4 rounded-lg border border-neutral-700">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="font-medium text-gray-200">Face Detection Model</h3>
                  {downloadProgress.detection.status === 'complete' ? (
                    <CheckCircle className="text-green-500" size={20} />
                  ) : (
                    <Loader className="text-blue-500 animate-spin" size={20} />
                  )}
                </div>
                <p className="text-xs text-gray-400 mb-2">{downloadProgress.detection.message || 'Downloading...'}</p>
                <div className="w-full bg-neutral-700 rounded-full h-2">
                  <div
                    className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${downloadProgress.detection.progress}%` }}
                  />
                </div>
                <p className="text-xs text-gray-500 mt-1 text-right">{downloadProgress.detection.progress}%</p>
              </div>
            )}

            {/* Recognition Model Progress - only show if downloading */}
            {downloadProgress.recognition.status === 'downloading' && (
              <div className="bg-neutral-800 p-4 rounded-lg border border-neutral-700">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="font-medium text-gray-200">Face Recognition Model</h3>
                  {downloadProgress.recognition.status === 'complete' ? (
                    <CheckCircle className="text-green-500" size={20} />
                  ) : (
                    <Loader className="text-blue-500 animate-spin" size={20} />
                  )}
                </div>
                <p className="text-xs text-gray-400 mb-2">{downloadProgress.recognition.message || 'Downloading...'}</p>
                <div className="w-full bg-neutral-700 rounded-full h-2">
                  <div
                    className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${downloadProgress.recognition.progress}%` }}
                  />
                </div>
                <p className="text-xs text-gray-500 mt-1 text-right">{downloadProgress.recognition.progress}%</p>
              </div>
            )}
          </div>
        )}

        {/* Success Message */}
        {downloadComplete && (
          <div className="bg-green-900/20 border border-green-600 p-4 rounded-lg mb-6">
            <div className="flex items-center gap-3">
              <CheckCircle className="text-green-500" size={24} />
              <div>
                <p className="text-green-300 font-medium">Download Complete!</p>
                <p className="text-green-400 text-sm">All models have been downloaded successfully.</p>
              </div>
            </div>
          </div>
        )}

        {error && (
          <div className="bg-red-900/20 border border-red-600 p-4 rounded-lg mb-6">
            <p className="text-red-400 text-sm">{error}</p>
          </div>
        )}

        {!modelStatus?.allModelsPresent && !isDownloading && !downloadComplete && (
          <div className="bg-blue-900/20 border border-blue-600 p-4 rounded-lg mb-6">
            <p className="text-blue-300 text-sm">
              <strong>Note:</strong> Models will be downloaded from HuggingFace 
              (~200MB total). This may take a few minutes depending on your connection.
            </p>
          </div>
        )}

        <div className="flex gap-3">
          {!modelStatus?.allModelsPresent && !downloadComplete && (
            <button
              onClick={handleDownload}
              disabled={isDownloading}
              className="flex-1 flex items-center justify-center gap-2 px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors font-medium"
            >
              {isDownloading ? (
                <>
                  <Loader className="animate-spin" size={20} />
                  Downloading...
                </>
              ) : (
                <>
                  <Download size={20} />
                  Download Models
                </>
              )}
            </button>
          )}

          {downloadComplete && (
            <button
              onClick={handleFinish}
              className="flex-1 px-6 py-3 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors font-medium"
            >
              Continue to App
            </button>
          )}
          
          {!isDownloading && (
            <button
              onClick={onClose}
              className="px-6 py-3 bg-neutral-700 text-white rounded-lg hover:bg-neutral-600 transition-colors"
            >
              {modelStatus?.allModelsPresent || downloadComplete ? 'Continue' : 'Cancel'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
