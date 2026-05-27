import { useMemo } from 'react'
import { useThunderID } from '@thunderid/react'
import {
  fetchConsignments as _fetchConsignments,
  fetchApplications as _fetchApplications,
  fetchApplicationDetail as _fetchApplicationDetail,
  submitReview as _submitReview,
  submitFeedback as _submitFeedback,
} from '../api'
import { uploadFile as _uploadFile, getDownloadUrl as _getDownloadUrl } from './storage'

/**
 * Single hook for all authenticated API operations.
 *
 * Uses ThunderID's built-in http.request (token injected automatically via
 * attachToken: true) — no custom ApiClient, no context provider needed.
 *
 * All domain methods are pre-bound so callers never pass a transport manually.
 */
export function useApi() {
  const { http } = useThunderID()

  return useMemo(
    () => ({
      // ── Consignment / Application domain ──────────────────────────────────
      fetchConsignments: (params?: Parameters<typeof _fetchConsignments>[1], signal?: AbortSignal) =>
        _fetchConsignments(http.request, params, signal),

      fetchApplications: (params?: Parameters<typeof _fetchApplications>[1], signal?: AbortSignal) =>
        _fetchApplications(http.request, params, signal),

      fetchApplicationDetail: (taskId: string, signal?: AbortSignal) =>
        _fetchApplicationDetail(http.request, taskId, signal),

      submitReview: (taskId: string, formValues: Record<string, unknown>, signal?: AbortSignal) =>
        _submitReview(http.request, taskId, formValues, signal),

      submitFeedback: (taskId: string, content: Record<string, unknown>, signal?: AbortSignal) =>
        _submitFeedback(http.request, taskId, content, signal),

      // ── Storage ───────────────────────────────────────────────────────────
      uploadFile: (file: File) => _uploadFile(http.request, file),
      getDownloadUrl: (key: string) => _getDownloadUrl(http.request, key),
    }),
    [http.request],
  )
}

export type BoundApiClient = ReturnType<typeof useApi>
