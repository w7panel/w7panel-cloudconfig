import axios from 'axios'
import { Message } from '@arco-design/web-vue'
import { clearToken, fetchToken, getToken } from './auth'
import { getAPIBaseURL } from './request-base'

const request = axios.create({
  baseURL: getAPIBaseURL(),
  timeout: 15000,
})

request.interceptors.request.use((config) => {
  const token = getToken()
  config.headers = config.headers || {}
  if (token) {
    config.headers['Authorization-config'] = `Bearer ${token}`
  }
  return config
})

request.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401 && !error.config?._authRetried) {
      clearToken()
      try {
        const token = await fetchToken()
        if (token) {
          error.config._authRetried = true
          error.config.headers = error.config.headers || {}
          error.config.headers['Authorization-config'] = `Bearer ${token}`
          return request(error.config)
        }
      } catch {
        // Preserve the original API error below.
      }
    }
    Message.error(error.response?.data?.message || error.message || '请求失败')
    return Promise.reject(error)
  },
)

export function listConfigs(namespace) {
  return request.get('/cloudconfig-api/v1/configs', { params: { namespace } }).then((r) => r.data || [])
}

export function createConfig(data, namespace) {
  return request.post('/cloudconfig-api/v1/configs', data, { params: { namespace } }).then((r) => r.data)
}

export function updateConfig(namespace, name, data) {
  return request.put(`/cloudconfig-api/v1/configs/${namespace}/${name}`, data).then((r) => r.data)
}

export function deleteConfig(namespace, name) {
  return request.delete(`/cloudconfig-api/v1/configs/${namespace}/${name}`).then((r) => r.data)
}

export function resolveConfig(namespace, name, version = '') {
  const params = version === undefined || version === null ? {} : { version }
  return request.get(`/cloudconfig-api/v1/configs/${namespace}/${name}/resolved`, { params }).then((r) => r.data)
}

export function listTargets(namespace) {
  return request.get('/cloudconfig-api/v1/targets', { params: { namespace } }).then((r) => r.data || [])
}

export function applyStrategy(namespace, name, strategyId, data) {
  return request.post(`/cloudconfig-api/v1/configs/${namespace}/${name}/strategies/${strategyId}/apply`, data).then((r) => r.data)
}
