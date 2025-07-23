/**
 * PWA related utilities
 */

// Theme colors for different modes
const THEME_COLORS = {
  light: '#ffffff', // White for light mode
  dark: '#141414', // Dark background for dark mode
} as const

/**
 * Update the theme-color meta tag based on current theme
 * @param theme - 'light' or 'dark'
 */
export function updateThemeColor(theme: 'light' | 'dark') {
  const themeColorMeta = document.querySelector('meta[name="theme-color"]') as HTMLMetaElement
  const appleStatusBarMeta = document.querySelector('meta[name="apple-mobile-web-app-status-bar-style"]') as HTMLMetaElement
  const msApplicationTileColorMeta = document.querySelector('meta[name="msapplication-TileColor"]') as HTMLMetaElement

  if (themeColorMeta) {
    const color = THEME_COLORS[theme]
    themeColorMeta.setAttribute('content', color)

    // Also update apple status bar style
    if (appleStatusBarMeta) {
      appleStatusBarMeta.setAttribute('content', theme === 'dark' ? 'black-translucent' : 'default')
    }

    // Update Windows tile color
    if (msApplicationTileColorMeta) {
      msApplicationTileColorMeta.setAttribute('content', color)
    }
  }
}

/**
 * Get the current theme from document body class
 */
export function getCurrentTheme(): 'light' | 'dark' {
  return document.body.classList.contains('dark') ? 'dark' : 'light'
}

/**
 * Initialize PWA theme color based on current theme
 */
export function initPWAThemeColor() {
  const currentTheme = getCurrentTheme()
  updateThemeColor(currentTheme)
}

/**
 * Watch for theme changes and update PWA theme color accordingly
 */
export function watchThemeChanges() {
  // Use MutationObserver to watch for class changes on body
  const observer = new MutationObserver(mutations => {
    mutations.forEach(mutation => {
      if (mutation.type === 'attributes' && mutation.attributeName === 'class') {
        const currentTheme = getCurrentTheme()
        updateThemeColor(currentTheme)
      }
    })
  })

  observer.observe(document.body, {
    attributes: true,
    attributeFilter: ['class'],
  })

  return observer
}
