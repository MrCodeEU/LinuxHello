import * as App from '../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'

export type { UserResponse, EnrollmentStatus, AuthTestResult, ServiceInfo, LogEntry, AppConfig } from '../wailsjs/go/main/App'

// User management
export const getUsers = () => App.GetUsers()
export const deleteUser = (username: string) => App.DeleteUser(username)

// Configuration
export const getConfig = () => App.GetConfig()
export const saveConfig = (cfg: App.AppConfig) => App.SaveConfig(cfg)

// Enrollment
export const startEnrollment = (username: string) => App.StartEnrollment(username)
export const getEnrollmentStatus = () => App.GetEnrollmentStatus()
export const cancelEnrollment = () => App.CancelEnrollment()

// Auth testing
export const runAuthTest = () => App.RunAuthTest()

// Camera
export const startCamera = () => App.StartCamera()
export const stopCamera = () => App.StopCamera()
export const startCameraStream = () => App.StartCameraStream()
export const stopCameraStream = () => App.StopCameraStream()

// Service management
export const getServiceStatus = () => App.GetServiceStatus()
export const controlService = (action: string) => App.ControlService(action)

// PAM
export const getPAMStatus = () => App.GetPAMStatus()
export const pamAction = (action: string, service: string) => App.PAMAction(action, service)
export const pamToggle = (enable: boolean) => App.PAMToggle(enable)

// Logs
export const getLogs = (count: number) => App.GetLogs(count)
export const downloadLogs = () => App.DownloadLogs()

// Event subscriptions
export { EventsOn, EventsOff }
