// frontend/src/features/watchlist/Page.tsx
import { useEffect, useState } from 'react'
import {
  addToWatchlist,
  getWatchlist,
  removeFromWatchlist,
  setWatchlistItemViewed,
  type WatchlistItem,
} from '../../shared/api'
import { Card, RowCard } from '@/components/pouf/surface'
import { Field, Input } from '@/components/pouf/Input'
import { Button, IconButton } from '@/components/pouf/Button'
import { Icon } from '@/components/pouf/Icon'
import { Stack, Row } from '@/components/pouf/layout'

const POSTER_BASE = 'https://image.tmdb.org/t/p/w185'

export default function WatchlistPage() {
  const [items, setItems] = useState<WatchlistItem[]>([])
  const [link, setLink] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set())

  useEffect(() => {
    getWatchlist()
      .then(setItems)
      .catch(() => setError('failed to load watchlist'))
  }, [])

  async function handleAdd() {
    if (link.trim() === '') return
    setError(null)
    setSubmitting(true)
    try {
      const created = await addToWatchlist(link.trim())
      setItems((prev) => [created, ...prev])
      setLink('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to add link')
    } finally {
      setSubmitting(false)
    }
  }

  function toggleExpand(id: number) {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  async function toggleViewed(item: WatchlistItem) {
    const nextViewed = !item.viewed
    setItems((prev) => prev.map((i) => (i.id === item.id ? { ...i, viewed: nextViewed } : i)))
    try {
      await setWatchlistItemViewed(item.id, nextViewed)
    } catch {
      setItems((prev) => prev.map((i) => (i.id === item.id ? { ...i, viewed: item.viewed } : i)))
      setError('failed to update viewed status')
    }
  }

  async function remove(id: number) {
    setError(null)
    try {
      await removeFromWatchlist(id)
      setItems((prev) => prev.filter((i) => i.id !== id))
    } catch {
      setError('failed to remove item')
    }
  }

  return (
    <Stack gap={5}>
      <h1 className="text-2xl font-black text-ink">Watchlist</h1>
      {error && (
        <p className="font-bold text-[var(--on-accent)] bg-orange rounded-xl px-3 py-2 self-start">
          {error}
        </p>
      )}

      <Card>
        <Stack gap={3}>
          <Field label="IMDb link">
            {(id, describedBy) => (
              <Input
                id={id}
                describedBy={describedBy}
                value={link}
                onChange={setLink}
                placeholder="https://www.imdb.com/title/tt0111161/"
              />
            )}
          </Field>
          <Row justify="end">
            <Button onClick={handleAdd} tone="purple" loading={submitting}>
              <Icon name="add" /> Add
            </Button>
          </Row>
        </Stack>
      </Card>

      <Stack gap={2}>
        {items.map((item) => {
          const expanded = expandedIds.has(item.id)
          return (
            <RowCard key={item.id}>
              <Row gap={3} align="top">
                {item.poster_path ? (
                  <img
                    src={`${POSTER_BASE}${item.poster_path}`}
                    alt=""
                    className="w-16 rounded-control shrink-0"
                  />
                ) : (
                  <div className="w-16 h-24 rounded-control bg-bg shrink-0 flex items-center justify-center">
                    <Icon name="photo" />
                  </div>
                )}
                <Stack gap={2}>
                  <Row justify="between">
                    <Row gap={2}>
                      <span className="font-black text-ink">{item.title}</span>
                      {item.release_year && <span className="text-muted">({item.release_year})</span>}
                      <span className="text-xs font-black uppercase px-2 py-1 rounded-full bg-blue text-[var(--on-accent)]">
                        {item.media_type === 'tv' ? 'TV' : 'Movie'}
                      </span>
                    </Row>
                    <Row gap={1}>
                      <IconButton
                        variant={item.viewed ? 'solid' : 'quiet'}
                        tone="mint"
                        size="sm"
                        onClick={() => toggleViewed(item)}
                        label={
                          item.viewed
                            ? `Mark "${item.title}" as unwatched`
                            : `Mark "${item.title}" as watched`
                        }
                        icon={<Icon name="ok" />}
                      />
                      <IconButton
                        variant="quiet"
                        size="sm"
                        onClick={() => toggleExpand(item.id)}
                        label={
                          expanded
                            ? `Collapse details for "${item.title}"`
                            : `Expand details for "${item.title}"`
                        }
                        icon={<Icon name="expand" />}
                      />
                      <IconButton
                        variant="quiet"
                        size="sm"
                        onClick={() => remove(item.id)}
                        label={`Remove "${item.title}" from watchlist`}
                        icon={<Icon name="remove" />}
                      />
                    </Row>
                  </Row>
                  <Row gap={3} align="center">
                    <a
                      href={`https://www.imdb.com/title/${item.imdb_id}/`}
                      target="_blank"
                      rel="noreferrer"
                      className="font-bold underline text-ink"
                    >
                      IMDb
                    </a>
                    <Row gap={1} align="center">
                      <Icon name="star" size="sm" />
                      <span className="text-muted">{item.vote_average.toFixed(1)}</span>
                    </Row>
                    {item.genres && <span className="text-muted">{item.genres}</span>}
                  </Row>
                  {expanded && (
                    <div className="rounded-control bg-bg p-3">
                      <p className="text-ink">{item.overview}</p>
                    </div>
                  )}
                </Stack>
              </Row>
            </RowCard>
          )
        })}
      </Stack>

      <p className="text-xs text-muted">
        This product uses the{' '}
        <a href="https://www.themoviedb.org/" target="_blank" rel="noreferrer" className="underline">
          TMDB
        </a>{' '}
        API but is not endorsed or certified by TMDB.
      </p>
    </Stack>
  )
}
