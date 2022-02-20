import $gettext from "@/lib/translate/gettext";

const columns = [{
    title: $gettext('Configuration Name'),
    dataIndex: 'name',
    edit: {
        type: 'input'
    }
}, {
    title: $gettext('Server Names (server_name)'),
    dataIndex: 'server_name',
    edit: {
        type: 'input'
    }
}, {
    title: $gettext('Root Directory (root)'),
    dataIndex: 'root',
    edit: {
        type: 'input'
    }
}, {
    title: $gettext('Index (index)'),
    dataIndex: 'index',
    edit: {
        type: 'input'
    }
}, {
    title: $gettext('HTTP Listen Port'),
    dataIndex: 'http_listen_port',
    edit: {
        type: 'number',
        min: 80
    }
}, {
    title: $gettext('Enable TLS'),
    dataIndex: 'support_ssl',
    edit: {
        type: 'switch',
        event: 'change_support_ssl'
    }
}]

const columnsSSL = [{
    title: $gettext('Certificate Auto-renewal'),
    dataIndex: 'auto_cert',
    edit: {
        type: 'switch',
        event: 'change_auto_cert'
    },
    description: $gettext('The certificate for the domain will be checked every hour, ' +
        'and will be renewed if it has been more than 1 month since it was last issued.' +
        '<br/>If you do not have a certificate before, please click "Getting Certificate from Let\'s Encrypt" first.')
}, {
    title: $gettext('HTTPS Listen Port'),
    dataIndex: 'https_listen_port',
    edit: {
        type: 'number',
        min: 443
    }
}, {
    title: $gettext('Certificate Path (ssl_certificate)'),
    dataIndex: 'ssl_certificate',
    edit: {
        type: 'input'
    }
}, {
    title: $gettext('Private Key Path (ssl_certificate_key)'),
    dataIndex: 'ssl_certificate_key',
    edit: {
        type: 'input'
    }
}]

export {columns, columnsSSL}
