import type { BackendTask } from '../types'

const backendTasks: Record<string, BackendTask> = {
  'Directory-Sites': {
    name: () => $gettext('Sites Directory'),
    description: () => $gettext('Check if the sites-available and sites-enabled directories are under the nginx configuration directory.'),
    type: 'backend',
  },
  'Directory-Streams': {
    name: () => $gettext('Streams Directory'),
    description: () => $gettext('Check if the streams-available and streams-enabled directories are under the nginx configuration directory.'),
    type: 'backend',
  },
  'NginxConf-Sites-Enabled': {
    name: () => $gettext('Nginx Conf Include Sites Enabled'),
    description: () => $gettext('Check if the nginx.conf includes the sites-enabled directory.'),
    type: 'backend',
  },
  'NginxConf-Streams-Enabled': {
    name: () => $gettext('Nginx Conf Include Streams Enabled'),
    description: () => $gettext('Check if the nginx.conf includes the streams-enabled directory.'),
    type: 'backend',
  },
  'NginxConf-ConfD': {
    name: () => $gettext('Nginx Conf Include Conf.d'),
    description: () => $gettext('Check if the nginx.conf includes the conf.d directory.'),
    type: 'backend',
  },
  'Docker-Socket': {
    name: () => $gettext('Docker Socket'),
    description: () => $gettext('Check if /var/run/docker.sock exists. '
      + 'If you are using Nginx UI Official Docker Image, '
      + 'please make sure the docker socket is mounted like this: '
      + '`-v /var/run/docker.sock:/var/run/docker.sock`.'),
    type: 'backend',
  },
}

export default backendTasks
