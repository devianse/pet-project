// Phase 1: just enough to prove frontend -> backend wiring works end to end.
// Grows into a real API client (auth headers, error handling) in phase 2.
export async function getHealth(): Promise<{ status: string }> {
  const res = await fetch('/api/health')
  if (!res.ok) {
    throw new Error(`health check failed: ${res.status}`)
  }
  return res.json()
}
