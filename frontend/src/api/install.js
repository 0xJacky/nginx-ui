import http from "@/lib/http";

const install = {
    get_lock() {
        return http.get('/install')
    },
    install_nginx_ui(data) {
        return http.post('/install', data)
    }
}

export default install
