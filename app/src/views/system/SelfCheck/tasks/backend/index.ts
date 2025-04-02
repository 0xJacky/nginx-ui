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
}

export default backendTasks
