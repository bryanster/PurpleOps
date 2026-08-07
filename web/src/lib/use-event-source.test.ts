import { describe, expect, it } from 'vitest'

import { API_BASE_URL } from '@/api/client'

import { eventsUrl, isHubEnvelope } from './use-event-source'

describe('eventsUrl', () => {
  it('repeats the topics query parameter under the generated events path', () => {
    expect(eventsUrl(['content.jobs', 'content.jobs.abc'])).toBe(
      `${API_BASE_URL}/events?topics=content.jobs&topics=content.jobs.abc`,
    )
  })

  it('encodes odd topic characters', () => {
    expect(eventsUrl(['a b'])).toBe(`${API_BASE_URL}/events?topics=a+b`)
  })

  it('appends lastEventId query param when provided', () => {
    expect(eventsUrl(['engagement.abc'], 'cursor-123')).toBe(
      `${API_BASE_URL}/events?topics=engagement.abc&lastEventId=cursor-123`,
    )
  })
})

describe('isHubEnvelope', () => {
  it('returns true for a hub Event envelope', () => {
    expect(
      isHubEnvelope({
        id: 'abc-123',
        topic: 'engagement.xyz',
        type: 'execution.red_updated',
        at: '2026-01-01T00:00:00Z',
        data: { engagementId: 'xyz', verb: 'execution.red_updated' },
      }),
    ).toBe(true)
  })

  it('returns true even when data is null', () => {
    expect(
      isHubEnvelope({ id: 'abc', type: 'test.ping', data: null }),
    ).toBe(true)
  })

  it('returns false when id is missing', () => {
    expect(
      isHubEnvelope({ type: 'test.ping', data: {} }),
    ).toBe(false)
  })

  it('returns false when type is missing', () => {
    expect(
      isHubEnvelope({ id: 'abc', data: {} }),
    ).toBe(false)
  })

  it('returns false when data key is missing', () => {
    expect(
      isHubEnvelope({ id: 'abc', type: 'test.ping' }),
    ).toBe(false)
  })

  it('returns false for a non-object', () => {
    expect(isHubEnvelope(null)).toBe(false)
    expect(isHubEnvelope(undefined)).toBe(false)
    expect(isHubEnvelope('string')).toBe(false)
    expect(isHubEnvelope(42)).toBe(false)
  })

  it('returns false for a flat content job payload (no envelope)', () => {
    // Legacy / non-envelope payloads have flat keys like jobId, not id/type/data.
    expect(
      isHubEnvelope({ jobId: 'job-1', phase: 'import', status: 'running' }),
    ).toBe(false)
  })
})

