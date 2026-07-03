import { useState, useEffect } from 'react'
import { useDebounce } from '@/hooks/useDebounce'
import { type ConsignmentSummary } from '../types'
import { fetchConsignments } from '../service'

const PAGE_SIZE = 20

export function useConsignmentList(searchTerm: string) {
  const debouncedSearchTerm = useDebounce(searchTerm, 400)
  const [consignments, setConsignments] = useState<ConsignmentSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const [prevDebouncedSearchTerm, setPrevDebouncedSearchTerm] = useState(debouncedSearchTerm)
  if (debouncedSearchTerm !== prevDebouncedSearchTerm) {
    setPrevDebouncedSearchTerm(debouncedSearchTerm)
    setPage(1)
  }

  useEffect(() => {
    const controller = new AbortController()

    async function fetchData(isSilent = false) {
      try {
        if (!isSilent) setLoading(true)
        const result = await fetchConsignments({ page, pageSize: PAGE_SIZE, q: debouncedSearchTerm }, controller.signal)
        if (!controller.signal.aborted) {
          setConsignments(result.items || [])
          setTotal(result.total || 0)
          const maxPages = Math.max(1, Math.ceil((result.total || 0) / PAGE_SIZE))
          if (page > maxPages) setPage(1)
        }
      } catch (error) {
        if (error instanceof Error && error.name === 'AbortError') return
        console.error('Failed to fetch consignments:', error)
      } finally {
        if (!controller.signal.aborted && !isSilent) setLoading(false)
      }
    }
    void fetchData()
    const interval = setInterval(() => void fetchData(true), 15000)
    return () => {
      controller.abort()
      clearInterval(interval)
    }
  }, [page, debouncedSearchTerm])

  const isLoading = loading || searchTerm !== debouncedSearchTerm

  return {
    data: consignments,
    status: { loading: isLoading },
    pagination: { page, setPage, total, totalPages },
  }
}
