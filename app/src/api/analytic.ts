import { http } from '@uozi-admin/request'

export interface CPUInfoStat {
  cpu: number
  vendorId: string
  family: string
  model: string
  stepping: number
  physicalId: string
  coreId: string
  cores: number
  modelName: string
  mhz: number
  cacheSize: number
  flags: string[]
  microcode: string
}

export interface IOCountersStat {
  name: string
  bytesSent: number
  bytesRecv: number
  packetsSent: number
  packetsRecv: number
  errin: number
  errout: number
  dropin: number
  dropout: number
  fifoin: number
  fifoout: number
}

export interface HostInfoStat {
  hostname: string
  uptime: number
  bootTime: number
  procs: number
  os: string
  platform: string
  platformFamily: string
  platformVersion: string
  kernelVersion: string
  kernelArch: string
  virtualizationSystem: string
  virtualizationRole: string
  hostId: string
}

export interface MemStat {
  total: string
  used: string
  cached: string
  free: string
  swap_used: string
  swap_total: string
  swap_cached: string
  swap_percent: number
  pressure: number
}

export interface PartitionStat {
  mountpoint: string
  device: string
  fstype: string
  total: string
  used: string
  free: string
  percentage: number
}

export interface DiskStat {
  total: string
  used: string
  percentage: number
  writes: Usage
  reads: Usage
  partitions: PartitionStat[]
}

export interface LoadStat {
  load1: number
  load5: number
  load15: number
}

export interface Usage {
  x: string
  y: number
}

export interface CPURecords {
  info: CPUInfoStat[]
  user: Usage[]
  total: Usage[]
}

export interface NetworkRecords {
  init: IOCountersStat
  bytesRecv: Usage[]
  bytesSent: Usage[]
}

export interface DiskIORecords {
  writes: Usage[]
  reads: Usage[]
}

export interface AnalyticInit {
  host: HostInfoStat
  cpu: CPURecords
  network: NetworkRecords
  disk_io: DiskIORecords
  disk: DiskStat
  memory: MemStat
  loadavg: LoadStat
}

const analytic = {
  init(): Promise<AnalyticInit> {
    return http.get('/analytic/init')
  },
  serverWebSocketUrl: '/api/analytic',
  nodesWebSocketUrl: '/api/analytic/nodes',
}

export default analytic
