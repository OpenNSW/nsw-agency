import { useState, useEffect } from 'react'
import { type AgencyApplication } from '../types'
import { fetchApplications } from '../service'

const PAGE_SIZE = 20

export function useApplicationList(consignmentId: string | undefined) {
  const [applications, setApplications] = useState<AgencyApplication[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)

  const [lastConsignmentId, setLastConsignmentId] = useState(consignmentId)
  if (consignmentId !== lastConsignmentId) {
    setLastConsignmentId(consignmentId)
    setPage(1)
    setApplications([])
    setTotal(0)
    setLoading(true)
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  useEffect(() => {
    const controller = new AbortController()
    async function fetchData() {
      if (!consignmentId) return
      try {
        setLoading(true)
        const result = await fetchApplications({ consignmentId, page, pageSize: PAGE_SIZE }, controller.signal)
        setApplications(result.items)
        setTotal(result.total)
      } catch (error) {
        if (error instanceof Error && error.name === 'AbortError') return
        console.error('Failed to fetch tasks:', error)
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    }

    void fetchData()
    return () => controller.abort()
  }, [consignmentId, page])

  return {
    data: applications,
    status: { loading },
    pagination: { page, setPage, total, totalPages },
  }
}
