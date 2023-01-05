function changeCss(css: string, value: string) {
    const body = document.body.style
    body.setProperty(css, value)
}

function changeTheme(theme: string) {
    const head = document.head
    document.getElementById('theme')?.remove()
    const styleDom = document.createElement('style')
    styleDom.id = 'theme'
    styleDom.innerHTML = theme
    head.appendChild(styleDom)
}

export const dark_mode = async (enabled: Boolean) => {
    document.body.setAttribute('class', enabled ? 'dark' : 'light')
    if (enabled) {
        changeTheme((await import('@/dark.less?inline')).default)
        changeCss('--page-bg-color', '#141414')
        changeCss('--head-bg-color', 'rgba(0, 0, 0, 0.5)')
        changeCss('--line-color', '#2e2e2e')
        changeCss('--content-bg-color', 'rgb(255 255 255 / 4%)')
        changeCss('--text-color', 'rgba(255, 255, 255, 0.85)')
    } else {
        changeTheme((await import('@/style.less?inline')).default)
        changeCss('--page-bg-color', 'white')
        changeCss('--head-bg-color', 'rgba(255, 255, 255, 0.7)')
        changeCss('--line-color', '#e8e8e8')
        changeCss('--content-bg-color', '#f0f2f5')
        changeCss('--text-color', 'rgba(0, 0, 0, 0.85)')
    }
}
