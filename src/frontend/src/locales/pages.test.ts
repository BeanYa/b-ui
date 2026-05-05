import { describe, expect, it } from 'vitest'
import en from './en'
import fa from './fa'
import ru from './ru'
import vi from './vi'
import zhcn from './zhcn'
import zhtw from './zhtw'

describe('page route titles', () => {
  it('defines translated titles for cluster detail routes in every locale', () => {
    const locales = [en, fa, ru, vi, zhcn, zhtw]

    for (const messages of locales) {
      expect(messages.pages.clusterNodeDetail).toBeTruthy()
      expect(messages.pages.clusterNodeDetail).not.toBe('pages.clusterNodeDetail')
      expect(messages.pages.clusterScatterTaskResult).toBeTruthy()
      expect(messages.pages.clusterScatterTaskResult).not.toBe('pages.clusterScatterTaskResult')
    }
  })
})
