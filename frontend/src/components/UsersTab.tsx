import React from 'react'
import { Users, Trash2, CheckCircle, AlertTriangle } from 'lucide-react'
import type { User } from '../hooks/useAppData'

interface UsersTabProps {
  users: User[]
  handleDeleteUser: (username: string, fetchData: () => Promise<void>) => Promise<void>
  fetchData: () => Promise<void>
}

export const UsersTab: React.FC<UsersTabProps> = ({
  users,
  handleDeleteUser,
  fetchData
}) => {
  const formatLastUsed = (lastUsed?: string) => {
    if (!lastUsed) return 'Never'
    
    try {
      return new Date(lastUsed).toLocaleString()
    } catch {
      return 'Unknown'
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3 mb-6">
        <Users className="text-green-500" size={28} />
        <h2 className="text-2xl font-bold text-gray-100">User Manager</h2>
      </div>

      <div className="bg-neutral-800 p-6 rounded-xl border border-neutral-700">
        <div className="flex justify-between items-center mb-4">
          <h3 className="text-lg font-medium text-gray-200">Enrolled Users</h3>
          <div className="text-sm text-gray-400">
            Total: {users.length} user{users.length === 1 ? '' : 's'}
          </div>
        </div>

        {users.length === 0 ? (
          <div className="text-center py-8 text-gray-400">
            <Users size={48} className="mx-auto mb-4 opacity-50" />
            <p>No users enrolled yet</p>
            <p className="text-sm">Go to the Enrollment tab to add users</p>
          </div>
        ) : (
          <div className="space-y-3">
            {users.map((user) => (
              <div
                key={user.username}
                className="flex items-center justify-between p-4 bg-neutral-700 rounded-lg border border-neutral-600"
              >
                <div className="flex items-center space-x-4">
                  <div className="flex items-center space-x-2">
                    {user.active ? (
                      <CheckCircle className="text-green-500" size={20} />
                    ) : (
                      <AlertTriangle className="text-yellow-500" size={20} />
                    )}
                    <span className="font-medium text-gray-200">{user.username}</span>
                  </div>
                  <div className="text-sm text-gray-400">
                    {user.samples} sample{user.samples === 1 ? '' : 's'}
                  </div>
                  <div className="text-sm text-gray-400">
                    Last used: {formatLastUsed(user.last_used)}
                  </div>
                </div>
                <button
                  onClick={() => handleDeleteUser(user.username, fetchData)}
                  className="p-2 text-red-400 hover:text-red-300 hover:bg-red-900/30 rounded transition-colors"
                  title={`Delete ${user.username}`}
                >
                  <Trash2 size={16} />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}