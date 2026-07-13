export type ClientLogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface ClientLogEvent {
  timestamp: string
  level: ClientLogLevel
  service: 'woom-web'
  event: string
  component?: string
  operation?: string
  request_id?: string
  session_id: string
  room_id?: string
  stream_id?: string
  browser?: string
  os?: string
  duration_ms?: number
  error?: Record<string, string | number | boolean>
  context?: Record<string, string | number | boolean>
}

interface LogFields {
  component?: string
  operation?: string
  requestId?: string
  roomId?: string
  streamId?: string
  browser?: string
  os?: string
  durationMs?: number
  error?: Record<string, unknown>
  context?: Record<string, unknown>
}

interface ClientLoggerOptions {
  sessionId?: string
  send?: (event: ClientLogEvent) => void | Promise<void>
  console?: Partial<Pick<Console, 'debug' | 'info' | 'warn' | 'error'>>
}

const safeContextKeys = new Set([
  'browser',
  'connection_state',
  'device_kind',
  'feature',
  'http_status',
  'os',
  'permission_state',
  'retry_count',
])

const safeErrorKeys = new Set(['code', 'kind', 'message', 'name', 'status'])

function createId(prefix: string): string {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID()
  }
  return `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export function createSessionId(): string {
  return createId('session')
}

export function createRequestId(): string {
  return createId('request')
}

function safeFields(
  fields: Record<string, unknown> | undefined,
  allowedKeys: Set<string>,
): Record<string, string | number | boolean> | undefined {
  if (!fields) return undefined
  const result: Record<string, string | number | boolean> = {}
  for (const [key, value] of Object.entries(fields)) {
    if (!allowedKeys.has(key)) continue
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
      result[key] = value
    }
  }
  return Object.keys(result).length > 0 ? result : undefined
}

async function sendToServer(event: ClientLogEvent): Promise<void> {
  if (typeof fetch !== 'function') return
  await fetch('/client-events', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(event),
    keepalive: true,
  }).catch(() => undefined)
}

export function createClientLogger(options: ClientLoggerOptions = {}) {
  const sessionId = options.sessionId ?? createSessionId()
  const output = options.console ?? globalThis.console
  const send = options.send ?? sendToServer

  const write = (level: ClientLogLevel, event: string, fields: LogFields = {}): ClientLogEvent => {
    const payload: ClientLogEvent = {
      timestamp: new Date().toISOString(),
      level,
      service: 'woom-web',
      event,
      session_id: sessionId,
      ...(fields.component ? { component: fields.component } : {}),
      ...(fields.operation ? { operation: fields.operation } : {}),
      ...(fields.requestId ? { request_id: fields.requestId } : {}),
      ...(fields.roomId ? { room_id: fields.roomId } : {}),
      ...(fields.streamId ? { stream_id: fields.streamId } : {}),
      ...(fields.browser ? { browser: fields.browser } : {}),
      ...(fields.os ? { os: fields.os } : {}),
      ...(fields.durationMs === undefined ? {} : { duration_ms: fields.durationMs }),
      ...(safeFields(fields.error, safeErrorKeys) ? { error: safeFields(fields.error, safeErrorKeys) } : {}),
      ...(safeFields(fields.context, safeContextKeys) ? { context: safeFields(fields.context, safeContextKeys) } : {}),
    }

    output[level]?.(payload)
    if (level === 'warn' || level === 'error') {
      void Promise.resolve(send(payload)).catch(() => undefined)
    }
    return payload
  }

  return {
    sessionId,
    requestId: createRequestId,
    debug: (event: string, fields?: LogFields) => write('debug', event, fields),
    info: (event: string, fields?: LogFields) => write('info', event, fields),
    warn: (event: string, fields?: LogFields) => write('warn', event, fields),
    error: (event: string, fields?: LogFields) => write('error', event, fields),
  }
}

export const clientLogger = createClientLogger()
