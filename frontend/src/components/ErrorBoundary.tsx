import { Component, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}
interface State {
  hasError: boolean
  message: string
}

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, message: '' }

  static getDerivedStateFromError(err: Error): State {
    return { hasError: true, message: err.message }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="error-boundary">
          <div className="error-boundary-emoji">😵</div>
          <div className="error-boundary-title">出了点小问题</div>
          <div className="error-boundary-msg">{this.state.message}</div>
          <button className="error-boundary-btn" onClick={() => window.location.reload()}>
            刷新重试
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
