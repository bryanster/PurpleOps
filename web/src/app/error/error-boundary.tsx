import { Component, type ErrorInfo, type ReactNode } from 'react'

import { Button } from '@/components/ui/button'

interface ErrorBoundaryProps {
  children: ReactNode
  /** Rendered instead of the default fallback. Takes the error and a reset callback. */
  fallback?: (error: Error, reset: () => void) => ReactNode
  /** Called when an error is caught. The default reports to the console. */
  onError?: (error: Error, info: ErrorInfo) => void
}

interface ErrorBoundaryState {
  error: Error | null
}

/**
 * The last line of defence: without one of these, a render-time throw unmounts
 * the whole tree and the user gets a white page with nothing to report.
 *
 * Still a class component — `getDerivedStateFromError` and `componentDidCatch`
 * have no hook equivalent in React 19.
 *
 * Note what this does *not* catch, so nobody assumes otherwise: errors thrown
 * in event handlers, in timeouts, or in promise rejections. Those surface as
 * toasts from the code that raised them.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  override state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    const { onError } = this.props
    if (onError) {
      onError(error, info)
      return
    }
    // Deliberately console.error and not a logging service: the deployment may
    // be air-gapped, and the browser console is the only sink guaranteed to
    // exist. A user reporting a bug is asked for this.
    console.error('Unhandled error in a React subtree', error, info.componentStack)
  }

  private readonly reset = (): void => {
    this.setState({ error: null })
  }

  override render(): ReactNode {
    const { error } = this.state
    const { children, fallback } = this.props

    if (error === null) {
      return children
    }
    if (fallback) {
      return fallback(error, this.reset)
    }

    return (
      <div role="alert" className="mx-auto flex max-w-prose flex-col items-start gap-4 p-8">
        <h1 className="text-2xl font-semibold">Something broke</h1>
        <p className="text-muted-foreground">
          This screen failed to render. Nothing was lost — the error is in the browser console, and
          quoting it makes this much easier to fix.
        </p>
        <pre className="bg-muted w-full overflow-x-auto rounded-lg border p-4 text-sm">
          {error.message}
        </pre>
        <Button onClick={this.reset}>Try again</Button>
      </div>
    )
  }
}
