import http from '@/lib/http'

function reloadNginx(nodeIds: number[]) {
  return http.post('/environments/reload_nginx', { node_ids: nodeIds })
}

function restartNginx(nodeIds: number[]) {
  return http.post('/environments/restart_nginx', { node_ids: nodeIds })
}

export default {
  reloadNginx,
  restartNginx,
}
