<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

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
let refreshTimer: number | undefined

const remoteStreams = computed(() => Object.entries(room.value?.streams || {}).filter(([id]) => id !== streamId.value))

async function ensureUser() {
  if (token.value && streamId.value) return
  const response = await fetch('/user/', { method: 'POST' })
  if (!response.ok) throw new Error('无法创建用户身份')
  const user = await response.json() as User
  token.value = user.token
  streamId.value = user.streamId
  localStorage.setItem('woom-token', user.token)
  localStorage.setItem('woom-stream', user.streamId)
}

async function api<T>(url: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers)
  headers.set('Authorization', `Bearer ${token.value}`)
  if (init.body) headers.set('Content-Type', 'application/json')
  const response = await fetch(url, { ...init, headers })
  if (!response.ok) throw new Error('会议服务请求失败')
  return response.status === 204 ? undefined as T : await response.json() as T
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
  mediaStream.value = await navigator.mediaDevices.getUserMedia({ audio: true, video: true })
}

function setVideoStream(element: unknown) {
  if (element instanceof HTMLVideoElement && mediaStream.value) element.srcObject = mediaStream.value
}

async function joinRoom() {
  await run(async () => {
    await prepareMedia()
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
  })
}

async function refreshRoom() {
  if (!meetingId.value || !token.value) return
  room.value = await api<Room>(`/room/${meetingId.value}`)
}

async function leaveMeeting() {
  await run(async () => {
    if (meetingId.value && streamId.value) {
      await api(`/room/${meetingId.value}/stream/${streamId.value}`, { method: 'DELETE' })
    }
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
        <div v-if="screen === 'meeting'" class="badge badge-outline">{{ meetingId }}</div>
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
              <div class="flex aspect-video items-center justify-center rounded-box bg-neutral text-neutral-content">等待媒体</div>
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
