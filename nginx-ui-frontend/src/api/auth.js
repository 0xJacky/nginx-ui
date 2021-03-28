import http from '@/lib/http'
import store from '@/lib/store'

const auth = {
    async login(name, password) {
        return http.post('/login', {
            name: name,
            password: password
        }).then(r => {
            store.dispatch('login', r)
        })
    },
    logout() {
        return http.delete('/logout').then(() => {
            store.dispatch('logout').finally()
        })
    }
}

export default auth
