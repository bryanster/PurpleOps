import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState, type ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { ErrorBoundary } from './error-boundary'

function Boom({ shouldThrow = true }: { shouldThrow?: boolean }): ReactNode {
  if (shouldThrow) {
    throw new Error('the child exploded')
  }
  return <p>child rendered</p>
}

describe('ErrorBoundary', () => {
  it('renders the fallback instead of a blank page when a child throws', () => {
    // React logs the caught error itself, which would otherwise bury the real
    // output of a failing test run in noise.
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const onError = vi.fn()

    render(
      <ErrorBoundary onError={onError}>
        <Boom />
      </ErrorBoundary>,
    )

    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('Something broke')
    expect(alert).toHaveTextContent('the child exploded')
    expect(onError).toHaveBeenCalledOnce()
  })

  it('renders its children untouched when nothing throws', () => {
    render(
      <ErrorBoundary>
        <Boom shouldThrow={false} />
      </ErrorBoundary>,
    )

    expect(screen.getByText('child rendered')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('recovers when the cause is gone and the user retries', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const user = userEvent.setup()

    function Flaky(): ReactNode {
      const [fixed, setFixed] = useState(false)
      return (
        <>
          <button
            onClick={() => {
              setFixed(true)
            }}
          >
            fix it
          </button>
          <ErrorBoundary>
            <Boom shouldThrow={!fixed} />
          </ErrorBoundary>
        </>
      )
    }

    render(<Flaky />)
    expect(screen.getByRole('alert')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'fix it' }))
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(screen.getByText('child rendered')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('uses a custom fallback when one is given', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})

    render(
      <ErrorBoundary fallback={(error) => <p>caught: {error.message}</p>}>
        <Boom />
      </ErrorBoundary>,
    )

    expect(screen.getByText('caught: the child exploded')).toBeInTheDocument()
  })
})
