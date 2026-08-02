import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

import { blacklightBinary } from './paths'

const run = promisify(execFile)

/** What `blacklight --version` prints: `internal/version`'s `Info.String()`. */
const versionLine = /^(?<version>\S+) \(commit (?<commit>\S+), built (?<buildDate>\S+)\)$/

export interface BuildInfo {
  version: string
  commit: string
  buildDate: string
}

let cached: Promise<BuildInfo> | undefined

/**
 * The build identity the binary reports on the command line.
 *
 * The smoke spec compares this with what the version screen displays, which
 * only agrees if the whole chain is intact: ldflags reached the binary, the
 * binary embedded this build of the SPA, the SPA called the API, and the
 * generated client parsed the answer. Asking the API instead would compare the
 * server with itself and prove none of it.
 */
export function reportedBuild(): Promise<BuildInfo> {
  cached ??= read()
  return cached
}

async function read(): Promise<BuildInfo> {
  const { stdout } = await run(blacklightBinary, ['--version'])
  const line = stdout.trim()
  const fields = versionLine.exec(line)?.groups
  if (
    fields?.version === undefined ||
    fields.commit === undefined ||
    fields.buildDate === undefined
  ) {
    throw new Error(
      `could not read a build identity from \`${blacklightBinary} --version\`.\n` +
        `  printed: ${JSON.stringify(line)}\n` +
        `  wanted:  <version> (commit <sha>, built <timestamp>)\n` +
        `If internal/version.Info.String() changed, this pattern has to change with it.`,
    )
  }
  return { version: fields.version, commit: fields.commit, buildDate: fields.buildDate }
}
