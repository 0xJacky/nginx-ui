<script setup lang="ts">
import { SendOutlined } from '@ant-design/icons-vue'
import { storeToRefs } from 'pinia'
import { useChatGPTStore } from './chatgpt'

const chatGPTStore = useChatGPTStore()
const { loading, askBuffer, messages } = storeToRefs(chatGPTStore)

const messagesLength = computed(() => messages.value?.length ?? 0)
</script>

<template>
  <div class="input-msg">
    <div class="control-btn">
      <ASpace v-show="!loading">
        <APopconfirm
          :cancel-text="$gettext('No')"
          :ok-text="$gettext('OK')"
          :title="$gettext('Are you sure you want to clear the record of chat?')"
          @confirm="chatGPTStore.clearRecord()"
        >
          <AButton type="text">
            {{ $gettext('Clear') }}
          </AButton>
        </APopconfirm>
        <AButton
          type="text"
          @click="chatGPTStore.regenerate(messagesLength - 1)"
        >
          {{ $gettext('Regenerate response') }}
        </AButton>
      </ASpace>
    </div>
    <ATextarea
      v-model:value="askBuffer"
      auto-size
      @press-enter="chatGPTStore.send(askBuffer)"
    />
    <div class="send-btn">
      <AButton
        size="small"
        type="text"
        :loading="loading"
        @click="chatGPTStore.send(askBuffer)"
      >
        <SendOutlined />
      </AButton>
    </div>
  </div>
</template>

<style lang="less" scoped>
.input-msg {
  position: sticky;
  bottom: 0;
  left: 0;
  right: 0;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  padding: 16px;
  border-radius: 0 0 8px 8px;

  .control-btn {
    display: flex;
    justify-content: center;
  }

  .send-btn {
    position: absolute;
    right: 16px;
    bottom: 19px;
  }
}

.dark {
  .input-msg {
    background: rgba(30, 30, 30, 0.8);
  }
}
</style>
