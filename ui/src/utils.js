export function uid(prefix = 'id') {
  return `${prefix}-${Math.random().toString(36).slice(2, 10)}`
}

export function formatDate(value) {
  if (!value) return '-'
  const raw = value.Time || value.time || value
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

export function isRecent(status = {}) {
  const raw = status.updatedAt?.Time || status.updatedAt?.time || status.updatedAt
  if (!raw) return false
  return Date.now() - new Date(raw).getTime() < 24 * 60 * 60 * 1000
}

export function versionsOf(configs = []) {
  const set = new Set()
  configs.forEach((config) => {
    ;(config.spec?.items || []).forEach((item) => item.version && set.add(item.version))
    if (config.spec?.inherit?.version) set.add(config.spec.inherit.version)
  })
  return Array.from(set).sort()
}

export function parseQuick(text, version = '') {
  return String(text || '')
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const index = line.indexOf('=')
      if (index < 0) return { version, name: line, value: '', remark: '' }
      return {
        version,
        name: line.slice(0, index).trim(),
        value: line.slice(index + 1),
        remark: '',
      }
    })
}

export function appliedRevision(config, strategyId) {
  return (config.status?.lastApplied || []).find((item) => item.strategyId === strategyId && item.success)?.revision || ''
}
