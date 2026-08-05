import { describe, expect, it } from 'vitest'

import { DEFAULT_LANDING, loginUrlFor, returnToFor, safeReturnTo } from './return-to'

/**
 * The open-redirect check, as its own tests — the same shape
 * `internal/authn/returnto` has on the server, and for the same reason: this is
 * the rule, and it should be provable without going through a screen that
 * happens to call it.
 *
 * Every rejection below is a way that a login page becomes a credible phishing
 * page on your own domain.
 */
describe('safeReturnTo', () => {
  it('accepts a path within this application', () => {
    for (const safe of [
      '/',
      '/settings/account',
      '/admin/users?status=active',
      '/engagements/018f3b2c#step-4',
    ]) {
      expect(safeReturnTo(safe)).toBe(safe)
    }
  })

  it('falls back for anything that could leave this origin', () => {
    for (const unsafe of [
      'https://evil.example/phish',
      '//evil.example/phish',
      'http:/evil.example',
      '\\evil.example',
      '/settings\\..\\..',
      'javascript:alert(1)',
      'settings/account',
      '/settings\nLocation: https://evil.example',
      `/${'x'.repeat(1000)}`,
    ]) {
      expect(safeReturnTo(unsafe)).toBe(DEFAULT_LANDING)
    }
  })

  it('falls back for an absent value', () => {
    expect(safeReturnTo(null)).toBe(DEFAULT_LANDING)
    expect(safeReturnTo(undefined)).toBe(DEFAULT_LANDING)
    expect(safeReturnTo('')).toBe(DEFAULT_LANDING)
  })
})

describe('returnToFor and loginUrlFor', () => {
  it('round-trips a location through the login URL', () => {
    const path = returnToFor({
      pathname: '/admin/users',
      search: '?status=disabled',
      hash: '#row-3',
    })

    const url = loginUrlFor(path)
    const encoded = new URLSearchParams(url.split('?')[1]).get('return_to')

    expect(safeReturnTo(encoded)).toBe(path)
  })

  it('does not bother with a return_to for the landing path', () => {
    expect(loginUrlFor(DEFAULT_LANDING)).toBe('/login')
  })
})
