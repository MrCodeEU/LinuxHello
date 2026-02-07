import { useState, useCallback } from 'react'
import { getUsers, getConfig, getPAMStatus, getPAMServices, getServiceStatus } from '../wails'
import type { AppConfig, UserResponse, PAMServiceStatus } from '../wails'

export type User = UserResponse

export const useAppData = () => {
  const [users, setUsers] = useState<User[]>([])
  const [config, setConfig] = useState<AppConfig | null>(null)
  const [pamStatus, setPamStatus] = useState('')
  const [pamServices, setPamServices] = useState<PAMServiceStatus[]>([])
  const [serviceInfo, setServiceInfo] = useState({ status: 'unknown', enabled: 'unknown' })

  const fetchUsers = useCallback(async () => {
    try {
      const result = await getUsers()
      setUsers(result || [])
    } catch (e) {
      console.error("Failed to fetch users", e)
    }
  }, [])

  const fetchConfig = useCallback(async () => {
    try {
      const result = await getConfig()
      setConfig(result)
    } catch (e) {
      console.error("Failed to fetch config", e)
    }
  }, [])

  const fetchData = useCallback(async () => {
    const results = await Promise.allSettled([
      getUsers(),
      getConfig(),
      getPAMStatus(),
      getPAMServices(),
      getServiceStatus()
    ])
    if (results[0].status === 'fulfilled') setUsers(results[0].value || [])
    if (results[1].status === 'fulfilled') setConfig(results[1].value)
    if (results[2].status === 'fulfilled') setPamStatus(results[2].value)
    if (results[3].status === 'fulfilled') setPamServices(results[3].value || [])
    if (results[4].status === 'fulfilled') setServiceInfo(results[4].value)
  }, [])

  const fetchServiceStatus = useCallback(async () => {
    try {
      const result = await getServiceStatus()
      setServiceInfo(result)
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
    pamServices,
    serviceInfo,
    fetchData,
    fetchUsers,
    fetchConfig,
    fetchServiceStatus
  }
}
