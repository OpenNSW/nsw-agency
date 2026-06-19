import { API_BASE_URL } from '@/constants'
import { http } from '@/services/http'
import { type PaginatedResponse, type ReviewResponse } from '@/services/types'
import {
  type AgencyApplication,
  type DownloadMetadataResponse,
  type UploadMetadataRequest,
  type UploadMetadataResponse,
  type UploadResponse,
} from './types'

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

export async function uploadFile(file: File): Promise<UploadResponse> {
  const res = await http.request({
    url: `${API_BASE_URL}/api/v1/storage`,
    method: 'POST',
    data: {
      filename: file.name,
      mime_type: file.type || 'application/octet-stream',
      size: file.size,
    } satisfies UploadMetadataRequest,
    attachToken: true,
  })
  const metadata = res.data as UploadMetadataResponse

  // Upload file bytes directly to the storage destination (presigned URL — no auth header needed)
  const uploadResponse = await fetch(metadata.upload_url, {
    method: 'PUT',
    headers: {
      'Content-Type': file.type || 'application/octet-stream',
    },
    body: file,
  })

  if (!uploadResponse.ok) {
    const errorText = await uploadResponse.text()
    console.error(`Direct storage upload error ${uploadResponse.status}: ${errorText}`)
    throw new Error(`Failed to upload file to storage: ${uploadResponse.status} ${uploadResponse.statusText}`)
  }

  return { key: metadata.key, name: metadata.name }
}

export async function getDownloadUrl(key: string): Promise<{ url: string; expiresAt: number }> {
  const res = await http.request({
    url: `${API_BASE_URL}/api/v1/storage/${key}`,
    method: 'GET',
    attachToken: true,
  })
  const response = res.data as DownloadMetadataResponse

  // Normalize the URL if it's a relative path (common in local dev)
  const url = response.download_url.startsWith('/')
    ? new URL(API_BASE_URL).origin + response.download_url
    : response.download_url

  return { url, expiresAt: response.expires_at }
}
