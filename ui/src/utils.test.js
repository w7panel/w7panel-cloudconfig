import { describe, expect, it } from 'vitest'
import { inheritOptionValue, normalizeInherit, normalizeVersions, parseQuick, versionsOf } from './utils'

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

describe('configuration versions', () => {
  it('keeps the explicit pool order and appends legacy item versions', () => {
    expect(versionsOf([{ spec: {
      versions: ['prod', ' dev ', 'prod'],
      items: [{ version: 'staging' }, { version: 'dev' }],
      inherit: { version: 'parent-only' },
    } }])).toEqual(['prod', 'dev', 'staging'])
  })

  it('normalizes an editable version pool', () => {
    expect(normalizeVersions([' prod ', '', 'dev', 'prod'])).toEqual(['prod', 'dev'])
  })
})

describe('bulk text import', () => {
  it('keeps equals signs in values and supports empty values', () => {
    expect(parseQuick('TOKEN=a=b\nEMPTY\n', 'prod')).toEqual([
      { version: 'prod', name: 'TOKEN', value: 'a=b', remark: '' },
      { version: 'prod', name: 'EMPTY', value: '', remark: '' },
    ])
  })
})
