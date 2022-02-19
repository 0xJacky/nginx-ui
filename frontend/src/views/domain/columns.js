const columns = [{
    title: '配置文件名称',
    dataIndex: 'name',
    edit: {
        type: 'input'
    }
}, {
    title: '网站域名 (server_name)',
    dataIndex: 'server_name',
    edit: {
        type: 'input'
    }
}, {
    title: '网站根目录 (root)',
    dataIndex: 'root',
    edit: {
        type: 'input'
    }
}, {
    title: '网站首页 (index)',
    dataIndex: 'index',
    edit: {
        type: 'input'
    }
}, {
    title: 'http 监听端口',
    dataIndex: 'http_listen_port',
    edit: {
        type: 'number',
        min: 80
    }
}, {
    title: '支持 SSL',
    dataIndex: 'support_ssl',
    edit: {
        type: 'switch',
        event: 'change_support_ssl'
    }
}]

const columnsSSL = [{
    title: '自动续签',
    dataIndex: 'auto_cert',
    edit: {
        type: 'switch',
        event: 'change_auto_cert'
    },
    description: '启用自动续签后，系统将会每小时检测一次该域名证书的信息，' +
        '如果距离上次签发已超过1个月，则将执行自动续签。' +
        '<br/>启用前先点击下方「自动申请 Let\'s Encrypt 证书」即可获得证书路径。'
}, {
    title: 'https 监听端口',
    dataIndex: 'https_listen_port',
    edit: {
        type: 'number',
        min: 443
    }
}, {
    title: 'SSL 证书路径 (ssl_certificate)',
    dataIndex: 'ssl_certificate',
    edit: {
        type: 'input'
    }
}, {
    title: 'SSL 证书私钥路径 (ssl_certificate_key)',
    dataIndex: 'ssl_certificate_key',
    edit: {
        type: 'input'
    }
}]

export {columns, columnsSSL}
