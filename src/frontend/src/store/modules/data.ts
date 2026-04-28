import HttpUtils from '@/plugins/httputil'
import { defineStore } from 'pinia'
import { push } from 'notivue'
import { i18n } from '@/locales'
import { Inbound } from '@/types/inbounds'
import { Client } from '@/types/clients'
import {
  remotePanelCheckOutbound,
  remotePanelKeypairs,
  remotePanelLinkConvert,
  remotePanelLoad,
  remotePanelPartial,
  remotePanelSave,
  remotePanelStats,
} from '@/features/remotePanelApi'
import {
  createDefaultConfig,
  createEmptyOnlines,
  normalizeCollection,
  normalizeConfig,
  normalizeOnlines,
} from '@/features/data/normalize'
import { parseReloadItems } from '@/features/dashboard/persistence'

const Data = defineStore('Data', {
  state: () => ({ 
    lastLoad: 0,
    remoteNodeId: '',
    remoteBaseUrl: '',
    remoteHostname: '',
    reloadItems: parseReloadItems(localStorage.getItem('reloadItems')),
    subURI: "",
    enableTraffic: false,
    onlines: createEmptyOnlines(),
    config: createDefaultConfig(),
    inbounds: <any[]>[],
    outbounds: <any[]>[],
    services: <any[]>[],
    endpoints: <any[]>[],
    clients: <any>[],
    tlsConfigs: <any[]>[],
  }),
  actions: {
    enterRemoteNode(nodeId: string, baseUrl: string) {
      this.remoteNodeId = nodeId
      this.remoteBaseUrl = baseUrl
      this.remoteHostname = panelHostname(baseUrl)
      this.lastLoad = 0
      this.resetPanelData()
    },
    exitRemoteNode() {
      this.remoteNodeId = ''
      this.remoteBaseUrl = ''
      this.remoteHostname = ''
      this.lastLoad = 0
      this.resetPanelData()
    },
    isRemote(): boolean {
      return this.remoteNodeId.length > 0
    },
    resetPanelData() {
      this.subURI = ''
      this.enableTraffic = false
      this.onlines = createEmptyOnlines()
      this.config = createDefaultConfig()
      this.clients = []
      this.inbounds = []
      this.outbounds = []
      this.endpoints = []
      this.services = []
      this.tlsConfigs = []
    },
    async loadData() {
      if (this.isRemote()) {
        const payload = {
          ...(this.lastLoad > 0 ? { lu: String(this.lastLoad) } : {}),
          hostname: this.remoteHostname,
        }
        const data = await remotePanelLoad(this.remoteNodeId, payload)
        this.setNewData(data ?? {})
        return
      }
      const msg = await HttpUtils.get('api/load', this.lastLoad >0 ? {lu: this.lastLoad} : {} )
      if(msg.success) {
        this.lastLoad = Math.floor((new Date()).getTime()/1000)
        this.onlines = normalizeOnlines(msg.obj?.onlines)
        if (msg.obj?.lastLog) {
          push.error({
            title: i18n.global.t('error.core'),
            duration: 5000,
            message: msg.obj.lastLog
          })
        }
        
        if (msg.obj?.config) {
          this.setNewData(msg.obj)
        }
      }
    },
    setNewData(data: any) {
      this.lastLoad = Math.floor((new Date()).getTime()/1000)
      if (Object.hasOwn(data, 'subURI')) this.subURI = data.subURI ?? ''
      if (Object.hasOwn(data, 'enableTraffic')) this.enableTraffic = !!data.enableTraffic
      if (Object.hasOwn(data, 'onlines')) this.onlines = normalizeOnlines(data.onlines)
      if (Object.hasOwn(data, 'config')) this.config = normalizeConfig(data.config)
      if (Object.hasOwn(data, 'clients')) this.clients = normalizeCollection(data.clients)
      if (Object.hasOwn(data, 'inbounds')) this.inbounds = normalizeCollection(data.inbounds)
      if (Object.hasOwn(data, 'outbounds')) this.outbounds = normalizeCollection(data.outbounds)
      if (Object.hasOwn(data, 'services')) this.services = normalizeCollection(data.services)
      if (Object.hasOwn(data, 'endpoints')) this.endpoints = normalizeCollection(data.endpoints)
      if (Object.hasOwn(data, 'tls')) this.tlsConfigs = normalizeCollection(data.tls)
    },
    async loadInbounds(ids: number[]): Promise<Inbound[]> {
      if (this.isRemote()) {
        const data = await remotePanelPartial(this.remoteNodeId, {
          object: 'inbounds',
          ...(ids.length > 0 ? { id: ids.join(',') } : {}),
          hostname: this.remoteHostname,
        })
        return data?.inbounds ?? []
      }
      const options = ids.length > 0 ? {id: ids.join(",")} : {}
      const msg = await HttpUtils.get('api/inbounds', options)
      if(msg.success) {
        return msg.obj.inbounds
      }
      return <Inbound[]>[]
    },
    async loadClients(id: number): Promise<Client> {
      if (this.isRemote()) {
        const data = await remotePanelPartial(this.remoteNodeId, {
          object: 'clients',
          ...(id > 0 ? { id: String(id) } : {}),
          hostname: this.remoteHostname,
        })
        return <Client>data?.clients?.[0] ?? <Client>{}
      }
      const options = id > 0 ? {id: id} : {}
      const msg = await HttpUtils.get('api/clients', options)
      if(msg.success) {
        return <Client>msg.obj.clients[0]??{}
      }
      return <Client>{}
    },
    async save (object: string, action: string, data: any, initUsers?: number[]): Promise<boolean> {
      if (this.isRemote()) {
        try {
          const newData = await remotePanelSave(this.remoteNodeId, {
            object,
            action,
            data,
            initUsers,
            hostname: this.remoteHostname,
          })
          const objectName = ['tls', 'config'].includes(object) ? object : object.substring(0, object.length - 1)
          push.success({
            title: i18n.global.t('success'),
            duration: 5000,
            message: i18n.global.t('actions.' + action) + " " + i18n.global.t('objects.' + objectName)
          })
          this.setNewData(newData ?? {})
          return true
        } catch (error: any) {
          push.error({
            title: i18n.global.t('failed'),
            message: error?.message ?? String(error),
          })
          return false
        }
      }
      let postData = {
        object: object,
        action: action,
        data: JSON.stringify(data, null, 2),
        initUsers: initUsers?.join(',') ?? undefined
      }
      const msg = await HttpUtils.post('api/save', postData)
      if (msg.success) {
        const objectName = ['tls', 'config'].includes(object) ? object : object.substring(0, object.length - 1)
        push.success({
          title: i18n.global.t('success'),
          duration: 5000,
          message: i18n.global.t('actions.' + action) + " " + i18n.global.t('objects.' + objectName)
        })
        this.setNewData(msg.obj)
      }
      return msg.success
    },
    async keypairs(kind: string, options = ''): Promise<string[]> {
      if (this.isRemote()) {
        return remotePanelKeypairs(this.remoteNodeId, {
          k: kind,
          ...(options.length > 0 ? { o: options } : {}),
        })
      }
      const msg = await HttpUtils.get('api/keypairs', options.length > 0 ? { k: kind, o: options } : { k: kind })
      return msg.success ? msg.obj : []
    },
    async linkConvert(link: string): Promise<any> {
      if (this.isRemote()) {
        return remotePanelLinkConvert(this.remoteNodeId, { link })
      }
      const msg = await HttpUtils.post('api/linkConvert', { link })
      return msg.success ? msg.obj : null
    },
    async checkOutbound(tag: string, link = ''): Promise<any> {
      if (this.isRemote()) {
        return remotePanelCheckOutbound(this.remoteNodeId, {
          tag,
          ...(link.length > 0 ? { link } : {}),
        })
      }
      const msg = await HttpUtils.get('api/checkOutbound', link.length > 0 ? { tag, link } : { tag })
      return msg.success ? msg.obj : { OK: false, Error: msg.msg }
    },
    async stats(resource: string, tag: string, limit: number): Promise<any[]> {
      if (this.isRemote()) {
        return remotePanelStats(this.remoteNodeId, { resource, tag, limit })
      }
      const msg = await HttpUtils.get('api/stats', { resource, tag, limit })
      return msg.success ? (msg.obj ?? []) : []
    },
    // Check duplicate client name
    checkClientName (id: number, newName: string): boolean {
      const oldName = id > 0 ? this.clients.findLast((i: any) => i.id == id)?.name : null
      if (newName != oldName && this.clients.findIndex((c: any) => c.name == newName) != -1) {
        push.error({
          message: i18n.global.t('error.dplData') + ": " + i18n.global.t('client.name')
        })
        return true
      }
      return false
    },
    // Check bulk client names
    checkBulkClientNames (names: string[]): boolean {
      const newNames = new Set(names)
      const oldNames = new Set(this.clients.map((c: any) => c.name))
      const allNames = new Set([...oldNames, ...newNames])
      if (newNames.size != names.length || oldNames.size + newNames.size != allNames.size) {
        push.error({
          message: i18n.global.t('error.dplData') + ": " + i18n.global.t('client.name')
        })
        return true
      }
      return false
    },
    // check duplicate tag
    checkTag (object: string, id: number, tag: string): boolean {
      let objects = <any[]>[]
      switch (object) {
        case 'inbound':
          objects = this.inbounds
          break
        case 'outbound':
          objects = this.outbounds
          break
        case 'service':
          objects = this.services
          break
        case 'endpoint':
          objects = this.endpoints
          break
        default:
          return false
      }
      const oldObject = id > 0 ? objects.findLast((i: any) => i.id == id) : null
      if (tag != oldObject?.tag && objects.findIndex((i: any) => i.tag == tag) != -1) {
        push.error({
          message: i18n.global.t('error.dplData') + ": " + i18n.global.t('objects.tag')
        })
        return true
      }
      return false
    },
  }
})

function panelHostname(baseUrl: string): string {
  try {
    return new URL(baseUrl).hostname
  } catch {
    return baseUrl
      .replace(/^https?:\/\//i, '')
      .replace(/\/.*$/, '')
      .replace(/:\d+$/, '')
  }
}

export default Data
