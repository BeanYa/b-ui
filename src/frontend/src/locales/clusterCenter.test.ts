import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import en from './en'
import fa from './fa'
import ru from './ru'
import vi from './vi'
import zhcn from './zhcn'
import zhtw from './zhtw'

describe('cluster center locale copy', () => {
  const locales = [en, fa, ru, vi, zhcn, zhtw]

  it('defines the join-domain dialog and toast copy in every locale', () => {
    const requiredKeys = [
      'displayName',
      'displayNameHint',
      'alreadyExists',
      'alreadyExistsHint',
      'pullDomain',
      'stepDomainInfo',
      'stepDisplayName',
      'successRegistered',
    ] as const
    const requiredFieldKeys = ['joinUri', 'localBaseUrl'] as const
    const requiredActionKeys = ['manage', 'pingAll', 'confirmRegister'] as const

    for (const messages of locales) {
      for (const key of requiredKeys) {
        expect(messages.clusterCenter[key]).toBeTruthy()
        expect(messages.clusterCenter[key]).not.toBe(`clusterCenter.${key}`)
      }
      for (const key of requiredFieldKeys) {
        expect(messages.clusterCenter.fields[key]).toBeTruthy()
        expect(messages.clusterCenter.fields[key]).not.toBe(`clusterCenter.fields.${key}`)
      }
      for (const key of requiredActionKeys) {
        expect(messages.clusterCenter.actions[key]).toBeTruthy()
        expect(messages.clusterCenter.actions[key]).not.toBe(`clusterCenter.actions.${key}`)
      }
      expect(messages.clusterCenter.validation.displayName).toBeTruthy()
      expect(messages.clusterCenter.validation.invalidJoinUri).toBeTruthy()
      expect(messages.clusterCenter.joinUriHint).toBeTruthy()
    }
  })

  it('does not add a trailing Chinese full stop to the join-domain success toast', () => {
    expect(zhcn.clusterCenter.successRegistered).toBe('集群节点已加入域并完成信息刷新')
    expect(zhtw.clusterCenter.successRegistered).toBe('叢集節點已加入域並完成資訊重新整理')
  })

  it('uses localized copy for the invalid join URI toast', () => {
    const source = readFileSync(fileURLToPath(new URL('../views/ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain("i18n.global.t('clusterCenter.validation.invalidJoinUri')")
    expect(source).not.toContain('URI 格式无效，请检查后重试')
  })

  it('uses localized copy in the join-domain dialogs and member actions', () => {
    const source = readFileSync(fileURLToPath(new URL('../views/ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain("$t('clusterCenter.fields.joinUri')")
    expect(source).toContain("$t('clusterCenter.joinUriHint')")
    expect(source).toContain("$t('clusterCenter.actions.manage')")
    expect(source).toContain("$t('clusterCenter.actions.pingAll')")
    expect(source).toContain("$t('clusterCenter.fields.localBaseUrl')")
    expect(source).toContain("$t('clusterCenter.actions.confirmRegister')")
    expect(source).not.toContain('Hub 地址')
    expect(source).not.toContain('本机地址')
    expect(source).not.toContain('确认注册')
    expect(source).not.toContain('>管理<')
    expect(source).not.toContain('Ping All')
  })
})
