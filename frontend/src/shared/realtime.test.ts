import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import type { Envelope } from './realtime'
import { RealtimeClient } from './realtime'

// MockWebSocket implements just enough of the browser WebSocket surface
// for RealtimeClient: readyState, send/close, and the four event
// handlers it assigns. Tests drive it manually (calling mock.open() /
// mock.message() / mock.close()) instead of a real network socket.
class MockWebSocket {
  static OPEN = 1
  static CLOSED = 3
  readyState = 0
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null

  send(data: string) {
    this.sent.push(data)
  }
  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.()
  }
  open() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.()
  }
  message(env: Envelope) {
    this.onmessage?.({ data: JSON.stringify(env) })
  }
}

function makeClient() {
  const sockets: MockWebSocket[] = []
  const factory = () => {
    const ws = new MockWebSocket()
    sockets.push(ws)
    return ws as unknown as WebSocket
  }
  const client = new RealtimeClient('ws://test/api/ws', factory)
  return { client, sockets }
}

beforeEach(() => {
  vi.useFakeTimers()
})
afterEach(() => {
  vi.useRealTimers()
})

test('subscribing before connect sends the subscribe control message once open', () => {
  const { client, sockets } = makeClient()
  const handler = vi.fn()
  client.subscribe('ops.health', handler)
  client.connect()
  sockets[0].open()

  const sent = sockets[0].sent.map((s) => JSON.parse(s))
  expect(sent).toContainEqual({ topic: 'ops.health', type: 'subscribe' })
})

test('delivers an update envelope only to handlers subscribed to its topic', () => {
  const { client, sockets } = makeClient()
  const healthHandler = vi.fn()
  const otherHandler = vi.fn()
  client.subscribe('ops.health', healthHandler)
  client.subscribe('ops.audit', otherHandler)
  client.connect()
  sockets[0].open()

  sockets[0].message({ topic: 'ops.health', type: 'update', payload: { n: 1 } })

  expect(healthHandler).toHaveBeenCalledTimes(1)
  expect(otherHandler).not.toHaveBeenCalled()
})

test('unsubscribe stops delivery and sends the unsubscribe control message', () => {
  const { client, sockets } = makeClient()
  const handler = vi.fn()
  client.subscribe('ops.health', handler)
  client.connect()
  sockets[0].open()
  sockets[0].sent = [] // clear the initial subscribe send

  client.unsubscribe('ops.health', handler)
  sockets[0].message({ topic: 'ops.health', type: 'update' })

  expect(handler).not.toHaveBeenCalled()
  const sent = sockets[0].sent.map((s) => JSON.parse(s))
  expect(sent).toContainEqual({ topic: 'ops.health', type: 'unsubscribe' })
})

test('reconnects with growing, capped backoff after an unexpected close', () => {
  const { client, sockets } = makeClient()
  client.connect()
  sockets[0].open()
  sockets[0].close() // simulate a drop, not client.disconnect()

  // First reconnect attempt: base delay.
  vi.advanceTimersByTime(1000)
  expect(sockets.length).toBe(2)

  sockets[1].close() // drop again immediately
  const beforeSecondAttempt = sockets.length
  vi.advanceTimersByTime(200) // well under even the base delay
  expect(sockets.length).toBe(beforeSecondAttempt) // hasn't retried yet — backoff, not instant
  vi.advanceTimersByTime(5000)
  expect(sockets.length).toBeGreaterThan(beforeSecondAttempt)
})

test('resubscribes to all active topics after a reconnect', () => {
  const { client, sockets } = makeClient()
  const handler = vi.fn()
  client.subscribe('ops.health', handler)
  client.connect()
  sockets[0].open()
  sockets[0].close()

  vi.advanceTimersByTime(2000)
  expect(sockets.length).toBe(2)
  sockets[1].open()

  const sent = sockets[1].sent.map((s) => JSON.parse(s))
  expect(sent).toContainEqual({ topic: 'ops.health', type: 'subscribe' })
})

test('disconnect() does not trigger a reconnect', () => {
  const { client, sockets } = makeClient()
  client.connect()
  sockets[0].open()
  client.disconnect()

  vi.advanceTimersByTime(20000)
  expect(sockets.length).toBe(1)
})
