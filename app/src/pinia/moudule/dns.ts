import type { Pagination } from '@/api/curd'
import type { DDNSConfig, DDNSDomainItem, DNSDomain, DNSRecord, DomainListParams, RecordListParams, RecordPayload, UpdateDDNSPayload } from '@/api/dns'
import { defineStore } from 'pinia'
import { dnsApi } from '@/api/dns'

interface DnsState {
  domains: DNSDomain[]
  pagination?: Pagination
  isLoading: boolean
  currentDomain?: DNSDomain
  records: DNSRecord[]
  recordsLoading: boolean
  recordsPagination?: Pagination
  ddnsConfig?: DDNSConfig
  ddnsLoading: boolean
  ddnsList: DDNSDomainItem[]
  ddnsListLoading: boolean
}

export const useDnsStore = defineStore('dns-store', {
  state: (): DnsState => ({
    domains: [],
    pagination: undefined,
    isLoading: false,
    currentDomain: undefined,
    records: [],
    recordsLoading: false,
    recordsPagination: undefined,
    ddnsConfig: undefined,
    ddnsLoading: false,
    ddnsList: [],
    ddnsListLoading: false,
  }),
  actions: {
    async fetchDomains(params?: DomainListParams) {
      this.isLoading = true
      try {
        const response = await dnsApi.getList(params)
        this.domains = response.data
        this.pagination = response.pagination
      }
      finally {
        this.isLoading = false
      }
    },
    async fetchDomainDetail(id: number) {
      const data = await dnsApi.getItem(id)
      this.currentDomain = data
      return data
    },
    async fetchRecords(domainId: number, params?: RecordListParams) {
      this.recordsLoading = true
      try {
        const { data, pagination } = await dnsApi.listRecords(domainId, params)
        this.records = data
        this.recordsPagination = pagination
      }
      finally {
        this.recordsLoading = false
      }
    },
    async createRecord(domainId: number, payload: RecordPayload) {
      const data = await dnsApi.createRecord(domainId, payload)
      this.records.unshift(data)
      return data
    },
    async updateRecord(domainId: number, recordId: string, payload: RecordPayload) {
      const data = await dnsApi.updateRecord(domainId, recordId, payload)
      const index = this.records.findIndex(item => item.id === recordId)
      if (index !== -1)
        this.records.splice(index, 1, data)
      return data
    },
    async deleteRecord(domainId: number, recordId: string) {
      await dnsApi.deleteRecord(domainId, recordId)
      this.records = this.records.filter(item => item.id !== recordId)
    },
    async fetchDDNSConfig(domainId: number) {
      this.ddnsLoading = true
      try {
        this.ddnsConfig = await dnsApi.getDDNSConfig(domainId)
        return this.ddnsConfig
      }
      finally {
        this.ddnsLoading = false
      }
    },
    async updateDDNSConfig(domainId: number, payload: UpdateDDNSPayload) {
      this.ddnsLoading = true
      try {
        this.ddnsConfig = await dnsApi.updateDDNSConfig(domainId, payload)
        return this.ddnsConfig
      }
      finally {
        this.ddnsLoading = false
      }
    },
    resetRecords() {
      this.records = []
      this.recordsPagination = undefined
    },
    resetDDNS() {
      this.ddnsConfig = undefined
    },
    async fetchDDNSList() {
      this.ddnsListLoading = true
      try {
        const res = await dnsApi.listDDNS()
        this.ddnsList = res.data
        return res.data
      }
      finally {
        this.ddnsListLoading = false
      }
    },
    async refreshDDNSItem(domainId: number) {
      const cfg = await this.fetchDDNSConfig(domainId)
      const itemIndex = this.ddnsList.findIndex(item => item.id === domainId)
      if (itemIndex !== -1 && cfg) {
        this.ddnsList.splice(itemIndex, 1, {
          ...this.ddnsList[itemIndex],
          config: cfg,
        })
      }
    },
  },
})
