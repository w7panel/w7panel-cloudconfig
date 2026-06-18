import axios from 'axios'
import { Message } from '@arco-design/web-vue'

const request = axios.create({
  baseURL: window?.$wujie?.props?.backendUrl || '',
  timeout: 15000,
})

request.interceptors.request.use((config) => {
  const token = window?.$wujie?.props?.paneltoken || localStorage.getItem('panelToken') || localStorage.getItem('token')
  config.headers = config.headers || {}
  if (token) {
    config.headers['X-W7Panel-Token'] = token
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

request.interceptors.response.use(
  (response) => response,
  (error) => {
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
  return request.get(`/cloudconfig-api/v1/configs/${namespace}/${name}/resolved`, { params: { version } }).then((r) => r.data)
}

export function listTargets(namespace) {
  return request.get('/cloudconfig-api/v1/targets', { params: { namespace } }).then((r) => r.data || [])
}

export function applyStrategy(namespace, name, strategyId, data) {
  return request.post(`/cloudconfig-api/v1/configs/${namespace}/${name}/strategies/${strategyId}/apply`, data).then((r) => r.data)
}
