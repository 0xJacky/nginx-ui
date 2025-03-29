import _ from 'lodash'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'

NProgress.configure({ showSpinner: false, trickleSpeed: 300 })

const done = _.debounce(NProgress.done, 300, {
  leading: false,
  trailing: true,
})

export function useNProgress() {
  return {
    start: NProgress.start,
    done,
  }
}
