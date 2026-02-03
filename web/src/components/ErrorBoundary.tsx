import { Component, ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="card p-12 text-center">
          <div className="text-4xl mb-3 opacity-30">!</div>
          <h1 className="text-xl font-semibold mb-1">Something went wrong</h1>
          <p className="text-sm text-muted dark:text-muted-dark mb-4">
            An unexpected error occurred.
          </p>
          <button
            onClick={() => window.location.reload()}
            className="px-4 py-2.5 text-sm font-semibold rounded-lg bg-accent text-gray-900 hover:bg-accent/90 transition-colors"
          >
            Reload page
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
