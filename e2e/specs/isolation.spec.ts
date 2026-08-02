import fs from 'node:fs/promises'
import path from 'node:path'

import { reportedBuild } from '../harness/build-info'
import { externalBaseURL, runDir } from '../harness/paths'
import { managedPaths } from '../harness/server'
import { expect, test } from '../harness/test'

/**
 * The harness checking its own two structural claims, because a suite that only
 * *believes* it is isolated behaves exactly like one that is — right up until a
 * spec starts failing depending on which file ran before it.
 *
 * `version` is the seed step because it is the one command that changes
 * nothing: this file is checking the harness, not the CLI. It is a real seed
 * step regardless — it proves the hook runs, in order, before the server boots,
 * with the environment naming this spec file's database.
 */
test.use({ seed: [['version']] })

// The only claims this file makes are about a database the harness created. An
// external server has none, so there is nothing here to check — and inventing
// an assertion that passes anyway is how a suite stops meaning something.
test.skip(
  externalBaseURL() !== undefined,
  'BASE_URL points the suite at a server it does not own; there is no harness database to inspect',
)

test('this spec file got its own fresh database, seeded before the server booted', async ({
  server,
}) => {
  const paths = managedPaths(server)

  // Under this run's scratch directory — `<run>/<per-spec dir>/blacklight.duckdb`
  // — and therefore not the developer's ./blacklight.duckdb, and not shared with
  // any other spec file.
  expect(path.dirname(path.dirname(paths.dbPath))).toBe(runDir())

  const database = await fs.stat(paths.dbPath)
  expect(database.size).toBeGreaterThan(0)

  // The seed ran, it ran against this database, and its output was captured — a
  // seed step whose failure went unnoticed is how a suite ends up asserting
  // against an empty database and passing.
  const build = await reportedBuild()
  expect(paths.seedOutput).toHaveLength(1)
  expect(paths.seedOutput[0]).toContain(build.version)
})
