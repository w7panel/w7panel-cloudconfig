import { webcrypto } from 'node:crypto'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const axiosMock = vi.hoisted(() => ({
  client: { get: vi.fn(), post: vi.fn() },
  create: vi.fn(),
}))

vi.mock('axios', () => ({
  default: { create: axiosMock.create },
}))

class MemoryStorage {
  constructor() {
    this.values = new Map()
  }

  getItem(key) {
    return this.values.has(key) ? this.values.get(key) : null
  }

  setItem(key, value) {
    this.values.set(key, String(value))
  }

  removeItem(key) {
    this.values.delete(key)
  }
}

function setupWindow({ microapp = true, path = '/', search = '', emit } = {}) {
  globalThis.localStorage = new MemoryStorage()
  globalThis.window = {
    __POWERED_BY_WUJIE__: microapp,
    crypto: webcrypto,
    setTimeout,
    clearTimeout,
    location: { pathname: path, search, hash: '', assign: vi.fn() },
    history: { replaceState: vi.fn() },
  }
  if (microapp) {
    window.$wujie = {
      props: {
        url: 'https://panel.example/backend',
        backendUrl: '/wrong-backend',
        paneltoken: 'panel-proxy-token',
      },
      bus: { $emit: emit || vi.fn() },
    }
  }
}

describe('cloudconfig auth', () => {
  beforeEach(() => {
    vi.resetModules()
    axiosMock.client.get.mockReset()
    axiosMock.client.post.mockReset()
    axiosMock.create.mockReset()
    axiosMock.create.mockReturnValue(axiosMock.client)
  })

  it('exchanges one Wujie OIDC code for concurrent callers', async () => {
    const emit = vi.fn((_event, payload, callback) => {
      expect(payload).toEqual({
        client_id: 'default',
        redirect_uri: 'https://panel.example/callback',
        scope: 'openid profile',
      })
      setTimeout(() => callback('oidc-code'), 0)
    })
    setupWindow({ emit })
    axiosMock.client.get.mockResolvedValue({ data: {
      client_id: 'default',
      redirect_uri: 'https://panel.example/callback',
      scope: 'openid profile',
    } })
    axiosMock.client.post.mockResolvedValue({ data: { access_token: 'signed-id-token' } })
    const auth = await import('./auth.js')

    const [first, second] = await Promise.all([auth.fetchToken(), auth.fetchToken()])

    expect(first).toBe('signed-id-token')
    expect(second).toBe('signed-id-token')
    expect(emit).toHaveBeenCalledTimes(1)
    expect(emit.mock.calls[0][0]).toBe('getOidcCode')
    expect(axiosMock.create).toHaveBeenCalledWith({ baseURL: 'https://panel.example/backend', timeout: 15000 })
    expect(axiosMock.client.get).toHaveBeenCalledWith('/cloudconfig-api/v1/login/config', {
      headers: { 'X-W7Panel-Token': 'panel-proxy-token' },
    })
    expect(axiosMock.client.post).toHaveBeenCalledWith('/cloudconfig-api/v1/login', { code: 'oidc-code' }, {
      headers: { 'X-W7Panel-Token': 'panel-proxy-token' },
    })
    expect(auth.getToken()).toBe('signed-id-token')
  })

  it('validates OAuth state and restores the original location', async () => {
    setupWindow({ microapp: false, path: '/callback', search: '?code=code-1&state=expected' })
    localStorage.setItem('cloudconfig-oauth-state', 'expected')
    localStorage.setItem('cloudconfig-oauth-redirect', '/configs?namespace=default')
    axiosMock.client.post.mockResolvedValue({ data: { access_token: 'callback-token' } })
    const auth = await import('./auth.js')

    await expect(auth.bootstrapAuth()).resolves.toBe('callback-token')
    expect(window.history.replaceState).toHaveBeenCalledWith({}, '', '/configs?namespace=default')
    expect(axiosMock.create).toHaveBeenCalledWith({ baseURL: '', timeout: 15000 })
    expect(axiosMock.client.post).toHaveBeenCalledWith('/cloudconfig-api/v1/login', { code: 'code-1' }, undefined)
    expect(auth.getToken()).toBe('callback-token')
  })

  it('uses the CKM-compatible local panel token fallback', async () => {
    setupWindow()
    delete window.$wujie.props.paneltoken
    localStorage.setItem('panelToken', 'stored-panel-token')
    axiosMock.client.get.mockResolvedValue({ data: {} })
    const auth = await import('./auth.js')

    await auth.getLoginConfig()

    expect(axiosMock.client.get).toHaveBeenCalledWith('/cloudconfig-api/v1/login/config', {
      headers: { 'X-W7Panel-Token': 'stored-panel-token' },
    })
  })

  it('rejects a callback with a mismatched OAuth state', async () => {
    setupWindow({ microapp: false, path: '/callback', search: '?code=code-1&state=wrong' })
    localStorage.setItem('cloudconfig-oauth-state', 'expected')
    const auth = await import('./auth.js')

    await expect(auth.bootstrapAuth()).rejects.toThrow('invalid oauth state')
    expect(axiosMock.client.post).not.toHaveBeenCalled()
  })
})
