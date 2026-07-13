<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { WHIPClient } from 'whip-whep/whip'
import { WHEPClient } from 'whip-whep/whep'
import { clientLogger } from './logger'

type Screen = 'home' | 'prepare' | 'meeting'
type StreamState = 'new' | 'signaled' | 'connecting' | 'connected' | 'disconnected' | 'failed' | 'closed'

interface User {
  streamId: string
  token: string
}

interface Stream {
  name: string
  state: StreamState
  audio: boolean
  video: boolean
  screen: boolean
}

interface Room {
  roomId: string
  owner: string
  locked: boolean
  streams?: Record<string, Stream>
}

const screen = ref<Screen>('home')
const meetingId = ref('')
const meetingInput = ref('')
const displayName = ref('')
const token = ref(localStorage.getItem('woom-token') || '')
const streamId = ref(localStorage.getItem('woom-stream') || '')
const mediaStream = ref<MediaStream | null>(null)
const room = ref<Room | null>(null)
const loading = ref(false)
const errorMessage = ref('')
const remoteMediaVersion = ref(0)
let publisher: { client: WHIPClient, pc: RTCPeerConnection } | undefined
const remoteMedia = new Map<string, { client: WHEPClient, pc: RTCPeerConnection, stream: MediaStream }>()
let refreshTimer: number | undefined

const remoteStreams = computed(() => {
  remoteMediaVersion.value
  return Object.entries(room.value?.streams || {}).filter(([id]) => id !== streamId.value)
})

async function ensureUser() {
  if (token.value && streamId.value) return
  const requestId = clientLogger.requestId()
  clientLogger.info('user_create_start', { component: 'api', operation: 'create_user', requestId })
  const response = await fetch('/user/', { method: 'POST', headers: { 'X-Request-ID': requestId } })
  if (!response.ok) {
    clientLogger.error('api_request_failed', { component: 'api', operation: 'create_user', requestId, error: { status: response.status, code: 'user_create_failed', message: '无法创建用户身份' } })
    throw new Error('无法创建用户身份')
  }
  const user = await response.json() as User
  token.value = user.token
  streamId.value = user.streamId
  localStorage.setItem('woom-token', user.token)
  localStorage.setItem('woom-stream', user.streamId)
  clientLogger.info('user_create_complete', { component: 'api', operation: 'create_user', requestId, streamId: user.streamId })
}

async function api<T>(url: string, init: RequestInit = {}) {
  const requestId = clientLogger.requestId()
  let loggedFailure = false
  const headers = new Headers(init.headers)
  headers.set('Authorization', `Bearer ${token.value}`)
  headers.set('X-Request-ID', requestId)
  if (init.body) headers.set('Content-Type', 'application/json')
  try {
    const response = await fetch(url, { ...init, headers })
    if (!response.ok) {
      const body = await response.json().catch(() => undefined) as { error?: { code?: string, message?: string } } | undefined
      const message = body?.error?.message || '会议服务请求失败'
      clientLogger.error('api_request_failed', {
        component: 'api',
        operation: init.method || 'GET',
        requestId,
        roomId: meetingId.value,
        streamId: streamId.value,
        error: { status: response.status, code: body?.error?.code || 'request_failed', message },
        context: { http_status: response.status }
      })
      loggedFailure = true
      throw new Error(message)
    }
    return response.status === 204 ? undefined as T : await response.json() as T
  } catch (error) {
    if (!loggedFailure) {
      clientLogger.error('api_request_exception', { component: 'api', operation: init.method || 'GET', requestId, roomId: meetingId.value, streamId: streamId.value, error: { kind: 'network_error', message: error instanceof Error ? error.message : '网络请求失败' } })
    }
    throw error
  }
}

async function enterPrepare(id: string) {
  meetingId.value = id
  meetingInput.value = id
  localStorage.setItem('woom-meeting', id)
  screen.value = 'prepare'
}

async function createMeeting() {
  await run(async () => {
    await ensureUser()
    const created = await api<Room>('/room/', { method: 'POST' })
    await enterPrepare(created.roomId)
  })
}

async function joinMeeting() {
  if (!meetingInput.value.trim()) return
  await run(async () => {
    await ensureUser()
    await enterPrepare(meetingInput.value.trim())
  })
}

async function prepareMedia() {
  if (mediaStream.value) return
  clientLogger.info('media_prepare_start', { component: 'media', operation: 'prepare', roomId: meetingId.value, streamId: streamId.value })
  try {
    mediaStream.value = await withTimeout(navigator.mediaDevices.getUserMedia({ audio: true, video: true }), 5_000)
    clientLogger.info('media_prepare_complete', { component: 'media', operation: 'prepare', roomId: meetingId.value, streamId: streamId.value })
  } catch {
    mediaStream.value = new MediaStream()
    clientLogger.warn('media_prepare_failed', { component: 'media', operation: 'prepare', roomId: meetingId.value, streamId: streamId.value, error: { kind: 'permission_or_timeout', message: '无法获取摄像头或麦克风' } })
  }
}

