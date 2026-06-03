import { getEnv } from '../runtimeConfig'

// Empty string means same-origin (integrated deployment where Go serves both API and frontend).
// Set VITE_API_BASE_URL only when the backend is on a different origin.
export const API_BASE_URL = getEnv('VITE_API_BASE_URL', '')
