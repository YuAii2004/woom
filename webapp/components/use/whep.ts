import { useSyncExternalStore } from 'react'
import { event, Context, Data } from './whxp'
import { StreamState, Stream } from '../../lib/api'
import { WHEPClient } from 'whip-whep/whep'
import { clientLogger } from '../../lib/logger'

interface WHIPData extends Data {
  connStatus: string
  setRemoteStatus: (userStatus: Stream) => void,
}

class WHEPContext extends Context {
  client: WHEPClient = new WHEPClient()
  cache: WHIPData
  remoteStatus: Stream
  connStatus: string = 'new'
  constructor(id: string) {
    super(id)
    this.cache = this.clone()
    this.remoteStatus = Object.assign({}, this.userStatus)
  }

  private newPeerConnection() {
    const { pc, setStream } = this
    pc.addTransceiver('video', { 'direction': 'recvonly' })
    pc.addTransceiver('audio', { 'direction': 'recvonly' })
    pc.ontrack = ev => setStream(new MediaStream([...this.stream.getTracks(), ev.track]))
  }

  setStream = (stream: MediaStream) => {
    this.stream = stream
    this.sync()
  }

  setRemoteStatus = (userStatus: Stream) => this.remoteStatus = userStatus

  clone(): WHIPData {
    return {
      id: this.id,
      stream: this.stream,
      connStatus: this.connStatus,
      userStatus: this.userStatus,
      stop: () => this.stop(),
      start: () => this.start(),
      restart: () => this.restart(),
      setRemoteStatus: (userStatus: Stream) => this.setRemoteStatus(userStatus),
    }
  }

  export = () => this.cache

  sync() {
    this.cache = this.clone()
    this.dispatchEvent(event)
  }

  onconnectionstatechange = () => {
    this.userStatus.state = this.pc.connectionState as StreamState
    this.connStatus = this.pc.connectionState
    this.sync()
  }

  async start() {
    const { id, pc, client, userStatus } = this
    pc.addEventListener('connectionstatechange', this.onconnectionstatechange)
    userStatus.state = StreamState.Signaled
    this.sync()
    this.newPeerConnection()

    try {
      const url = location.origin + `/whep/${id}`
      clientLogger.info('whep_start', { component: 'media', operation: 'view', streamId: id })
      await client.view(pc, url)
      clientLogger.info('whep_connected', { component: 'media', operation: 'view', streamId: id })
    } catch (e) {
      clientLogger.error('whep_failed', {
        component: 'media',
        operation: 'view',
        streamId: id,
        error: { kind: 'view_failed', message: e instanceof Error ? e.message : String(e) },
      })
      userStatus.state = StreamState.Failed
      this.sync()
    }

    if (!this.timer) this.timer = setInterval(() => this.run(), 5000)
  }

  async stop() {
    if (this.timer) {
      clearInterval(this.timer)
      this.timer = undefined
    }
    try {
      await this.client.stop()
      this.pc.removeEventListener('connectionstatechange', this.onconnectionstatechange)
      clientLogger.info('whep_stop', { component: 'media', operation: 'view', streamId: this.id })
    } catch (e) {
      clientLogger.error('whep_stop_failed', {
        component: 'media',
        operation: 'view',
        streamId: this.id,
        error: { kind: 'stop_failed', message: e instanceof Error ? e.message : String(e) },
      })
    }
  }

  async restart() {
    clientLogger.info('whep_restart', { component: 'media', operation: 'view', streamId: this.id })
    await this.stop()
    this.stream = new MediaStream()
    this.pc = new RTCPeerConnection()
    await this.start()
  }

  run() {
    // WatchDog, Auto Restart
    const restartStates = [StreamState.Disconnected, StreamState.Closed, StreamState.Failed]
    if (restartStates.find(i => i === this.userStatus.state) && this.remoteStatus.state === StreamState.Connected) this.restart()
  }
}

const contexts: WHEPContext[] = []

export default function useWhepClient(id: string) {
  const newContext = (id: string) => {
    const context = new WHEPContext(id)
    contexts.push(context)
    return context
  }

  const context = contexts.find(ctx => ctx.id === id) || newContext(id)
  return useSyncExternalStore((callback: () => void) => {
    context.addEventListener(event.type, callback)
    return () => {
      context.removeEventListener(event.type, callback)
    }
  }, () => context.export())
}
