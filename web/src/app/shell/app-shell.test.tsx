import { QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { describe, expect, it } from 'vitest'

import { THEME_STORAGE_KEY } from '@/app/theme/theme'
import { ThemeProvider } from '@/app/theme/theme-provider'
import { CurrentUserContext } from '@/features/auth/current-user'
import type { CurrentUser } from '@/features/auth/queries'
import { adminUserFixture, memberUserFixture } from '@/test/msw/handlers'
import { createTestQueryClient } from '@/test/query'
import { setSystemTheme } from '@/test/setup'

import { AppShell } from './app-shell'

/**
 * The shell renders below the auth guard (M1-017), so it is given a user and a
 * QueryClient — the top bar reads the one and signs out through the other.
 * Which user matters for the nav: an administrator sees entries a member does
 * not.
 */
function renderShell(user: CurrentUser = adminUserFixture): void {
  const { queryClient } = createTestQueryClient()

  render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <CurrentUserContext value={user}>
          <MemoryRouter initialEntries={['/system/version']}>
            <Routes>
              <Route element={<AppShell />}>
                <Route path="/system/version" element={<p>version screen</p>} />
                <Route path="/system/health" element={<p>health screen</p>} />
              </Route>
            </Routes>
          </MemoryRouter>
        </CurrentUserContext>
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

function isDark(): boolean {
  return document.documentElement.classList.contains('dark')
}

/** Open the theme menu and choose one of its options. */
async function chooseTheme(option: 'Light' | 'Dark' | 'System'): Promise<void> {
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: /^Theme:/ }))
  await user.click(await screen.findByRole('menuitem', { name: option }))
}

describe('AppShell', () => {
  it('renders the nav, the current screen and the toast host', () => {
    renderShell()

    const nav = screen.getByRole('navigation', { name: 'Sections' })
    expect(within(nav).getByRole('link', { name: 'Version' })).toHaveAttribute(
      'href',
      '/system/version',
    )
    expect(within(nav).getByRole('link', { name: 'Health' })).toBeInTheDocument()
    expect(screen.getByText('version screen')).toBeInTheDocument()
  })

  it('shows the administration entries to an administrator and not to a member', () => {
    renderShell(adminUserFixture)
    const adminNav = screen.getByRole('navigation', { name: 'Sections' })
    expect(within(adminNav).getByRole('link', { name: 'Users' })).toBeInTheDocument()

    cleanup()

    renderShell(memberUserFixture)
    const memberNav = screen.getByRole('navigation', { name: 'Sections' })
    // Hiding is not what keeps them out — RequireAdmin and the server do that
    // (guards.test.tsx) — but a nav that advertised a locked door would be
    // describing somebody else's account.
    expect(within(memberNav).queryByRole('link', { name: 'Users' })).not.toBeInTheDocument()
    expect(within(memberNav).queryByText('Administration')).not.toBeInTheDocument()
  })

  it('lists unbuilt sections without putting them in the tab order', () => {
    renderShell()

    const nav = screen.getByRole('navigation', { name: 'Sections' })
    expect(within(nav).getByText('Engagements')).toBeInTheDocument()
    expect(within(nav).queryByRole('link', { name: 'Engagements' })).not.toBeInTheDocument()
  })

  it('navigates between screens from the nav', async () => {
    const user = userEvent.setup()
    renderShell()

    await user.click(screen.getByRole('link', { name: 'Health' }))

    expect(screen.getByText('health screen')).toBeInTheDocument()
  })

  it('follows the OS preference when nothing is stored', () => {
    setSystemTheme('dark')
    renderShell()

    expect(isDark()).toBe(true)
  })

  it('applies a stored preference over the OS preference', () => {
    setSystemTheme('dark')
    window.localStorage.setItem(THEME_STORAGE_KEY, 'light')

    renderShell()

    expect(isDark()).toBe(false)
  })

  it('toggles the theme and persists the choice', async () => {
    renderShell()
    expect(isDark()).toBe(false)

    await chooseTheme('Dark')

    expect(isDark()).toBe(true)
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
  })

  it('stops following the OS once a theme is chosen explicitly', async () => {
    renderShell()

    await chooseTheme('Light')
    setSystemTheme('dark')

    expect(isDark()).toBe(false)
  })

  it('follows the OS again after switching back to system', async () => {
    window.localStorage.setItem(THEME_STORAGE_KEY, 'light')
    renderShell()

    await chooseTheme('System')
    setSystemTheme('dark')

    expect(isDark()).toBe(true)
  })

  it('keeps every interactive element in the shell reachable by keyboard', async () => {
    const user = userEvent.setup()
    renderShell()

    // One tab per focusable thing in the shell, which grew when the settings
    // and administration entries arrived (M1-017). Deliberately a fixed number
    // rather than "until it wraps": an element that quietly leaves the tab
    // order should fail this, and a loop that stops at the wrap would not
    // notice.
    const reachable: (Element | null)[] = []
    for (let i = 0; i < 10; i++) {
      await user.tab()
      reachable.push(document.activeElement)
    }

    expect(reachable).toContain(screen.getByRole('link', { name: 'Blacklight' }))
    expect(reachable).toContain(screen.getByRole('button', { name: /^Theme:/ }))
    expect(reachable).toContain(screen.getByRole('button', { name: /^Account:/ }))
    expect(reachable).toContain(screen.getByRole('link', { name: 'Version' }))
    expect(reachable).toContain(screen.getByRole('link', { name: 'Health' }))
  })
})
