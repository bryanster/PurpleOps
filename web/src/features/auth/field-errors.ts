import { ApiError } from '@/api/errors'

/**
 * The message the server attached to one field, if it attached one.
 *
 * Field errors are how the password policy is reported (M1-002): there is one
 * definition of an acceptable password and it lives on the server, so the form
 * shows what came back rather than re-implementing the rules and disagreeing
 * with them.
 */
export function fieldErrorOf(error: unknown, field: string): string | undefined {
  return error instanceof ApiError ? error.fieldError(field) : undefined
}
