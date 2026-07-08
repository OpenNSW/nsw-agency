import i18n from '@/i18n'

export const formatDateForTable = (dateString?: string) => {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleDateString(i18n.resolvedLanguage || undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}
