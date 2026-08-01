import { setupServer } from 'msw/node'

import { handlers } from './handlers'

/**
 * The interceptor every test runs against, started in `src/test/setup.ts`.
 *
 * One instance for the whole suite: `server.use(...)` adds a handler for one
 * test and `resetHandlers()` after each test puts the defaults back, so a test
 * that overrides `/version` cannot change what the next one sees.
 */
export const server = setupServer(...handlers)
