const unparse = (text, config) => {
    // http_listen_port: /listen (.*);/i,
    // https_listen_port: /listen (.*) ssl/i,
    const reg = {
        server_name: /server_name[\s](.*);/ig,
        index: /index[\s](.*);/i,
        root: /root[\s](.*);/i,
        ssl_certificate: /ssl_certificate[\s](.*);/i,
        ssl_certificate_key: /ssl_certificate_key[\s](.*);/i
    }
    text = text.replace(/listen[\s](.*);/i, 'listen\t'
        + config['http_listen_port'] + ';')
    text = text.replace(/listen[\s](.*) ssl/i, 'listen\t'
        + config['https_listen_port'] + ' ssl')

    text = text.replace(/listen(.*):(.*);/i, 'listen\t[::]:'
        + config['http_listen_port'] + ';')
    text = text.replace(/listen(.*):(.*) ssl/i, 'listen\t[::]:'
        + config['https_listen_port'] + ' ssl')

    for (let k in reg) {
        text = text.replace(new RegExp(reg[k]), k + '\t' +
            (config[k] !== undefined ? config[k] : ' ') + ';')
    }

    return text
}

export {unparse}
