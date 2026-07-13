import { clientLogger } from './logger'

interface Room {
  roomId: string,
  locked: boolean,
  owner: string,
  presenter?: string,
  streamId?: string,
  streams?: Record<string, Stream>,
}

/**
 * @see https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/connectionState#value
 */
enum StreamState {
  New = 'new',
  Signaled = 'signaled',
  Connecting = 'connecting',
  Connected = 'connected',
  Disconnected = 'disconnected',
  Failed = 'failed',
  Closed = 'closed',
}

interface Stream {
  name: string,
  state: StreamState
  audio: boolean,
  video: boolean,
  screen: boolean,
}

interface User {
  streamId: string,
  token: string,
}

let token = ''
let roomId = ''
const logger = clientLogger

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId: string

  constructor(status: number, code: string, message: string, requestId: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.requestId = requestId
  }
}

interface RequestFields {
  operation: string
  roomId?: string
  streamId?: string
}

async function request(url: string, init: RequestInit, fields: RequestFields): Promise<Response> {
  const requestId = logger.requestId()
  const headers = new Headers(init.headers)
  headers.set('X-Request-ID', requestId)

  try {
    const response = await fetch(url, { ...init, headers })
    if (response.ok) return response

    const body = await response.json().catch(() => undefined) as { error?: { code?: string, message?: string } } | undefined
    const code = body?.error?.code ?? 'request_failed'
    const message = body?.error?.message ?? '请求失败'
    const error = new ApiError(response.status, code, message, requestId)
    logger.error('api_request_failed', {
      component: 'api',
      ...fields,
      requestId,
      error: { code, message, status: response.status },
      context: { http_status: response.status },
    })
    throw error
  } catch (error) {
    if (error instanceof ApiError) throw error
    logger.error('api_request_exception', {
      component: 'api',
      ...fields,
      requestId,
      error: { kind: 'network_error', message: error instanceof Error ? error.message : String(error) },
    })
    throw error
  }
}

async function requestJSON<T>(url: string, init: RequestInit, fields: RequestFields): Promise<T> {
  const response = await request(url, init, fields)
  return response.json() as Promise<T>
}

function setApiToken(str: string) {
  token = str
}

function setRoomId(str: string) {
  roomId = str
}

async function newUser(): Promise<User> {
  return requestJSON<User>('/user/', {
    method: 'POST',
  }, { operation: 'create_user' })
}

async function newRoom(): Promise<Room> {
  return requestJSON<Room>('/room/', {
    headers: {
      'Authorization': `Bearer ${token}`,
    },
    method: 'POST',
  }, { operation: 'create_room' })
}

async function getRoom(roomId: string): Promise<Room> {
  return requestJSON<Room>(`/room/${roomId}`, {
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  }, { operation: 'get_room', roomId })
}

async function newStream(roomId: string): Promise<Room> {
  return requestJSON<Room>(`/room/${roomId}/stream`, {
    headers: {
      'Authorization': `Bearer ${token}`,
    },
    method: 'POST',
  }, { operation: 'create_stream', roomId })
}

async function setStream(streamId: string, data: Stream): Promise<Room> {
  return requestJSON<Room>(`/room/${roomId}/stream/${streamId}`, {
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    method: 'PATCH',
    body: JSON.stringify(data),
  }, { operation: 'update_stream', roomId, streamId })
}

async function delStream(roomId: string, streamId: string): Promise<Response> {
  return request(`/room/${roomId}/stream/${streamId}`, {
    headers: {
      'Authorization': `Bearer ${token}`,
    },
    method: 'DELETE',
    keepalive: true,
  }, { operation: 'delete_stream', roomId, streamId })
}

export {
  setRoomId,
  setApiToken,
  newUser,

  newRoom,
  getRoom,
  newStream,
  setStream,
  delStream,

  StreamState,

  token,
}

export type {
  Room,
  Stream,
  User,
}
