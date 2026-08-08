/**
 * Verb-family grouping for the activity rail filter chips (M4-008).
 *
 * Every engagement-scoped activity verb maps to one of six families.
 * Unknown verbs default to "Other" so new milestones don't silently
 * disappear from the rail.
 */

export type VerbFamily = 'Structure' | 'Execution' | 'Comments' | 'Findings' | 'Members' | 'Other'

export const VERB_FAMILIES: readonly VerbFamily[] = [
  'Structure',
  'Execution',
  'Comments',
  'Findings',
  'Members',
  'Other',
] as const

const FAMILY: Record<string, VerbFamily> = {
  // Engagement lifecycle
  'engagement.created': 'Structure',
  'engagement.updated': 'Structure',
  'engagement.status_changed': 'Structure',
  'engagement.deleted': 'Structure',

  // Workbook structure
  'scenario.created': 'Structure',
  'scenario.updated': 'Structure',
  'scenario.deleted': 'Structure',
  'scenario.reordered': 'Structure',
  'scenario.imported': 'Structure',
  'step.created': 'Structure',
  'step.updated': 'Structure',
  'step.deleted': 'Structure',
  'step.reordered': 'Structure',
  'step.revealed': 'Structure',

  // Executions
  'execution.red_updated': 'Execution',
  'execution.blue_updated': 'Execution',

  // Evidence
  'evidence.uploaded': 'Execution',
  'evidence.deleted': 'Execution',

  // Comments
  'comment.created': 'Comments',
  'comment.edited': 'Comments',

  // Findings
  'finding.created': 'Findings',
  'finding.updated': 'Findings',
  'finding.deleted': 'Findings',
  'finding.steps_changed': 'Findings',

  // Members
  'member.added': 'Members',
  'member.role_changed': 'Members',
  'member.removed': 'Members',
}

/**
 * Return the family for a verb, defaulting to `"Other"`.
 */
export function verbFamily(verb: string): VerbFamily {
  return FAMILY[verb] ?? 'Other'
}

/**
 * Human-readable label for a verb: drop the object prefix.
 * `step.created` → "Created", `execution.red_updated` → "Red updated".
 */
export function verbLabel(verb: string): string {
  const dot = verb.lastIndexOf('.')
  if (dot === -1) return verb
  const action = verb.slice(dot + 1)
  return action.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

/**
 * Human-readable label for a verb family filter chip.
 */
export function familyLabel(family: VerbFamily): string {
  return family === 'Other' ? 'Other' : family
}
