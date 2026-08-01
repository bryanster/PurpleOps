import '@testing-library/jest-dom/vitest'

import { act, cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll, beforeEach, vi } from 'vitest'

import { DARK_QUERY } from '@/app/theme/theme'

import { server } from './msw/server'

/**
 * Every test runs against the MSW fake server (`src/test/msw/handlers.ts`).
 *
 * `onUnhandledRequest: 'error'` is the point: a request to an endpoint no
 * handler describes fails the test rather than hanging or silently returning
 * nothing, so a component that starts calling something new cannot do it
 * unnoticed.
 */
beforeAll(() => {
  server.listen({ onUnhandledRequest: 'error' })
})

afterAll(() => {
  server.close()
})

const noop = (): void => {}

/**
 * Radix's menus and dialogs call these during interaction and jsdom does not
 * implement them — lib.dom types them as always present, which is true of a
 * real browser and not of this one. They are no-ops: the tests assert what ends
 * up in the DOM, not how the pointer got there.
 */
Element.prototype.hasPointerCapture = () => false
Element.prototype.setPointerCapture = noop
Element.prototype.releasePointerCapture = noop
Element.prototype.scrollIntoView = noop

/**
 * jsdom implements no media queries at all, and both Radix and our theme code
 * call matchMedia during render. Without this stub every component test throws
 * before it asserts anything.
 *
 * An EventTarget rather than an object of vi.fn()s, so that the change events
 * setSystemTheme dispatches reach listeners the way the real thing does.
 */
/** What the OS is currently asking for. Reset to light before each test. */
let systemPrefersDark = false

class FakeMediaQueryList extends EventTarget {
  matches: boolean
  onchange = null

  constructor(readonly media: string) {
    super()
    this.matches = media === DARK_QUERY && systemPrefersDark
  }

  /** Deprecated half of the interface. Present because the type demands it. */
  readonly addListener = noop
  readonly removeListener = noop
}

let mediaQueryLists: FakeMediaQueryList[] = []

/**
 * Change what the operating system is asking for, and tell anything listening.
 *
 * The flag is kept separately from the lists so this works both before a render
 * — where it decides what the first matchMedia call answers — and after one,
 * where it also fires a change event at the listeners already registered.
 *
 * The dispatch is wrapped in act() because a listener will set React state, and
 * an update outside act is both a warning and, sometimes, not flushed by the
 * time the assertion runs.
 */
export function setSystemTheme(theme: 'light' | 'dark'): void {
  systemPrefersDark = theme === 'dark'

  act(() => {
    for (const list of mediaQueryLists) {
      if (list.media !== DARK_QUERY) {
        continue
      }
      list.matches = systemPrefersDark
      list.dispatchEvent(Object.assign(new Event('change'), { matches: list.matches }))
    }
  })
}

beforeEach(() => {
  systemPrefersDark = false
  mediaQueryLists = []
  window.localStorage.clear()
  document.documentElement.className = ''

  vi.stubGlobal(
    'matchMedia',
    vi.fn((query: string): MediaQueryList => {
      const list = new FakeMediaQueryList(query)
      mediaQueryLists.push(list)
      return list
    }),
  )
})

afterEach(() => {
  cleanup()
  server.resetHandlers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})
