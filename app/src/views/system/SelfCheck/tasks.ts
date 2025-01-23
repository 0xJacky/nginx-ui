const tasks = {
  'Directory-Sites': {
    name: () => $gettext('Sites Directory'),
    description: () => $gettext('Check if the sites-available and sites-enabled directories are under the nginx configuration directory.'),
  },
  'Directory-Streams': {
    name: () => $gettext('Streams Directory'),
    description: () => $gettext('Check if the streams-available and streams-enabled directories are under the nginx configuration directory.'),
  },
  'NginxConf-Sites-Enabled': {
    name: () => $gettext('Nginx Conf Include Sites Enabled'),
    description: () => $gettext('Check if the nginx.conf includes the sites-enabled directory.'),
  },
  'NginxConf-Streams-Enabled': {
    name: () => $gettext('Nginx Conf Include Streams Enabled'),
    description: () => $gettext('Check if the nginx.conf includes the streams-enabled directory.'),
  },
}

export default tasks
