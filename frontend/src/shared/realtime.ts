// frontend/src/shared/realtime.ts
//
// Framework-agnostic WebSocket client for the platform's real-time shell
// (see docs/superpowers/specs/2026-08-15-websockets-shell-design.md).
// No UI, no consumer-specific payload parsing — subscribe to a topic,
// get envelopes. useRealtimeTopic (a later addition) wraps this in a
// React hook for consumers to actually use.

export type MessageType = 'update' | 'subscribe' | 'unsubscribe'

export type Envelope<T = unknown> = {
  topic: string
  type: MessageType
  payload?: T
}

type Handler = (env: Envelope) => void

const RECONNECT_BASE_DELAY_MS = 500
const RECONNECT_MAX_DELAY_MS = 15000

export class RealtimeClient {
  private ws: WebSocket | null = null
  private topics = new Map<string, Set<Handler>>()
  private reconnectAttempt = 0
  private closedByUser = false
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private url: string
  private wsFactory: (url: string) => WebSocket

  constructor(url: string, wsFactory: (url: string) => WebSocket = (u) => new WebSocket(u)) {
    this.url = url
    this.wsFactory = wsFactory
  }

  // Idempotent — a second call while already connected/connecting is a
  // no-op, so a component mounting while another already opened the
  // shared client doesn't spawn a second socket.
  connect() {
    this.closedByUser = false
    if (this.ws && this.ws.readyState !== WebSocket.CLOSED) {
      return
    }
    this.open()
  }

  private open() {
    const ws = this.wsFactory(this.url)
    this.ws = ws
    ws.onopen = () => {
      this.reconnectAttempt = 0
      for (const topic of this.topics.keys()) {
        this.sendControl('subscribe', topic)
      }
    }
    ws.onmessage = (ev: MessageEvent) => {
      let env: Envelope
      try {
        env = JSON.parse(ev.data as string) as Envelope
      } catch (err) {
        console.error('realtime: failed to parse incoming message', err)
        return
      }
      const handlers = this.topics.get(env.topic)
      handlers?.forEach((h) => {
        try {
          h(env)
        } catch (err) {
          console.error('realtime: handler threw for topic', env.topic, err)
        }
      })
    }
    ws.onclose = () => {
      if (!this.closedByUser) {
        this.scheduleReconnect()
      }
    }
    ws.onerror = () => {
      ws.close()
    }
  }

  private scheduleReconnect() {
    const delay = Math.min(RECONNECT_BASE_DELAY_MS * 2 ** this.reconnectAttempt, RECONNECT_MAX_DELAY_MS)
    // Jitter (50-100% of delay) so many tabs/clients reconnecting after
    // the same server restart don't all retry in lockstep.
    const jitter = delay * (0.5 + Math.random() * 0.5)
    this.reconnectAttempt++
    this.reconnectTimer = setTimeout(() => this.open(), jitter)
  }

  disconnect() {
    this.closedByUser = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.ws?.close()
  }

  // Returns an unsubscribe function, so a caller (e.g. useRealtimeTopic's
  // effect cleanup) doesn't need to keep the handler reference around
  // separately just to remove it later.
  subscribe(topic: string, handler: Handler): () => void {
    if (!this.topics.has(topic)) {
      this.topics.set(topic, new Set())
    }
    this.topics.get(topic)!.add(handler)
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.sendControl('subscribe', topic)
    }
    return () => this.unsubscribe(topic, handler)
  }

  unsubscribe(topic: string, handler: Handler) {
    const handlers = this.topics.get(topic)
    if (!handlers) return
    handlers.delete(handler)
    if (handlers.size === 0) {
      this.topics.delete(topic)
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.sendControl('unsubscribe', topic)
      }
    }
  }

  private sendControl(type: 'subscribe' | 'unsubscribe', topic: string) {
    this.ws?.send(JSON.stringify({ topic, type }))
  }
}

function realtimeUrl(): string {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${location.host}/api/ws`
}

// App-wide singleton. Connecting is lazy (see useRealtimeTopic) — a page
// with no real-time consumer never opens a socket at all. One shared
// connection per tab, not one per consumer.
export const realtimeClient = new RealtimeClient(typeof location !== 'undefined' ? realtimeUrl() : '')
