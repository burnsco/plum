import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useLocation } from 'react-router-dom'
import { useIdentifyLibrary, useLibraries } from '../queries'

export type IdentifyLibraryPhase = 'queued' | 'identifying' | 'soft-reveal' | 'identify-failed' | 'complete'

type QueueIdentifyOptions = {
  prioritize?: boolean
  resetState?: boolean
}

type IdentifyQueueContextValue = {
  activeLibraryId: number | null
  identifyPhases: Record<number, IdentifyLibraryPhase>
  getLibraryPhase: (libraryId: number | null) => IdentifyLibraryPhase | undefined
  queueLibraryIdentify: (libraryId: number, options?: QueueIdentifyOptions) => void
}

const IDENTIFY_SOFT_REVEAL_MS = 90_000
const IDENTIFY_HARD_TIMEOUT_MS = 180_000

const IdentifyQueueContext = createContext<IdentifyQueueContextValue | null>(null)

function getRouteLibraryId(pathname: string): number | null {
  const match = pathname.match(/\/library\/(\d+)/)
  if (!match) return null
  const id = parseInt(match[1], 10)
  return Number.isFinite(id) ? id : null
}

export function IdentifyQueueProvider({ children }: { children: ReactNode }) {
  const location = useLocation()
  const { data: libraries = [] } = useLibraries()
  const identifyMutation = useIdentifyLibrary()
  const identifyLibraryInBackground = identifyMutation.mutateAsync
  const queuedLibsRef = useRef<Set<number>>(new Set())
  const identifyPhasesRef = useRef<Map<number, IdentifyLibraryPhase>>(new Map())
  const identifyRetryCountsRef = useRef<Map<number, number>>(new Map())
  const identifyOrderRef = useRef<number[]>([])
  const identifyPumpRunningRef = useRef(false)
  const [activeLibraryId, setActiveLibraryId] = useState<number | null>(null)
  const [identifyPhases, setIdentifyPhases] = useState<Record<number, IdentifyLibraryPhase>>({})
  const routeLibraryId = useMemo(() => getRouteLibraryId(location.pathname), [location.pathname])

  const setLibraryIdentifyPhase = useCallback(
    (libraryId: number, phase: IdentifyLibraryPhase | null) => {
      if (phase == null) {
        identifyPhasesRef.current.delete(libraryId)
      } else {
        identifyPhasesRef.current.set(libraryId, phase)
      }
      setIdentifyPhases((current) => {
        if (phase == null) {
          if (!(libraryId in current)) return current
          const next = { ...current }
          delete next[libraryId]
          return next
        }
        if (current[libraryId] === phase) return current
        return { ...current, [libraryId]: phase }
      })
    },
    [],
  )

  const identifyLibraryWithTimers = useCallback(
    (libraryId: number) => {
      const controller = new AbortController()
      return new Promise<void>((resolve, reject) => {
        const softRevealId = window.setTimeout(() => {
          if (identifyPhasesRef.current.get(libraryId) === 'identifying') {
            setLibraryIdentifyPhase(libraryId, 'soft-reveal')
          }
        }, IDENTIFY_SOFT_REVEAL_MS)
        const hardTimeoutId = window.setTimeout(() => {
          controller.abort()
          reject(new Error('identify-timeout'))
        }, IDENTIFY_HARD_TIMEOUT_MS)

        identifyLibraryInBackground({ libraryId, signal: controller.signal })
          .then(() => resolve())
          .catch((error) => {
            if (controller.signal.aborted) {
              reject(new Error('identify-timeout'))
              return
            }
            reject(error)
          })
          .finally(() => {
            window.clearTimeout(softRevealId)
            window.clearTimeout(hardTimeoutId)
          })
      })
    },
    [identifyLibraryInBackground, setLibraryIdentifyPhase],
  )

  const pumpIdentifyQueue = useCallback(async () => {
    if (identifyPumpRunningRef.current) return
    identifyPumpRunningRef.current = true
    try {
      while (true) {
        const nextLibraryId = identifyOrderRef.current.find((libraryId) =>
          queuedLibsRef.current.has(libraryId),
        )
        if (nextLibraryId == null) return

        queuedLibsRef.current.delete(nextLibraryId)
        setActiveLibraryId(nextLibraryId)
        setLibraryIdentifyPhase(nextLibraryId, 'identifying')

        try {
          await identifyLibraryWithTimers(nextLibraryId)
          identifyRetryCountsRef.current.delete(nextLibraryId)
          setLibraryIdentifyPhase(nextLibraryId, 'complete')
        } catch (error) {
          if (error instanceof Error && error.message === 'identify-timeout') {
            identifyRetryCountsRef.current.delete(nextLibraryId)
            setLibraryIdentifyPhase(nextLibraryId, 'identify-failed')
            continue
          }
          const retries = identifyRetryCountsRef.current.get(nextLibraryId) ?? 0
          if (retries < 1) {
            identifyRetryCountsRef.current.set(nextLibraryId, retries + 1)
            queuedLibsRef.current.add(nextLibraryId)
            setLibraryIdentifyPhase(nextLibraryId, 'queued')
            continue
          }
          identifyRetryCountsRef.current.delete(nextLibraryId)
          setLibraryIdentifyPhase(nextLibraryId, 'identify-failed')
        } finally {
          setActiveLibraryId((current) => (current === nextLibraryId ? null : current))
        }
      }
    } finally {
      identifyPumpRunningRef.current = false
    }
  }, [identifyLibraryWithTimers, setLibraryIdentifyPhase])

  const queueLibraryIdentify = useCallback(
    (libraryId: number, options?: QueueIdentifyOptions) => {
      if (options?.resetState) setLibraryIdentifyPhase(libraryId, null)
      identifyRetryCountsRef.current.delete(libraryId)
      queuedLibsRef.current.add(libraryId)
      setLibraryIdentifyPhase(libraryId, 'queued')
      if (options?.prioritize) {
        identifyOrderRef.current = [
          libraryId,
          ...identifyOrderRef.current.filter((queuedLibraryId) => queuedLibraryId !== libraryId),
        ]
      }
      void pumpIdentifyQueue()
    },
    [pumpIdentifyQueue, setLibraryIdentifyPhase],
  )

  const getLibraryPhase = useCallback(
    (libraryId: number | null) => (libraryId == null ? undefined : identifyPhases[libraryId]),
    [identifyPhases],
  )

  useEffect(() => {
    const identifyableLibraries = libraries
      .filter((library) => library.type !== 'music')
      .map((library) => library.id)
    const activeIds = new Set(identifyableLibraries)
    identifyOrderRef.current =
      routeLibraryId != null && activeIds.has(routeLibraryId)
        ? [routeLibraryId, ...identifyableLibraries.filter((libraryId) => libraryId !== routeLibraryId)]
        : identifyableLibraries

    for (const libraryId of [...queuedLibsRef.current]) {
      if (!activeIds.has(libraryId)) queuedLibsRef.current.delete(libraryId)
    }
    for (const libraryId of [...identifyPhasesRef.current.keys()]) {
      if (!activeIds.has(libraryId)) setLibraryIdentifyPhase(libraryId, null)
    }
    for (const libraryId of [...identifyRetryCountsRef.current.keys()]) {
      if (!activeIds.has(libraryId)) identifyRetryCountsRef.current.delete(libraryId)
    }

    if (activeLibraryId != null && !activeIds.has(activeLibraryId)) {
      setActiveLibraryId(null)
    }

    for (const libraryId of identifyOrderRef.current) {
      const identifyPhase = identifyPhasesRef.current.get(libraryId)
      if (identifyPhase != null) continue
      if (queuedLibsRef.current.has(libraryId)) continue
      queuedLibsRef.current.add(libraryId)
      setLibraryIdentifyPhase(libraryId, 'queued')
    }

    void pumpIdentifyQueue()
  }, [activeLibraryId, libraries, pumpIdentifyQueue, routeLibraryId, setLibraryIdentifyPhase])

  const value = useMemo<IdentifyQueueContextValue>(
    () => ({
      activeLibraryId,
      identifyPhases,
      getLibraryPhase,
      queueLibraryIdentify,
    }),
    [activeLibraryId, getLibraryPhase, identifyPhases, queueLibraryIdentify],
  )

  return <IdentifyQueueContext.Provider value={value}>{children}</IdentifyQueueContext.Provider>
}

export function useIdentifyQueue() {
  const ctx = useContext(IdentifyQueueContext)
  if (!ctx) throw new Error('useIdentifyQueue must be used within IdentifyQueueProvider')
  return ctx
}
