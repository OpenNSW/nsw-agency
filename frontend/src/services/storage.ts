import { API_BASE_URL } from '../constants'
import { http } from './http'

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

  // Normalize relative download URLs to absolute. When API_BASE_URL is empty or
  // a relative path (same-origin integrated deployment) use window.location.origin.
  const isAbsolute = API_BASE_URL?.startsWith('http://') || API_BASE_URL?.startsWith('https://')
  const origin = isAbsolute ? new URL(API_BASE_URL!).origin : window.location.origin
  const url = response.download_url.startsWith('/') ? origin + response.download_url : response.download_url

  return { url, expiresAt: response.expires_at }
}
