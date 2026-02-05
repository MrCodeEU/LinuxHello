import React from 'react'
import { Settings, Save, Camera, Shield, Zap, Lock, Database, FileText } from 'lucide-react'

interface SettingsTabProps {
  config: any
  setConfig: (config: any) => void
  saveStatus?: string
  handleSaveConfig: (config: any) => void
}

export const SettingsTab: React.FC<SettingsTabProps> = ({
  config,
  setConfig,
  saveStatus,
  handleSaveConfig
}) => {
  if (!config) return <div className="p-6 text-center text-gray-400">Loading configuration...</div>

  const updateConfig = (section: string, field: string, value: any) => {
    setConfig({
      ...config,
      [section]: {
        ...config[section],
        [field]: value
      }
    })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3 mb-6">
        <Settings className="text-gray-500" size={28} />
        <h2 className="text-2xl font-bold text-gray-100">System Configuration</h2>
      </div>

      {saveStatus && (
        <div className={`p-4 rounded-lg border ${saveStatus.includes('Error') ? 'bg-red-900/30 border-red-800 text-red-400' : 'bg-green-900/30 border-green-800 text-green-400'}`}>
          {saveStatus}
        </div>
      )}

      {/* Camera Settings */}
      <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
        <h3 className="text-lg font-bold border-b border-neutral-700 pb-3 mb-4 flex items-center gap-2">
          <Camera size={18} className="text-blue-400" /> Camera Settings
        </h3>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div className="space-y-4">
            <div>
              <label htmlFor="camera-device" className="block text-sm font-medium text-gray-300 mb-2">Primary Device</label>
              <input
                id="camera-device"
                type="text"
                value={config.camera.device}
                onChange={(e) => updateConfig('camera', 'device', e.target.value)}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none font-mono text-sm"
              />
            </div>
            <div>
              <label htmlFor="ir-device" className="block text-sm font-medium text-gray-300 mb-2">IR Device (Optional)</label>
              <input
                id="ir-device"
                type="text"
                value={config.camera.ir_device || ''}
                onChange={(e) => updateConfig('camera', 'ir_device', e.target.value)}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none font-mono text-sm"
                placeholder="/dev/video1"
              />
            </div>
            <div>
              <label htmlFor="pixel-format" className="block text-sm font-medium text-gray-300 mb-2">Pixel Format</label>
              <select
                id="pixel-format"
                value={config.camera.pixel_format}
                onChange={(e) => updateConfig('camera', 'pixel_format', e.target.value)}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
              >
                <option value="MJPEG">MJPEG</option>
                <option value="YUYV">YUYV</option>
                <option value="RGB24">RGB24</option>
                <option value="GREY">GREY</option>
              </select>
            </div>
          </div>
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label htmlFor="camera-width" className="block text-sm font-medium text-gray-300 mb-2">Width</label>
                <input
                  id="camera-width"
                  type="number"
                  value={config.camera.width}
                  onChange={(e) => updateConfig('camera', 'width', parseInt(e.target.value))}
                  className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>
              <div>
                <label htmlFor="camera-height" className="block text-sm font-medium text-gray-300 mb-2">Height</label>
                <input
                  id="camera-height"
                  type="number"
                  value={config.camera.height}
                  onChange={(e) => updateConfig('camera', 'height', parseInt(e.target.value))}
                  className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
                />
              </div>
            </div>
            <div>
              <label htmlFor="camera-fps" className="block text-sm font-medium text-gray-300 mb-2">Frame Rate (FPS)</label>
              <input
                id="camera-fps"
                type="number"
                value={config.camera.fps}
                onChange={(e) => updateConfig('camera', 'fps', parseInt(e.target.value))}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
            <div className="flex items-center gap-4">
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={config.camera.auto_exposure}
                  onChange={(e) => updateConfig('camera', 'auto_exposure', e.target.checked)}
                  className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
                />
                <span className="text-sm text-gray-300">Auto Exposure</span>
              </label>
            </div>
          </div>
        </div>
      </div>

      {/* Detection and Recognition Settings */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-bold border-b border-neutral-700 pb-3 mb-4 flex items-center gap-2">
            <Zap size={18} className="text-yellow-400" /> Detection Settings
          </h3>
          <div className="space-y-4">
            <div>
              <label htmlFor="detection-confidence" className="block text-sm font-medium text-gray-300 mb-2">
                Confidence Threshold ({config.detection.confidence.toFixed(2)})
              </label>
              <input
                id="detection-confidence"
                type="range"
                min="0"
                max="1"
                step="0.01"
                value={config.detection.confidence}
                onChange={(e) => updateConfig('detection', 'confidence', parseFloat(e.target.value))}
                className="w-full accent-blue-500"
              />
            </div>
            <div>
              <label htmlFor="nms-threshold" className="block text-sm font-medium text-gray-300 mb-2">
                NMS Threshold ({config.detection.nms_threshold.toFixed(2)})
              </label>
              <input
                id="nms-threshold"
                type="range"
                min="0"
                max="1"
                step="0.01"
                value={config.detection.nms_threshold}
                onChange={(e) => updateConfig('detection', 'nms_threshold', parseFloat(e.target.value))}
                className="w-full accent-blue-500"
              />
            </div>
            <div>
              <label htmlFor="max-detections" className="block text-sm font-medium text-gray-300 mb-2">Max Detections</label>
              <input
                id="max-detections"
                type="number"
                value={config.detection.max_detections}
                onChange={(e) => updateConfig('detection', 'max_detections', parseInt(e.target.value))}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
          </div>
        </div>

        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-bold border-b border-neutral-700 pb-3 mb-4 flex items-center gap-2">
            <Shield size={18} className="text-green-400" /> Recognition Settings
          </h3>
          <div className="space-y-4">
            <div>
              <label htmlFor="similarity-threshold" className="block text-sm font-medium text-gray-300 mb-2">
                Similarity Threshold ({config.recognition.similarity_threshold.toFixed(2)})
              </label>
              <input
                id="similarity-threshold"
                type="range"
                min="0"
                max="1"
                step="0.01"
                value={config.recognition.similarity_threshold}
                onChange={(e) => updateConfig('recognition', 'similarity_threshold', parseFloat(e.target.value))}
                className="w-full accent-blue-500"
              />
            </div>
            <div>
              <label htmlFor="enrollment-samples" className="block text-sm font-medium text-gray-300 mb-2">Enrollment Samples</label>
              <input
                id="enrollment-samples"
                type="number"
                min="1"
                max="20"
                value={config.recognition.enrollment_samples}
                onChange={(e) => updateConfig('recognition', 'enrollment_samples', parseInt(e.target.value))}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
          </div>
        </div>
      </div>

      {/* Authentication and Security Settings */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-bold border-b border-neutral-700 pb-3 mb-4 flex items-center gap-2">
            <Lock size={18} className="text-red-400" /> Authentication Settings
          </h3>
          <div className="space-y-4">
            <div>
              <label htmlFor="max-attempts" className="block text-sm font-medium text-gray-300 mb-2">Max Attempts</label>
              <input
                id="max-attempts"
                type="number"
                min="1"
                max="10"
                value={config.auth.max_attempts}
                onChange={(e) => updateConfig('auth', 'max_attempts', parseInt(e.target.value))}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
            <div>
              <label htmlFor="session-timeout" className="block text-sm font-medium text-gray-300 mb-2">Session Timeout (seconds)</label>
              <input
                id="session-timeout"
                type="number"
                min="5"
                max="300"
                value={config.auth.session_timeout}
                onChange={(e) => updateConfig('auth', 'session_timeout', parseInt(e.target.value))}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
            <div>
              <label htmlFor="security-level" className="block text-sm font-medium text-gray-300 mb-2">Security Level</label>
              <select
                id="security-level"
                value={config.auth.security_level}
                onChange={(e) => updateConfig('auth', 'security_level', e.target.value)}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
              >
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
              </select>
            </div>
            <div className="space-y-2">
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={config.auth.fallback_enabled}
                  onChange={(e) => updateConfig('auth', 'fallback_enabled', e.target.checked)}
                  className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
                />
                <span className="text-sm text-gray-300">Enable Password Fallback</span>
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={config.auth.continuous_auth}
                  onChange={(e) => updateConfig('auth', 'continuous_auth', e.target.checked)}
                  className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
                />
                <span className="text-sm text-gray-300">Continuous Authentication</span>
              </label>
            </div>
          </div>
        </div>

        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-bold border-b border-neutral-700 pb-3 mb-4 flex items-center gap-2">
            <Database size={18} className="text-purple-400" /> Storage & Logging
          </h3>
          <div className="space-y-4">
            <div>
              <label htmlFor="max-users" className="block text-sm font-medium text-gray-300 mb-2">Max Users</label>
              <input
                id="max-users"
                type="number"
                min="1"
                max="1000"
                value={config.storage.max_users}
                onChange={(e) => updateConfig('storage', 'max_users', parseInt(e.target.value))}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>
            <div>
              <label htmlFor="log-level" className="block text-sm font-medium text-gray-300 mb-2">Log Level</label>
              <select
                id="log-level"
                value={config.logging.level}
                onChange={(e) => updateConfig('logging', 'level', e.target.value)}
                className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
              >
                <option value="debug">Debug</option>
                <option value="info">Info</option>
                <option value="warn">Warning</option>
                <option value="error">Error</option>
              </select>
            </div>
            <div className="space-y-2">
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={config.storage.backup_enabled}
                  onChange={(e) => updateConfig('storage', 'backup_enabled', e.target.checked)}
                  className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
                />
                <span className="text-sm text-gray-300">Enable Backups</span>
              </label>
            </div>
          </div>
        </div>
      </div>

      {/* Advanced Features */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-bold border-b border-neutral-700 pb-3 mb-4 flex items-center gap-2">
            <Zap size={18} className="text-orange-400" /> Liveness Detection
          </h3>
          <div className="space-y-4">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={config.liveness.enabled}
                onChange={(e) => updateConfig('liveness', 'enabled', e.target.checked)}
                className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Enable Liveness Detection</span>
            </label>
            {config.liveness.enabled && (
              <>
                <div>
                  <label htmlFor="liveness-confidence" className="block text-sm font-medium text-gray-300 mb-2">
                    Confidence Threshold ({config.liveness.confidence_threshold.toFixed(2)})
                  </label>
                  <input
                    id="liveness-confidence"
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    value={config.liveness.confidence_threshold}
                    onChange={(e) => updateConfig('liveness', 'confidence_threshold', parseFloat(e.target.value))}
                    className="w-full accent-blue-500"
                  />
                </div>
                <div className="space-y-2">
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={config.liveness.use_depth_camera}
                      onChange={(e) => updateConfig('liveness', 'use_depth_camera', e.target.checked)}
                      className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
                    />
                    <span className="text-sm text-gray-300">Use Depth Camera</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={config.liveness.use_ir_analysis}
                      onChange={(e) => updateConfig('liveness', 'use_ir_analysis', e.target.checked)}
                      className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
                    />
                    <span className="text-sm text-gray-300">Use IR Analysis</span>
                  </label>
                </div>
              </>
            )}
          </div>
        </div>

        <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
          <h3 className="text-lg font-bold border-b border-neutral-700 pb-3 mb-4 flex items-center gap-2">
            <Shield size={18} className="text-cyan-400" /> Challenge System
          </h3>
          <div className="space-y-4">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={config.challenge.enabled}
                onChange={(e) => updateConfig('challenge', 'enabled', e.target.checked)}
                className="rounded bg-neutral-900 border-neutral-700 text-blue-600 focus:ring-blue-500"
              />
              <span className="text-sm text-gray-300">Enable Challenge-Response</span>
            </label>
            {config.challenge.enabled && (
              <>
                <div>
                  <label htmlFor="challenge-timeout" className="block text-sm font-medium text-gray-300 mb-2">Timeout (seconds)</label>
                  <input
                    id="challenge-timeout"
                    type="number"
                    min="5"
                    max="60"
                    value={config.challenge.timeout_seconds}
                    onChange={(e) => updateConfig('challenge', 'timeout_seconds', parseInt(e.target.value))}
                    className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
                  />
                </div>
                <div>
                  <label htmlFor="required-success" className="block text-sm font-medium text-gray-300 mb-2">Required Successes</label>
                  <input
                    id="required-success"
                    type="number"
                    min="1"
                    max="3"
                    value={config.challenge.required_success}
                    onChange={(e) => updateConfig('challenge', 'required_success', parseInt(e.target.value))}
                    className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
                  />
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Service Configuration */}
      <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
        <h3 className="text-lg font-bold border-b border-neutral-700 pb-3 mb-4 flex items-center gap-2">
          <FileText size={18} className="text-indigo-400" /> Inference Service
        </h3>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div>
            <label htmlFor="inference-address" className="block text-sm font-medium text-gray-300 mb-2">Service Address</label>
            <input
              id="inference-address"
              type="text"
              value={config.inference.address}
              onChange={(e) => updateConfig('inference', 'address', e.target.value)}
              className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none font-mono text-sm"
            />
          </div>
          <div>
            <label htmlFor="inference-timeout" className="block text-sm font-medium text-gray-300 mb-2">Timeout (seconds)</label>
            <input
              id="inference-timeout"
              type="number"
              min="1"
              max="60"
              value={config.inference.timeout}
              onChange={(e) => updateConfig('inference', 'timeout', parseInt(e.target.value))}
              className="w-full bg-neutral-900 border border-neutral-700 rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
            />
          </div>
        </div>
      </div>

      {/* Save Button */}
      <div className="flex justify-end">
        <button
          onClick={() => handleSaveConfig(config)}
          className="bg-green-600 hover:bg-green-500 text-white px-8 py-3 rounded-lg font-bold flex items-center gap-2 shadow-lg transition-colors"
        >
          <Save size={18} /> Save Configuration
        </button>
      </div>
    </div>
  )
}