function setVideoStream(element: unknown) {
  if (element instanceof HTMLVideoElement && mediaStream.value) element.srcObject = mediaStream.value
}

function setRemoteVideoStream(element: unknown, id: string) {
  if (element instanceof HTMLVideoElement) element.srcObject = remoteMedia.get(id)?.stream || null
}

async function withTimeout<T>(promise: Promise<T>, milliseconds: number) {
  let timer: number | undefined
  const timeout = new Promise<never>((_, reject) => {
    timer = window.setTimeout(() => reject(new Error('媒体连接超时')), milliseconds)
  })
  try {
    return await Promise.race([promise, timeout])
  } finally {
    if (timer) window.clearTimeout(timer)
  }
}

async function publishMedia() {
  if (!mediaStream.value || publisher) return
  const pc = new RTCPeerConnection()
  mediaStream.value.getTracks().forEach(track => pc.addTrack(track, mediaStream.value as MediaStream))
  const client = new WHIPClient()
  try {
    clientLogger.info('whip_start', { component: 'media', operation: 'publish', roomId: meetingId.value, streamId: streamId.value })
    await withTimeout(client.publish(pc, `${location.origin}/whip/${streamId.value}`, token.value), 10_000)
    publisher = { client, pc }
    clientLogger.info('whip_connected', { component: 'media', operation: 'publish', roomId: meetingId.value, streamId: streamId.value })
  } catch (error) {
    pc.close()
    clientLogger.error('whip_failed', { component: 'media', operation: 'publish', roomId: meetingId.value, streamId: streamId.value, error: { kind: 'publish_failed', message: error instanceof Error ? error.message : '媒体发布失败' } })
    throw error
  }
}

async function subscribeMedia(id: string) {
  if (remoteMedia.has(id)) return
  const pc = new RTCPeerConnection()
  pc.addTransceiver('video', { direction: 'recvonly' })
  pc.addTransceiver('audio', { direction: 'recvonly' })
  const stream = new MediaStream()
  pc.ontrack = event => {
    stream.addTrack(event.track)
    remoteMediaVersion.value += 1
  }
  const client = new WHEPClient()
  try {
    clientLogger.info('whep_start', { component: 'media', operation: 'view', roomId: meetingId.value, streamId: id })
    await withTimeout(client.view(pc, `${location.origin}/whep/${id}`), 10_000)
    remoteMedia.set(id, { client, pc, stream })
    remoteMediaVersion.value += 1
    clientLogger.info('whep_connected', { component: 'media', operation: 'view', roomId: meetingId.value, streamId: id })
  } catch (error) {
    pc.close()
    clientLogger.error('whep_failed', { component: 'media', operation: 'view', roomId: meetingId.value, streamId: id, error: { kind: 'view_failed', message: error instanceof Error ? error.message : '媒体播放失败' } })
  }
}

function closeMedia() {
  publisher?.client.stop().catch(() => undefined)
  publisher?.pc.close()
  publisher = undefined
  for (const media of remoteMedia.values()) {
    media.client.stop().catch(() => undefined)
    media.pc.close()
  }
  remoteMedia.clear()
  remoteMediaVersion.value += 1
}

async function joinRoom() {
  await run(async () => {
    clientLogger.info('meeting_join_start', { component: 'meeting', operation: 'join', roomId: meetingId.value, streamId: streamId.value })
    await prepareMedia()
    void publishMedia().catch(() => undefined)
    await api<Room>(`/room/${meetingId.value}/stream/${streamId.value}`, {
      method: 'PATCH',
      body: JSON.stringify({
        name: displayName.value || streamId.value,
        state: 'connected',
        audio: mediaStream.value?.getAudioTracks().some(track => track.enabled) || false,
        video: mediaStream.value?.getVideoTracks().some(track => track.enabled) || false,
        screen: false
      })
    })
    screen.value = 'meeting'
    await refreshRoom()
    clientLogger.info('meeting_joined', { component: 'meeting', operation: 'join', roomId: meetingId.value, streamId: streamId.value })
  })
}

async function refreshRoom() {
  if (!meetingId.value || !token.value) return
  room.value = await api<Room>(`/room/${meetingId.value}`)
  const ids = new Set(remoteStreams.value.map(([id]) => id))
  for (const id of ids) void subscribeMedia(id)
  for (const id of remoteMedia.keys()) {
    if (!ids.has(id)) remoteMedia.delete(id)
  }
}

