<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ title }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text class="app-dialog__body">
        <div class="app-code-editor">
          <div class="app-code-lines">
            <span v-for="n in lineCount" :key="n">{{ n }}</span>
          </div>
          <v-textarea
            ref="textareaRef"
            v-model="content"
            @scroll="syncScroll"
            hide-details
            variant="outlined"
            class="app-code-textarea"
            no-resize
            auto-grow
          ></v-textarea>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="primary"
          variant="outlined"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn
          color="primary"
          variant="tonal"
          @click="saveChanges"
        >
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
export default {
  props: ['visible', 'data', 'title'],
  emits: ['close', 'save'],
  data() {
    return {
      content: this.$props.data,
    }
  },
  computed: {
    lineCount() {
      return this.content?.split('\n').length
    }
  },
  methods: {
    syncScroll() {
      const textarea = document.querySelector('textarea')
      const lineNumbers = textarea?.parentElement?.parentElement?.querySelector('.app-code-lines')
      if (lineNumbers && textarea) {
        lineNumbers.scrollTop = textarea.scrollTop
      }
    },
    closeModal() {
      this.$emit('close')
    },
    saveChanges() {
      this.$emit('save', this.content)
    }
  },
  watch: {
    visible(v) {
      if (v) {
        this.content = this.$props.data
      }
    }
  }
}
</script>

<style scoped>
.app-code-lines span {
  display: block;
  font-family: var(--app-font-mono);
  height: 1.5em;
  line-height: 1.5;
}

:deep(.app-code-textarea .v-field) {
  background: transparent !important;
  border-radius: 0 !important;
  box-shadow: none !important;
}

:deep(.app-code-textarea .v-field__input) {
  font-family: var(--app-font-mono) !important;
  font-size: 13px !important;
  line-height: 1.5 !important;
  mask-image: inherit;
  padding: 12px 10px !important;
  white-space: pre;
}

:deep(.app-code-textarea textarea) {
  margin-top: 0 !important;
  padding-top: 0 !important;
}
</style>
