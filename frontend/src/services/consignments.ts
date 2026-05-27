import { API_BASE_URL } from '../constants'
import { http } from './http'
import { type ConsignmentSummary, type PaginatedResponse } from './types'

export async function fetchConsignments(
  params?: { q?: string; page?: number; pageSize?: number },
  signal?: AbortSignal,
): Promise<PaginatedResponse<ConsignmentSummary>> {
  const res = await http.request({
    url: `${API_BASE_URL}/api/v1/consignments`,
    method: 'GET',
    params: Object.fromEntries(
      Object.entries({ q: params?.q, page: params?.page, pageSize: params?.pageSize }).filter(
        ([, v]) => v !== undefined,
      ),
    ),
    attachToken: true,
    signal,
  })
  return res.data as PaginatedResponse<ConsignmentSummary>
}
