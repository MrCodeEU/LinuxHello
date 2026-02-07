import React, { Component, ErrorInfo, ReactNode } from 'react'
import { AlertTriangle, X } from 'lucide-react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
  errorInfo: ErrorInfo | null
}

export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
    errorInfo: null
  }

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error, errorInfo: null }
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('ErrorBoundary caught an error:', error, errorInfo)
    this.setState({
      error,
      errorInfo
    })
  }

  private handleClose = () => {
    this.setState({ hasError: false, error: null, errorInfo: null })
  }

  private handleReload = () => {
    window.location.reload()
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50 p-4">
          <div className="bg-neutral-800 border-2 border-red-600 rounded-xl max-w-2xl w-full max-h-[90vh] overflow-auto">
            <div className="p-6">
              <div className="flex items-start justify-between mb-4">
                <div className="flex items-center gap-3">
                  <AlertTriangle className="text-red-500" size={32} />
                  <h2 className="text-2xl font-bold text-red-400">Application Error</h2>
                </div>
                <button
                  onClick={this.handleClose}
                  className="text-gray-400 hover:text-white transition-colors"
                >
                  <X size={24} />
                </button>
              </div>

              <div className="space-y-4">
                <div className="bg-neutral-900 p-4 rounded-lg border border-neutral-700">
                  <h3 className="text-lg font-semibold text-red-400 mb-2">Error Message:</h3>
                  <p className="text-gray-300 font-mono text-sm">
                    {this.state.error?.message || 'Unknown error'}
                  </p>
                </div>

                {this.state.errorInfo && (
                  <div className="bg-neutral-900 p-4 rounded-lg border border-neutral-700">
                    <h3 className="text-lg font-semibold text-orange-400 mb-2">Component Stack:</h3>
                    <pre className="text-gray-400 font-mono text-xs overflow-x-auto whitespace-pre-wrap">
                      {this.state.errorInfo.componentStack}
                    </pre>
                  </div>
                )}

                {this.state.error?.stack && (
                  <div className="bg-neutral-900 p-4 rounded-lg border border-neutral-700">
                    <h3 className="text-lg font-semibold text-yellow-400 mb-2">Stack Trace:</h3>
                    <pre className="text-gray-400 font-mono text-xs overflow-x-auto whitespace-pre-wrap">
                      {this.state.error.stack}
                    </pre>
                  </div>
                )}

                <div className="flex gap-3">
                  <button
                    onClick={this.handleClose}
                    className="flex-1 px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors"
                  >
                    Close
                  </button>
                  <button
                    onClick={this.handleReload}
                    className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                  >
                    Reload Application
                  </button>
                </div>

                <div className="bg-yellow-900/20 border border-yellow-700 p-4 rounded-lg">
                  <p className="text-yellow-200 text-sm">
                    <strong>Note:</strong> This error has been logged to the console. 
                    If the problem persists, please report this error with the stack trace above.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
