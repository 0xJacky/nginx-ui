import type { Bread } from '@/components/Breadcrumb/types'
import { describe, expect, test } from 'bun:test'
import { collapseCrumbs, withoutHome } from '@/components/Breadcrumb/crumbs'

function bread(name: string, path = `/${name}`): Bread {
  return { name, path, translatedName: () => name }
}

function names(entries: ReturnType<typeof collapseCrumbs>) {
  return entries.map(entry => entry.type === 'item'
    ? entry.bread.name
    : `[${entry.breads.map(b => b.name).join(',')}]`)
}

describe('breadcrumb home crumb', () => {
  test('drops the route-derived home crumb', () => {
    const result = withoutHome([bread('Home', '/'), bread('Dashboard'), bread('Server')])
    expect(result.map(b => b.name)).toEqual(['Dashboard', 'Server'])
  })

  test('drops a hand-built home crumb by name', () => {
    const result = withoutHome([bread('Home', ''), bread('Manage Configs', '/config')])
    expect(result.map(b => b.name)).toEqual(['Manage Configs'])
  })
})

describe('breadcrumb collapsing', () => {
  test('keeps short trails intact', () => {
    expect(names(collapseCrumbs([bread('a'), bread('b'), bread('c'), bread('d')]))).toEqual(['a', 'b', 'c', 'd'])
  })

  test('folds the middle of long trails, keeping the first and last two', () => {
    const trail = ['config', 'conf.d', 'sites', 'api', 'edit'].map(n => bread(n))
    expect(names(collapseCrumbs(trail))).toEqual(['config', '[conf.d,sites]', 'api', 'edit'])
  })

  test('respects a custom limit', () => {
    const trail = ['a', 'b', 'c', 'd', 'e', 'f'].map(n => bread(n))
    expect(names(collapseCrumbs(trail, 5))).toEqual(['a', '[b,c]', 'd', 'e', 'f'])
  })

  test('always leaves the current page visible', () => {
    const trail = ['a', 'b', 'c'].map(n => bread(n))
    expect(names(collapseCrumbs(trail, 2))).toEqual(['a', '[b]', 'c'])
  })
})