async function leaveMeeting() {
  await run(async () => {
    clientLogger.info('meeting_leave', { component: 'meeting', operation: 'leave', roomId: meetingId.value, streamId: streamId.value })
    if (meetingId.value && streamId.value) {
      await api(`/room/${meetingId.value}/stream/${streamId.value}`, { method: 'DELETE' })
    }
    closeMedia()
    mediaStream.value?.getTracks().forEach(track => track.stop())
    mediaStream.value = null
    screen.value = 'home'
    room.value = null
  })
}

async function run(action: () => Promise<void>) {
  loading.value = true
  errorMessage.value = ''
  try {
    await action()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '操作失败'
    clientLogger.error('ui_action_failed', { component: 'meeting', operation: screen.value, roomId: meetingId.value, streamId: streamId.value, error: { kind: 'ui_action_failed', message: error instanceof Error ? error.message : '操作失败' } })
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  const pathMeetingId = location.pathname.slice(1)
  if (pathMeetingId) meetingInput.value = pathMeetingId
  if (localStorage.getItem('woom-meeting')) meetingInput.value = localStorage.getItem('woom-meeting') || ''
  refreshTimer = window.setInterval(() => {
    if (screen.value === 'meeting') refreshRoom().catch(() => undefined)
  }, 3000)
})

onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
  mediaStream.value?.getTracks().forEach(track => track.stop())
})
</script>

<template>
  <main class="min-h-screen bg-base-200 px-4 py-8 text-base-content">
    <div class="mx-auto flex min-h-[calc(100vh-4rem)] max-w-5xl flex-col justify-center">
      <header class="mb-8 flex items-center justify-between">
        <div>
          <p class="text-sm uppercase tracking-[0.2em] text-primary">WOOM</p>
          <h1 class="text-3xl font-bold">轻量会议</h1>
        </div>
        <div v-if="screen === 'meeting'" id="meeting-id" class="badge badge-outline">{{ meetingId }}</div>
      </header>

      <div v-if="errorMessage" role="alert" class="alert alert-error mb-4">
        <span>{{ errorMessage }}</span>
      </div>

      <section v-if="screen === 'home'" class="card border border-base-300 bg-base-100 shadow-xl">
        <div class="card-body gap-6">
          <div>
            <h2 class="card-title">开始一场会议</h2>
            <p class="text-sm text-base-content/70">创建新的会议，或输入会议号加入。</p>
          </div>
          <button class="btn btn-primary w-full sm:w-fit" :disabled="loading" @click="createMeeting">新建会议</button>
          <div class="divider">或</div>
          <div class="join w-full">
            <input v-model="meetingInput" class="input input-bordered join-item w-full" placeholder="输入会议号" maxlength="11" />
            <button class="btn btn-secondary join-item" :disabled="loading || !meetingInput" @click="joinMeeting">加入</button>
          </div>
        </div>
      </section>

      <section v-else-if="screen === 'prepare'" class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div class="card border border-base-300 bg-base-100 shadow-xl">
          <div class="card-body">
            <h2 class="card-title">准备加入</h2>
            <video v-if="mediaStream" :ref="setVideoStream" autoplay muted playsinline class="w-full rounded-box bg-black" />
            <div v-else class="flex aspect-video items-center justify-center rounded-box bg-neutral text-neutral-content">点击加入后开启摄像头</div>
            <label class="form-control mt-4">
              <div class="label"><span class="label-text">显示名称</span></div>
              <input v-model="displayName" class="input input-bordered" placeholder="输入名称" />
            </label>
            <div class="card-actions mt-4 justify-end">
              <button class="btn btn-primary" :disabled="loading" @click="joinRoom">{{ loading ? '连接中' : '加入会议' }}</button>
            </div>
          </div>
        </div>
      </section>

      <section v-else class="space-y-6">
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <article class="card border border-primary/40 bg-base-100 shadow-lg">
            <div class="card-body p-4">
              <video v-if="mediaStream" :ref="setVideoStream" autoplay muted playsinline class="w-full rounded-box bg-black" />
              <div class="mt-2 flex items-center justify-between"><span>{{ displayName || streamId }}</span><span class="badge badge-primary">我</span></div>
            </div>
          </article>
          <article v-for="[id, participant] in remoteStreams" :key="id" class="card border border-base-300 bg-base-100 shadow-lg">
            <div class="card-body p-4">
              <video v-if="remoteMedia.get(id)" :ref="element => setRemoteVideoStream(element, id)" autoplay playsinline class="w-full rounded-box bg-black" />
              <div v-else class="flex aspect-video items-center justify-center rounded-box bg-neutral text-neutral-content">等待媒体</div>
              <div class="mt-2 flex items-center justify-between"><span>{{ participant.name || id }}</span><span class="badge badge-ghost">{{ participant.state }}</span></div>
            </div>
          </article>
        </div>
        <div class="flex justify-end">
          <button class="btn btn-error" :disabled="loading" @click="leaveMeeting">离开会议</button>
        </div>
      </section>
    </div>
  </main>
</template>
