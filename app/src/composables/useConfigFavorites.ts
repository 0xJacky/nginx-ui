import type { RouteLocationRaw } from 'vue-router'
import { useStorage } from '@vueuse/core'

export interface ConfigFavorite {
  /** File name exactly as the config list reports it, not URL encoded. */
  name: string
  /**
   * Directory the file lives in, in the same shape `ConfigList` builds it:
   * URL-encoded segments with a trailing slash, or an empty string for the
   * Nginx configuration root.
   */
  dir: string
}

const STORAGE_KEY = 'nginx-ui.config.favorites'

// Module scope so the config list, the home page, and any future consumer all
// read and write one reactive list instead of independent copies.
const favorites = useStorage<ConfigFavorite[]>(STORAGE_KEY, [])

/** Identity of a favorite. `dir` already carries its trailing slash. */
function favoriteKey(dir: string, name: string) {
  return `${dir}${name}`
}

/**
 * Route to the editor of a favorite. Mirrors the location `ConfigList` pushes
 * for "Modify" so both entry points land on the same URL.
 */
export function configFavoriteRoute(item: ConfigFavorite): RouteLocationRaw {
  return {
    path: `/config/${encodeURIComponent(item.name)}/edit`,
    query: { basePath: item.dir || undefined },
  }
}

/** Human readable directory of a favorite, empty for the configuration root. */
export function configFavoriteDir(item: ConfigFavorite) {
  return item.dir
    .split('/')
    .filter(Boolean)
    .map(segment => decodeURIComponent(segment))
    .join('/')
}

export function useConfigFavorites() {
  function isFavorite(dir: string, name: string) {
    const key = favoriteKey(dir, name)

    return favorites.value.some(item => favoriteKey(item.dir, item.name) === key)
  }

  function toggleFavorite(dir: string, name: string) {
    const key = favoriteKey(dir, name)

    if (isFavorite(dir, name)) {
      favorites.value = favorites.value.filter(item => favoriteKey(item.dir, item.name) !== key)
      return
    }

    favorites.value = [...favorites.value, { name, dir }]
  }

  /**
   * Mirror a rename that already succeeded on the server. Renaming a directory
   * rewrites the stored directory of every favorite below it.
   */
  function renameFavorites(dir: string, oldName: string, newName: string, isDir: boolean) {
    if (!isDir) {
      const key = favoriteKey(dir, oldName)

      favorites.value = favorites.value.map(item => (
        favoriteKey(item.dir, item.name) === key ? { ...item, name: newName } : item
      ))
      return
    }

    const oldPrefix = `${dir}${encodeURIComponent(oldName)}/`
    const newPrefix = `${dir}${encodeURIComponent(newName)}/`

    favorites.value = favorites.value.map(item => (
      item.dir.startsWith(oldPrefix)
        ? { ...item, dir: `${newPrefix}${item.dir.slice(oldPrefix.length)}` }
        : item
    ))
  }

  /**
   * Drop what a delete that already succeeded removed, so the home page never
   * links to a file that is gone.
   */
  function forgetFavorites(dir: string, name: string, isDir: boolean) {
    if (!isDir) {
      const key = favoriteKey(dir, name)

      favorites.value = favorites.value.filter(item => favoriteKey(item.dir, item.name) !== key)
      return
    }

    const prefix = `${dir}${encodeURIComponent(name)}/`

    favorites.value = favorites.value.filter(item => !item.dir.startsWith(prefix))
  }

  return {
    favorites,
    isFavorite,
    toggleFavorite,
    renameFavorites,
    forgetFavorites,
  }
}
