import axios from 'axios'
import { getAPIBaseURL } from './request-base'

const TOKEN_KEY = 'cloudconfig-token'
const OAUTH_STATE_KEY = 'cloudconfig-oauth-state'
const OAUTH_REDIRECT_KEY = 'cloudconfig-oauth-redirect'

const authRequest = axios.create({
  baseURL: getAPIBaseURL(),
  timeout: 15000,
})

let loginPromise = null

function randomState() {
  const bytes = new Uint8Array(16)
  window.crypto.getRandomValues(bytes)
  return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

export async function getLoginConfig() {
  const { data } = await authRequest.get('/cloudconfig-api/v1/login/config')
  return data
}

export async function exchangeLoginCode(code) {
  const { data } = await authRequest.post('/cloudconfig-api/v1/login', { code })
  setToken(data.access_token)
  return data
}

function requestMicroappCode(config) {
  return new Promise((resolve, reject) => {
    if (!window.$wujie?.bus) {
      reject(new Error('Wujie login bus is unavailable'))
      return
    }
    let settled = false
    const timer = window.setTimeout(() => {
      if (!settled) {
        settled = true
        reject(new Error('获取登录授权码超时'))
      }
    }, 15000)
    try {
      window.$wujie.bus.$emit('getOidcCode', {
        client_id: config.client_id,
        redirect_uri: config.redirect_uri,
        scope: config.scope,
      }, (code) => {
        if (settled) return
        settled = true
        window.clearTimeout(timer)
        if (code) resolve(code)
        else reject(new Error('未获取到登录授权码'))
      })
    } catch (error) {
      window.clearTimeout(timer)
      reject(error)
    }
  })
}

export async function startOAuthLogin(redirect = currentLocation()) {
  const config = await getLoginConfig()
  const state = randomState()
  localStorage.setItem(OAUTH_STATE_KEY, state)
  localStorage.setItem(OAUTH_REDIRECT_KEY, redirect || '/')

  const authURL = new URL(config.authorization_endpoint)
  authURL.searchParams.set('response_type', config.response_type || 'code')
  authURL.searchParams.set('client_id', config.client_id)
  authURL.searchParams.set('redirect_uri', config.redirect_uri)
  authURL.searchParams.set('scope', config.scope || 'openid profile')
  authURL.searchParams.set('state', state)
  window.location.assign(authURL.toString())
}

export function fetchToken(redirect = currentLocation()) {
  const existing = getToken()
  if (existing) return Promise.resolve(existing)
  if (loginPromise) return loginPromise

  loginPromise = (async () => {
    if (window.__POWERED_BY_WUJIE__) {
      const config = await getLoginConfig()
      const code = await requestMicroappCode(config)
      const data = await exchangeLoginCode(code)
      return data.access_token
    }
    await startOAuthLogin(redirect)
    return null
  })().finally(() => {
    loginPromise = null
  })
  return loginPromise
}

export async function bootstrapAuth() {
  if (isOAuthCallback()) {
    const query = new URLSearchParams(window.location.search)
    const error = query.get('error')
    if (error) throw new Error(query.get('error_description') || error)
    const code = query.get('code')
    const state = query.get('state')
    const expectedState = localStorage.getItem(OAUTH_STATE_KEY)
    if (!code) throw new Error('missing oauth code')
    if (!state || !expectedState || state !== expectedState) throw new Error('invalid oauth state')

    await exchangeLoginCode(code)
    const redirect = localStorage.getItem(OAUTH_REDIRECT_KEY) || '/'
    localStorage.removeItem(OAUTH_STATE_KEY)
    localStorage.removeItem(OAUTH_REDIRECT_KEY)
    window.history.replaceState({}, '', redirect)
    return getToken()
  }
  return fetchToken()
}

function isOAuthCallback() {
  return window.location.pathname === '/callback' || window.location.pathname.endsWith('/callback')
}

function currentLocation() {
  return `${window.location.pathname}${window.location.search}${window.location.hash || ''}`
}

export { TOKEN_KEY }
