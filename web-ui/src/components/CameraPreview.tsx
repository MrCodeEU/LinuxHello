import React, { useRef, useEffect, useState } from 'react'
import { Camera, CameraOff } from 'lucide-react'

interface CameraPreviewProps {
  isActive?: boolean
  showOverlay?: boolean
  overlayData?: {
    image_data?: string
    bounding_boxes?: Array<{
      x: number
      y: number
      width: number
      height: number
      confidence: number
    }>
  }
  className?: string
}

export const CameraPreview: React.FC<CameraPreviewProps> = ({
  isActive = false,
  showOverlay = false,
  overlayData,
  className = ''
}) => {
  const imgRef = useRef<HTMLImageElement>(null)
  const overlayCanvasRef = useRef<HTMLCanvasElement>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)

  // Camera stream URL
  const streamUrl = isActive ? '/api/stream' : ''

  useEffect(() => {
    if (!isActive || !imgRef.current) {
      setIsLoading(false)
      return
    }

    const img = imgRef.current
    
    const handleLoad = () => {
      setIsLoading(false)
      setHasError(false)
    }

    const handleError = () => {
      setIsLoading(false)
      setHasError(true)
    }

    img.addEventListener('load', handleLoad)
    img.addEventListener('error', handleError)

    // Start loading the stream
    img.src = streamUrl

    return () => {
      img.removeEventListener('load', handleLoad)
      img.removeEventListener('error', handleError)
      img.src = ''
    }
  }, [isActive, streamUrl])

  // Draw bounding boxes overlay
  useEffect(() => {
    if (!showOverlay || !overlayData || !overlayCanvasRef.current) {
      return
    }

    const canvas = overlayCanvasRef.current
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    // Clear canvas
    ctx.clearRect(0, 0, canvas.width, canvas.height)

    // If we have a base64 image, draw it first
    if (overlayData.image_data) {
      const img = new Image()
      img.onload = () => {
        // Set canvas size to match image
        canvas.width = img.width
        canvas.height = img.height
        
        // Draw the image
        ctx.drawImage(img, 0, 0)
        
        // Draw bounding boxes
        drawBoundingBoxes(ctx, overlayData.bounding_boxes || [])
      }
      img.src = `data:image/jpeg;base64,${overlayData.image_data}`
    } else if (overlayData.bounding_boxes && imgRef.current) {
      // Just draw bounding boxes over the video stream
      const img = imgRef.current
      canvas.width = img.naturalWidth || img.width
      canvas.height = img.naturalHeight || img.height
      drawBoundingBoxes(ctx, overlayData.bounding_boxes)
    }
  }, [showOverlay, overlayData])

  const drawBoundingBoxes = (ctx: CanvasRenderingContext2D, boxes: Array<{ x: number; y: number; width: number; height: number; confidence: number }>) => {
    ctx.strokeStyle = '#00ff00'
    ctx.lineWidth = 3
    ctx.font = '16px Arial'
    ctx.fillStyle = '#00ff00'

    boxes.forEach((box) => {
      // Draw rectangle
      ctx.strokeRect(box.x, box.y, box.width, box.height)
      
      // Draw confidence label
      const label = `${(box.confidence * 100).toFixed(1)}%`
      const labelWidth = ctx.measureText(label).width
      
      // Background for label
      ctx.fillStyle = 'rgba(0, 0, 0, 0.7)'
      ctx.fillRect(box.x, box.y - 25, labelWidth + 10, 20)
      
      // Label text
      ctx.fillStyle = '#00ff00'
      ctx.fillText(label, box.x + 5, box.y - 8)
    })
  }

  const renderContent = () => {
    if (!isActive) {
      return (
        <div className="flex flex-col items-center justify-center h-full text-gray-400">
          <CameraOff size={48} className="mb-4" />
          <p>Camera Inactive</p>
        </div>
      )
    }

    if (hasError) {
      return (
        <div className="flex flex-col items-center justify-center h-full text-red-400">
          <CameraOff size={48} className="mb-4" />
          <p>Failed to load camera stream</p>
        </div>
      )
    }

    if (isLoading) {
      return (
        <div className="flex flex-col items-center justify-center h-full text-gray-400">
          <Camera size={48} className="mb-4 animate-pulse" />
          <p>Starting camera...</p>
        </div>
      )
    }

    return (
      <div className="relative w-full h-full">
        <img
          ref={imgRef}
          src={streamUrl}
          alt="Camera Preview"
          className="w-full h-full object-cover rounded-lg"
          style={{ display: showOverlay && overlayData?.image_data ? 'none' : 'block' }}
        />
        {showOverlay && (
          <canvas
            ref={overlayCanvasRef}
            className="absolute top-0 left-0 w-full h-full rounded-lg"
            style={{ 
              display: overlayData?.image_data || overlayData?.bounding_boxes ? 'block' : 'none',
              objectFit: 'cover'
            }}
          />
        )}
      </div>
    )
  }

  return (
    <div className={`bg-neutral-900 rounded-lg border border-neutral-700 ${className}`}>
      <div className="aspect-video w-full min-h-[200px] flex items-center justify-center">
        {renderContent()}
      </div>
    </div>
  )
}