import { describe, expect, it } from 'vitest'

import {
  buildClusterDomainActionTree,
  flattenVisibleClusterDomainActionTree,
} from './clusterDomainActionTree'

describe('buildClusterDomainActionTree', () => {
  it('treats null input as no actions', () => {
    expect(() =>
      buildClusterDomainActionTree(null),
    ).not.toThrow()
    expect(buildClusterDomainActionTree(null)).toEqual([])
  })

  it('merges shared prefixes and preserves first-seen top-level order', () => {
    const tree = buildClusterDomainActionTree([
      'domain.cluster.changed',
      'domain.panel.update.available',
      'events',
      'heartbeat',
      '',
      'domain.cluster.changed',
    ])

    expect(tree.map(node => node.key)).toEqual([
      'domain',
      'events',
      'heartbeat',
    ])
    expect(tree[0].children.map(node => node.key)).toEqual([
      'domain.cluster',
      'domain.panel',
    ])
    expect(tree[0].children[0].children[0].key).toBe('domain.cluster.changed')
    expect(tree[0].children[1].children[0].key).toBe('domain.panel.update')
  })

  it('ignores malformed action values instead of normalizing them', () => {
    const tree = buildClusterDomainActionTree([
      'domain..cluster',
      '.events',
      'events.',
      'domain. .cluster',
      'valid.action',
    ])

    expect(tree.map(node => node.key)).toEqual(['valid'])
    expect(tree[0].children.map(node => node.key)).toEqual(['valid.action'])
  })

  it('keeps prefix actions marked as actions when they also have children', () => {
    const tree = buildClusterDomainActionTree([
      'domain.panel',
      'domain.panel.update',
    ])

    expect(tree[0].isAction).toBe(false)
    expect(tree[0].children[0]).toMatchObject({
      key: 'domain.panel',
      isAction: true,
    })
    expect(tree[0].children[0].children[0]).toMatchObject({
      key: 'domain.panel.update',
      isAction: true,
    })
  })
})

describe('flattenVisibleClusterDomainActionTree', () => {
  const tree = buildClusterDomainActionTree([
    'domain.cluster.changed',
    'domain.panel.update.available',
    'events',
  ])

  it('shows only top-level rows when no keys are expanded', () => {
    expect(
      flattenVisibleClusterDomainActionTree(tree, new Set()).map(row => row.key),
    ).toEqual(['domain', 'events'])
  })

  it('reveals only the next depth for expanded branches', () => {
    expect(
      flattenVisibleClusterDomainActionTree(
        tree,
        new Set(['domain', 'domain.panel', 'domain.panel.update']),
      ).map(row => `${row.depth}:${row.key}`),
    ).toEqual([
      '0:domain',
      '1:domain.cluster',
      '1:domain.panel',
      '2:domain.panel.update',
      '3:domain.panel.update.available',
      '0:events',
    ])
  })

  it('preserves action state for rows that are both actions and branches', () => {
    const prefixedTree = buildClusterDomainActionTree([
      'domain.panel',
      'domain.panel.update',
    ])

    expect(
      flattenVisibleClusterDomainActionTree(
        prefixedTree,
        new Set(['domain', 'domain.panel']),
      ),
    ).toEqual([
      {
        key: 'domain',
        label: 'domain',
        depth: 0,
        hasChildren: true,
        isAction: false,
      },
      {
        key: 'domain.panel',
        label: 'panel',
        depth: 1,
        hasChildren: true,
        isAction: true,
      },
      {
        key: 'domain.panel.update',
        label: 'update',
        depth: 2,
        hasChildren: false,
        isAction: true,
      },
    ])
  })

  it('returns an empty row list when the action list is empty', () => {
    expect(flattenVisibleClusterDomainActionTree([], new Set())).toEqual([])
  })
})
