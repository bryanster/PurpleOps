import { createHmac } from 'node:crypto'

/**
 * A TOTP code from a base32 secret (RFC 6238), for the specs that have to get
 * *through* an enrolment rather than only up to it.
 *
 * It is implemented here rather than pulled in as a dependency for the reason
 * the harness README gives about dependencies in general: this is thirty lines
 * of arithmetic that the server already implements independently
 * (`internal/authn/totp`), and two implementations agreeing is exactly what a
 * test of an authenticator flow should be checking. A shared library would make
 * both sides right by construction and prove nothing.
 */

/** The step size the server uses. Matches `totp.Period`. */
const PERIOD_SECONDS = 30

/** How many digits a code has. Matches the spec's `TOTPCodeRequest` pattern. */
const DIGITS = 6

const BASE32_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'

/**
 * The code an authenticator app would be showing for this secret, now.
 *
 * `at` exists so a test can ask for a neighbouring step; the default is the
 * clock, which is what makes this usable in a spec that has just been handed a
 * fresh secret.
 */
export function totpCode(secret: string, at: Date = new Date()): string {
  const counter = Math.floor(at.getTime() / 1000 / PERIOD_SECONDS)

  // The counter as eight big-endian bytes. `writeBigUInt64BE` rather than two
  // 32-bit halves, so the arithmetic is right past 2038 as well as today.
  const message = Buffer.alloc(8)
  message.writeBigUInt64BE(BigInt(counter))

  const digest = createHmac('sha1', decodeBase32(secret)).update(message).digest()

  // Dynamic truncation, RFC 4226 §5.3: the low nibble of the last byte picks
  // where to read the four-byte value from, and the top bit is masked off so
  // the result is positive on every implementation's idea of an integer.
  //
  // `readUInt32BE` rather than four indexed reads: it does the same arithmetic,
  // and it does not need four assertions that an index into a twenty-byte
  // digest is present.
  const offset = digest.readUInt8(digest.length - 1) & 0x0f
  const binary = digest.readUInt32BE(offset) & 0x7fffffff

  return String(binary % 10 ** DIGITS).padStart(DIGITS, '0')
}

/**
 * Decode a base32 secret. Padding and lower case are accepted because
 * authenticator apps and humans produce both; anything outside the alphabet is
 * an error rather than a silently different secret, which would show up as an
 * inexplicably wrong code.
 */
function decodeBase32(secret: string): Buffer {
  const cleaned = secret.replace(/=+$/, '').toUpperCase().replace(/\s+/g, '')

  let bits = 0
  let value = 0
  const bytes: number[] = []

  for (const character of cleaned) {
    const index = BASE32_ALPHABET.indexOf(character)
    if (index === -1) {
      throw new Error(`totp: ${JSON.stringify(character)} is not a base32 character`)
    }
    value = (value << 5) | index
    bits += 5
    if (bits >= 8) {
      bits -= 8
      bytes.push((value >>> bits) & 0xff)
    }
  }

  return Buffer.from(bytes)
}
