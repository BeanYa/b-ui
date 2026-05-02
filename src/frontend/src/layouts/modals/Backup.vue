<template>
  <v-dialog class="app-dialog app-dialog--compact" transition="dialog-bottom-transition" width="90%" max-width="540">
    <v-card class="rounded-lg">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('main.backup.title') }}</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto">
            <v-icon icon="mdi-close" @click="control.visible = false" />
          </v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclStats')" value="stats" hide-details></v-checkbox>
          </v-col>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclChanges')" value="changes" hide-details></v-checkbox>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="backup()" hide-details>{{ $t('main.backup.backup') }}</v-btn>
          </v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="restore()" hide-details>{{ $t('main.backup.restore') }}</v-btn>
          </v-col>
        </v-row>
        <v-row>
          <v-divider></v-divider>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="config()" hide-details>{{ $t('main.backup.sbConfig') }}</v-btn>
          </v-col>
        </v-row>

        <v-divider class="my-3"></v-divider>

        <div class="cleanup-section">
          <div class="cleanup-section__head">
            <span class="cleanup-section__title">{{ $t('main.backup.cleanupTitle') }}</span>
            <span class="cleanup-section__caption">{{ $t('main.backup.cleanupCaption') }}</span>
          </div>

          <v-row class="mt-2">
            <v-col cols="auto">
              <v-btn
                variant="tonal"
                color="warning"
                size="small"
                :loading="scanning"
                @click="scanResidual()"
              >
                {{ $t('main.backup.scan') }}
              </v-btn>
            </v-col>
          </v-row>

          <div v-if="residualFiles.length > 0" class="cleanup-section__list mt-2">
            <div
              v-for="file in residualFiles"
              :key="file.path"
              class="cleanup-file"
            >
              <v-checkbox
                v-model="selectedFiles"
                :value="file.path"
                hide-details
                density="compact"
              >
                <template #label>
                  <span class="cleanup-file__name">{{ file.name }}</span>
                  <span class="cleanup-file__meta">{{ kindLabel(file.kind) }} &middot; {{ sizeFormat(file.size) }}</span>
                </template>
              </v-checkbox>
            </div>
          </div>
          <div v-else-if="scanned && residualFiles.length === 0" class="cleanup-section__empty mt-2">
            {{ $t('main.backup.noResidual') }}
          </div>

          <v-row v-if="selectedFiles.length > 0" class="mt-2">
            <v-col cols="auto">
              <v-btn
                color="error"
                size="small"
                variant="tonal"
                :loading="deleting"
                @click="deleteSelected()"
              >
                {{ $t('main.backup.deleteSelected', { n: selectedFiles.length }) }}
              </v-btn>
            </v-col>
          </v-row>
        </div>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import { i18n } from '@/locales'

interface ResidualFile {
  name: string
  path: string
  size: number
  kind: string
}

export default {
  props: ['control', 'visible'],
  data() {
    return {
      exclude: ["stats", "changes"],
      residualFiles: [] as ResidualFile[],
      selectedFiles: [] as string[],
      scanning: false,
      scanned: false,
      deleting: false,
    }
  },
  methods: {
    backup() {
      const excludeOption = this.exclude.length>0 ? '?exclude=' +this.exclude.join(',') : ''
      window.location.href = 'api/getdb' + excludeOption
    },
    config() {
      window.location.href = 'api/singbox-config'
    },
    restore() {
      const fileInput = document.createElement('input')
      fileInput.type = 'file'
      fileInput.accept = '.db'

      fileInput.addEventListener('change', async (event: Event) => {
        const inputElement = event.target as HTMLInputElement
        const dbFile = inputElement.files ? inputElement.files[0] : null

        if (dbFile) {
          const formData = new FormData()
          formData.append('db', dbFile)

          this.control.visible = false

          const uploadMsg = await HttpUtils.post('api/importdb', formData, {
              headers: {
                  'Content-Type': 'multipart/form-data',
              },
          })

          if (uploadMsg.success) {
            await new Promise(resolve => setTimeout(resolve, 1000))
            location.reload()
          }
        }
    })

    fileInput.click()
    },
    async scanResidual() {
      this.scanning = true
      this.scanned = false
      this.selectedFiles = []
      const msg = await HttpUtils.get('api/cleanup')
      this.scanning = false
      this.scanned = true
      if (msg.success && Array.isArray(msg.obj)) {
        this.residualFiles = msg.obj
      }
    },
    async deleteSelected() {
      if (this.selectedFiles.length === 0) return
      this.deleting = true
      const formData = new FormData()
      for (const p of this.selectedFiles) {
        formData.append('paths[]', p)
      }
      const msg = await HttpUtils.post('api/cleanupFiles', formData)
      this.deleting = false
      if (msg.success) {
        this.residualFiles = this.residualFiles.filter(f => !this.selectedFiles.includes(f.path))
        this.selectedFiles = []
      }
    },
    kindLabel(kind: string): string {
      const key = `main.backup.kind_${kind}`
      const translated = this.$t(key)
      return translated !== key ? translated : kind
    },
    sizeFormat(bytes: number): string {
      return HumanReadable.sizeFormat(bytes)
    },
  },
  watch: {
    visible(v) {
      if (v) {
        this.exclude = ["stats", "changes"]
        this.residualFiles = []
        this.selectedFiles = []
        this.scanned = false
      }
    },
  },
}
</script>

<style scoped>
.cleanup-section__head {
  display: grid;
  gap: 4px;
}
.cleanup-section__title {
  font-size: 14px;
  font-weight: 600;
}
.cleanup-section__caption {
  color: var(--app-text-3);
  font-size: 12px;
  line-height: 1.4;
}
.cleanup-section__list {
  background: color-mix(in srgb, var(--app-surface-3) 60%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 12px;
  max-height: 200px;
  overflow-y: auto;
  padding: 4px 8px;
}
.cleanup-file__name {
  font-size: 12px;
  font-weight: 500;
  word-break: break-all;
}
.cleanup-file__meta {
  color: var(--app-text-3);
  display: block;
  font-size: 10px;
}
.cleanup-section__empty {
  color: var(--app-text-3);
  font-size: 12px;
  font-style: italic;
}
</style>
