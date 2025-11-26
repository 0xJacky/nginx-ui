import type { Pagination } from '@/api/curd'
import type { DNSDomain, DNSRecord, DomainListParams, RecordListParams, RecordPayload } from '@/api/dns'
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
    resetRecords() {
      this.records = []
      this.recordsPagination = undefined
    },
  },
})
