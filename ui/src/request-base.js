export function getAPIBaseURL() {
  return window.$wujie?.props?.url || ''
}

export function getPanelProxyToken() {
  return window.$wujie?.props?.paneltoken || localStorage.getItem('panelToken') || ''
}

export function getPanelProxyRequestConfig() {
  const token = getPanelProxyToken()
  if (!token) return undefined
  return { headers: { 'X-W7Panel-Token': token } }
}
