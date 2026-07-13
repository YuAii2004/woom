import { describe, expect, it, vi } from 'vitest'
import { createClientLogger } from './logger'

describe('客户端日志器', () => {
  it('只上报警告和错误，并过滤敏感上下文', () => {
    const sent: unknown[] = []
    const logger = createClientLogger({
      sessionId: 'session-id',
      send: event => {
        sent.push(event)
      },
      console: {
        debug: vi.fn(),
        info: vi.fn(),
        warn: vi.fn(),
        error: vi.fn(),
      },
    })

    logger.info('meeting_started')
    expect(sent).toHaveLength(0)

    logger.warn('permission_denied', {
      context: {
        permission_state: 'denied',
        token: 'secret-token',
      },
    })

    expect(sent).toHaveLength(1)
    expect(sent[0]).toMatchObject({
      level: 'warn',
      service: 'woom-web',
      event: 'permission_denied',
      session_id: 'session-id',
      context: { permission_state: 'denied' },
    })
    expect(JSON.stringify(sent[0])).not.toContain('secret-token')
  })

  it('为请求生成非空 request_id', () => {
    const logger = createClientLogger({ console: {} })
    expect(logger.requestId()).toEqual(expect.any(String))
    expect(logger.requestId()).not.toHaveLength(0)
  })
})
