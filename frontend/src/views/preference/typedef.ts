export interface IData {
    server: {
        http_port: string
        run_mode: string
        jwt_secret: string
        start_cmd: string
        http_challenge_port: string
        github_proxy: string,
        email: string
    },
    nginx_log: {
        access_log_path: string
        error_log_path: string
    },
    openai: {
        model: string
        base_url: string
        proxy: string
        token: string
    },
    git: {
        url: string
        auth_method: string
        username: string
        password: string
        private_key_file_path: string
    }
}
