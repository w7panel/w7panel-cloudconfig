export function getAPIBaseURL() {
  if (!window.__POWERED_BY_WUJIE__) return ''
  return window.$wujie?.props?.url || ''
}
