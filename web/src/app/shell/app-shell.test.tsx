import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { describe, expect, it } from 'vitest'

import { THEME_STORAGE_KEY } from '@/app/theme/theme'
import { ThemeProvider } from '@/app/theme/theme-provider'
import { setSystemTheme } from '@/test/setup'

import { AppShell } from './app-shell'

function renderShell(): void {
  render(
    <ThemeProvider>
      <MemoryRouter initialEntries={['/system/version']}>
        <Routes>
          <Route element={<AppShell />}>
            <Route path="/system/version" element={<p>version screen</p>} />
            <Route path="/system/health" element={<p>health screen</p>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </ThemeProvider>,
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

    const reachable: (Element | null)[] = []
    for (let i = 0; i < 6; i++) {
      await user.tab()
      reachable.push(document.activeElement)
    }

    expect(reachable).toContain(screen.getByRole('link', { name: 'PurpleOps' }))
    expect(reachable).toContain(screen.getByRole('button', { name: /^Theme:/ }))
    expect(reachable).toContain(screen.getByRole('button', { name: 'Account' }))
    expect(reachable).toContain(screen.getByRole('link', { name: 'Version' }))
    expect(reachable).toContain(screen.getByRole('link', { name: 'Health' }))
  })
})
