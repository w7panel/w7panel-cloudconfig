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

export function timeValue(value) {
  const raw = value?.Time || value?.time || value
  if (!raw) return 0
  const date = new Date(raw)
  return Number.isNaN(date.getTime()) ? 0 : date.getTime()
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

function stableStringify(value) {
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`
  if (value && typeof value === 'object') {
    return `{${Object.keys(value)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${stableStringify(value[key])}`)
      .join(',')}}`
  }
  return JSON.stringify(value)
}

function hashString(value) {
  let hash = 2166136261
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return (hash >>> 0).toString(16)
}

export function strategyRevision(strategy = {}) {
  const target = strategy.target || {}
  return hashString(
    stableStringify({
      lastSelectedVersion: strategy.lastSelectedVersion || '',
      mountPath: strategy.mountPath || '',
      target: {
        container: target.container || '',
        group: target.group || '',
        kind: target.kind || '',
        name: target.name || '',
        namespace: target.namespace || '',
      },
      type: strategy.type || '',
    }),
  )
}

export function isStrategyStale(config, strategyId, strategy = null) {
  const applied = (config?.status?.lastApplied || []).find((item) => item.strategyId === strategyId && item.success)
  if (!applied) return true
  if (strategy && applied.strategyRevision && applied.strategyRevision !== strategyRevision(strategy)) return true
  if (config?.status?.revision && applied.revision && applied.revision !== config.status.revision) return true
  const updatedAt = timeValue(config?.status?.updatedAt)
  const appliedAt = timeValue(applied.appliedAt)
  return updatedAt > 0 && (!appliedAt || appliedAt < updatedAt)
}
