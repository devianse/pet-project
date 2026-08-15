import { useEffect } from 'react'
import { realtimeClient, type Envelope } from './realtime'

// Subscribes to `topic` for the lifetime of the calling component,
// connecting the shared client lazily on first use. onMessage should be
// stable across renders (e.g. wrapped in useCallback by the caller) —
// like any useEffect dependency, an inline function here resubscribes
// on every render.
export function useRealtimeTopic(topic: string, onMessage: (env: Envelope) => void): void {
  useEffect(() => {
    realtimeClient.connect()
    return realtimeClient.subscribe(topic, onMessage)
  }, [topic, onMessage])
}
