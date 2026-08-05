import { describe, expect, it } from 'vitest'

import { API_BASE_URL } from '@/api/client'

import { eventsUrl } from './use-event-source'

describe('eventsUrl', () => {
  it('repeats the topics query parameter under the generated events path', () => {
    expect(eventsUrl(['content.jobs', 'content.jobs.abc'])).toBe(
      `${API_BASE_URL}/events?topics=content.jobs&topics=content.jobs.abc`,
    )
  })

  it('encodes odd topic characters', () => {
    expect(eventsUrl(['a b'])).toBe(`${API_BASE_URL}/events?topics=a+b`)
  })
})
