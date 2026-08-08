import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

import { DARK_CLASS, DARK_QUERY, THEME_STORAGE_KEY } from './theme'

/**
 * public/theme-bootstrap.js runs before any module and therefore cannot import
 * from this module — it repeats the storage key, the media query and the class
 * name as literals. This asserts the copies still agree, so renaming any of
 * them here fails the build instead of silently reintroducing the flash of the
 * wrong theme on first paint.
 */
// Resolved from the Vitest root — web/ — rather than from import.meta.url,
// which the jsdom environment reports as an http URL, not a file one.
const bootstrap = readFileSync(join(process.cwd(), 'public/theme-bootstrap.js'), 'utf8')

describe('theme bootstrap', () => {
  it('uses the same localStorage key as the application', () => {
    expect(bootstrap).toContain(`'${THEME_STORAGE_KEY}'`)
  })

  it('uses the same media query as the application', () => {
    expect(bootstrap).toContain(`'${DARK_QUERY}'`)
  })

  it('toggles the same class as the application', () => {
    expect(bootstrap).toContain(`classList.toggle('${DARK_CLASS}'`)
  })
})
