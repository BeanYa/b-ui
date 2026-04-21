import HttpUtils from '@/plugins/httputil'
import { defineStore } from 'pinia'

const Auth = defineStore('Auth', {
  state: () => ({
    username: '',
    isAdmin: false,
    loaded: false,
  }),
  actions: {
    reset() {
      this.username = ''
      this.isAdmin = false
      this.loaded = false
    },
    async loadAuthState() {
      const msg = await HttpUtils.get('api/authState')
      if (!msg.success) {
        return
      }

      this.username = msg.obj?.username ?? ''
      this.isAdmin = !!msg.obj?.isAdmin
      this.loaded = true
    },
  },
})

export default Auth
