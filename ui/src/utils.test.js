import { describe, expect, it } from 'vitest'
import { inheritOptionValue, normalizeInherit } from './utils'

describe('inherit value normalization', () => {
  it('normalizes omitted API fields to the same select value as an option', () => {
    const responseValue = inheritOptionValue({ configName: 'base-config' })
    const optionValue = inheritOptionValue({
      configName: 'base-config',
      version: '',
    })

    expect(responseValue).toBe(optionValue)
    expect(JSON.parse(responseValue)).toEqual({
      configName: 'base-config',
      version: '',
    })
  })

  it('preserves an inherited version', () => {
    expect(normalizeInherit({
      configName: 'base-config',
      version: 'production',
    })).toEqual({
      configName: 'base-config',
      version: 'production',
    })
  })
})
