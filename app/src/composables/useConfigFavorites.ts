import { useStorage } from '@vueuse/core'

export interface ConfigFavorite {
  name: string
  dir: string
  fullPath: string
  modifiedAt: string
}

const STORAGE_KEY = 'nginx-ui-config-favorites'

export function useConfigFavorites() {
  const favorites = useStorage<ConfigFavorite[]>(STORAGE_KEY, [])

  function isFavorite(fullPath: string) {
    return favorites.value.some(item => item.fullPath === fullPath)
  }

  function addFavorite(name: string, dir: string, modifiedAt?: string) {
    const fullPath = dir ? `${dir}/${name}` : name
    if (isFavorite(fullPath))
      return

    favorites.value.push({
      name,
      dir,
      fullPath,
      modifiedAt: modifiedAt || '',
    })
  }

  function removeFavorite(fullPath: string) {
    favorites.value = favorites.value.filter(item => item.fullPath !== fullPath)
  }

  function toggleFavorite(name: string, dir: string, modifiedAt?: string) {
    const fullPath = dir ? `${dir}/${name}` : name
    if (isFavorite(fullPath))
      removeFavorite(fullPath)
    else
      addFavorite(name, dir, modifiedAt)
  }

  return {
    favorites,
    isFavorite,
    addFavorite,
    removeFavorite,
    toggleFavorite,
  }
}
