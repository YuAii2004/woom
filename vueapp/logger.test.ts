import { describe, expect, it, vi } from 'vitest'
import { createClientLogger } from './logger'

describe('Vue 客户端日志器', () => {
  it('只发送允许字段并过滤凭据', async () => {
    const sent: Record<string, unknown>[] = []
    const logger = createClientLogger({
      sessionId: 'session-test',
      send: event => {
        sent.push(event)
      },
      output: { error: vi.fn() }
    })

    logger.error('api_failed', {
      error: { message: '请求失败', token: 'secret' },
      context: { http_status: 500, token: 'secret' }
    })
    await Promise.resolve()

    expect(sent).toHaveLength(1)
    expect(sent[0].session_id).toBe('session-test')
    expect(sent[0].error).toEqual({ message: '请求失败' })
    expect(sent[0].context).toEqual({ http_status: 500 })
  })
})
