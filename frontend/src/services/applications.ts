import { API_BASE_URL } from '../constants'
import { http } from './http'
import { type AgencyApplication, type PaginatedResponse, type ReviewResponse } from './types'

export async function fetchApplications(
  params?: { status?: string; consignmentId?: string; q?: string; page?: number; pageSize?: number },
  signal?: AbortSignal,
): Promise<PaginatedResponse<AgencyApplication>> {
  const res = await http.request({
    url: `${API_BASE_URL}/api/v1/applications`,
    method: 'GET',
    params: Object.fromEntries(
      Object.entries({
        status: params?.status,
        consignmentId: params?.consignmentId,
        q: params?.q,
        page: params?.page,
        pageSize: params?.pageSize,
      }).filter(([, v]) => v !== undefined),
    ),
    attachToken: true,
    signal,
  })
  return res.data as PaginatedResponse<AgencyApplication>
}

export async function fetchApplicationDetail(taskId: string, signal?: AbortSignal): Promise<AgencyApplication> {
  const res = await http.request({
    url: `${API_BASE_URL}/api/v1/applications/${taskId}`,
    method: 'GET',
    attachToken: true,
    signal,
  })
  return res.data as AgencyApplication
}

export async function submitReview(
  taskId: string,
  formValues: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<ReviewResponse> {
  const res = await http.request({
    url: `${API_BASE_URL}/api/v1/applications/${taskId}/review`,
    method: 'POST',
    data: formValues,
    attachToken: true,
    signal,
  })
  return res.data as ReviewResponse
}

export async function submitFeedback(
  taskId: string,
  content: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<ReviewResponse> {
  const res = await http.request({
    url: `${API_BASE_URL}/api/v1/applications/${taskId}/feedback`,
    method: 'POST',
    data: content,
    attachToken: true,
    signal,
  })
  return res.data as ReviewResponse
}
