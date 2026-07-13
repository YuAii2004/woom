export type ClientLogLevel = 'debug' | 'info' | 'warn' | 'error'

interface LogFields {
  component?: string
  operation?: string
  requestId?: string
  roomId?: string
  streamId?: string
  error?: Record<string, unknown>
  context?: Record<string, unknown>
}

const safeContextKeys = new Set(['browser', 'connection_state', 'device_kind', 'feature', 'http_status', 'os', 'permission_state', 'retry_count'])
const safeErrorKeys = new Set(['code', 'kind', 'message', 'name', 'status'])

function createId(prefix: string) {
  return globalThis.crypto?.randomUUID?.() || `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function safeFields(fields: Record<string, unknown> | undefined, allowed: Set<string>) {
  if (!fields) return undefined
  const result: Record<string, string | number | boolean> = {}
  for (const [key, value] of Object.entries(fields)) {
    if (allowed.has(key) && (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean')) result[key] = value
  }
  return Object.keys(result).length ? result : undefined
}

async function send(payload: Record<string, unknown>) {
  await fetch('/client-events', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
    keepalive: true
  }).catch(() => undefined)
}

const sessionId = createId('session')

interface LoggerOptions {
  sessionId?: string
  send?: (payload: Record<string, unknown>) => void | Promise<void>
  output?: Partial<Pick<Console, 'debug' | 'info' | 'warn' | 'error'>>
}

export function createClientLogger(options: LoggerOptions = {}) {
  const loggerSessionId = options.sessionId || sessionId
  const sendEvent = options.send || send
  const output = options.output || globalThis.console

  function write(level: ClientLogLevel, event: string, fields: LogFields = {}) {
    const payload = {
      timestamp: new Date().toISOString(),
      level,
      service: 'woom-web',
      event,
      session_id: loggerSessionId,
      ...(fields.component ? { component: fields.component } : {}),
      ...(fields.operation ? { operation: fields.operation } : {}),
      ...(fields.requestId ? { request_id: fields.requestId } : {}),
      ...(fields.roomId ? { room_id: fields.roomId } : {}),
      ...(fields.streamId ? { stream_id: fields.streamId } : {}),
      ...(safeFields(fields.error, safeErrorKeys) ? { error: safeFields(fields.error, safeErrorKeys) } : {}),
      ...(safeFields(fields.context, safeContextKeys) ? { context: safeFields(fields.context, safeContextKeys) } : {})
    }
    output?.[level]?.(payload)
    if (level === 'warn' || level === 'error') void sendEvent(payload)
    return payload
  }

  return {
    sessionId: loggerSessionId,
    requestId: () => createId('request'),
    debug: (event: string, fields?: LogFields) => write('debug', event, fields),
    info: (event: string, fields?: LogFields) => write('info', event, fields),
    warn: (event: string, fields?: LogFields) => write('warn', event, fields),
    error: (event: string, fields?: LogFields) => write('error', event, fields)
  }
}

export const clientLogger = createClientLogger()
