/**
 * nsw-agency-app–specific upload implementation. Points to this app's backend;
 * when NSW Agency moves to a separate repo, this file can target NSW Agency-specific endpoints/S3 without touching shared UI.
 */
import type { ApiClient } from '../api'
import { API_BASE_URL } from '../constants'

interface UploadMetadataRequest {
  filename: string
  mime_type: string
  size: number
}

interface UploadMetadataResponse {
  key: string
  name: string
  upload_url: string
}

interface DownloadMetadataResponse {
  download_url: string
  expires_at: number
}

export interface UploadResponse {
  key: string
  name: string
}

export async function uploadFile(apiClient: ApiClient, file: File): Promise<UploadResponse> {
  const metadata = await apiClient.post<UploadMetadataRequest, UploadMetadataResponse>('/api/nsw-agency/storage', {
    filename: file.name,
    mime_type: file.type || 'application/octet-stream',
    size: file.size,
  })

  // Upload file bytes directly to the storage destination (presigned URL)
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

export async function getDownloadUrl(apiClient: ApiClient, key: string): Promise<{ url: string; expiresAt: number }> {
  const response = await apiClient.get<DownloadMetadataResponse>(`/api/nsw-agency/storage/${key}`)

  // Normalize the URL if it's a relative path (common in local dev)
  const url = response.download_url.startsWith('/')
    ? new URL(API_BASE_URL).origin + response.download_url
    : response.download_url

  return { url, expiresAt: response.expires_at }
}
