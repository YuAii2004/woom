import { describe, expect, it } from 'vitest'
import { analyzeLogs, renderMarkdown } from './diagnose.mjs'

describe('诊断报告分析器', () => {
  it('识别依赖连接拒绝并给出可执行建议', () => {
    const report = analyzeLogs([
      {
        source: 'playwright.log',
        content: '[WebServer] Redis connection failed: connection refused (os error 10061)'
      }
    ], {
      platform: 'windows-latest',
      browser: 'chrome'
    })

    expect(report.findings).toEqual([
      expect.objectContaining({
        code: 'dependency_connection_refused',
        severity: 'error',
        retryable: true
      })
    ])
    expect(report.findings[0].suggestions).toContain('检查 Redis 是否在目标端口持续运行')
  })

  it('提取结构化事件并在 Markdown 中保留关联信息', () => {
    const report = analyzeLogs([
      {
        source: 'client-events.log',
        content: '{"level":"error","event":"meeting_join_failed","request_id":"req-123","error":{"code":"dependencies_unavailable","message":"Redis unavailable"}}'
      }
    ], {
      platform: 'macos-latest',
      browser: 'firefox'
    })

    expect(report.events).toEqual([
      expect.objectContaining({
        event: 'meeting_join_failed',
        request_id: 'req-123'
      })
    ])

    const markdown = renderMarkdown(report)
    expect(markdown).toContain('meeting_join_failed')
    expect(markdown).toContain('req-123')
    expect(markdown).toContain('macos-latest / firefox')
  })

  it('在报告中移除敏感字段和值', () => {
    const report = analyzeLogs([
      {
        source: 'client-events.log',
        content: '{"level":"error","event":"login_failed","password":"secret-value","authorization":"Bearer sensitive-token","message":"Authorization Bearer another-token"}'
      }
    ])

    const serialized = JSON.stringify(report)
    expect(serialized).not.toContain('secret-value')
    expect(serialized).not.toContain('sensitive-token')
    expect(serialized).toContain('[REDACTED]')
  })
})
