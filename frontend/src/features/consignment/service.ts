import { API_BASE_URL } from '../../constants'
import { http } from '../../services/http'
import { type ConsignmentSummary } from './types'
import { type PaginatedResponse } from '../../services/types'

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
