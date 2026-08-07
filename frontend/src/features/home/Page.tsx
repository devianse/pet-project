import { useEffect, useState } from 'react'
import { getHealth } from '../../shared/api'

// Phase 1 proof: this page hits the Go backend through the Vite dev proxy
// and renders whatever it says back. Nothing more than a wiring check.
export default function HomePage() {
  const [status, setStatus] = useState<string>('checking...')

  useEffect(() => {
    getHealth()
      .then((data) => setStatus(data.status))
      .catch(() => setStatus('unreachable'))
  }, [])

  return (
    <div>
      <h1>pet-projects</h1>
      <p>Backend health: {status}</p>
    </div>
  )
}
