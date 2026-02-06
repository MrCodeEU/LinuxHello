import { useState, useCallback } from 'react'

interface User {
  username: string
  samples: number
  last_used?: string
  active: boolean
}

interface AppConfig {
  inference: { address: string; timeout: number }
  camera: { device: string; ir_device: string; width: number; height: number; fps: number; pixel_format: string; auto_exposure: boolean }
  recognition: { similarity_threshold: number; enrollment_samples: number }
  liveness: { enabled: boolean; confidence_threshold: number }
  auth: { max_attempts: number; lockout_duration: number; fallback_enabled: boolean }
  challenge: { enabled: boolean; challenge_types: string[]; max_challenges: number; timeout: number }
  lockout: { enabled: boolean; max_failures: number; lockout_duration: number; progressive_lockout: boolean }
  storage: { data_dir: string; database_path: string }
  logging: { level: string }
}

export type { User, AppConfig }

export const useAppData = () => {
  const [users, setUsers] = useState<User[]>([])
  const [config, setConfig] = useState<AppConfig | null>(null)
  const [pamStatus, setPamStatus] = useState('')
  const [serviceInfo, setServiceInfo] = useState({ status: 'unknown', enabled: 'unknown' })

  const fetchUsers = useCallback(async () => {
    try {
      const res = await fetch('/api/users')
      if (res.ok) setUsers(await res.json())
    } catch (e) {
      console.error("Failed to fetch users", e)
    }
  }, [])

  const fetchConfig = useCallback(async () => {
    try {
      const res = await fetch('/api/config')
      if (res.ok) setConfig(await res.json())
    } catch (e) {
      console.error("Failed to fetch config", e)
    }
  }, [])

  const fetchData = useCallback(async () => {
    try {
      const [uRes, cRes, pRes, sRes] = await Promise.all([
        fetch('/api/users'),
        fetch('/api/config'),
        fetch('/api/pam'),
        fetch('/api/service')
      ])
      if (uRes.ok) setUsers(await uRes.json())
      if (cRes.ok) setConfig(await cRes.json())
      if (pRes.ok) setPamStatus(await pRes.text())
      if (sRes.ok) setServiceInfo(await sRes.json())
    } catch (e) {
      console.error("Failed to fetch data", e)
    }
  }, [])

  const fetchServiceStatus = useCallback(async () => {
    try {
      const res = await fetch('/api/service')
      if (res.ok) setServiceInfo(await res.json())
    } catch (e) {
      console.error("Failed to fetch service status", e)
    }
  }, [])

  return { 
    users, 
    setUsers, 
    config, 
    setConfig, 
    pamStatus, 
    serviceInfo, 
    fetchData,
    fetchUsers,
    fetchConfig,
    fetchServiceStatus 
  }
}