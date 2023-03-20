<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useGettext} from 'vue3-gettext'
import {useUserStore} from '@/pinia'
import {storeToRefs} from 'pinia'
import {urlJoin} from '@/lib/helper'
import {marked} from 'marked'
import hljs from 'highlight.js'
import 'highlight.js/styles/vs2015.css'
import {SendOutlined} from '@ant-design/icons-vue'
import Template from '@/views/template/Template.vue'
import openai from '@/api/openai'

const {$gettext} = useGettext()

const props = defineProps(['content', 'path', 'history_messages'])

watch(computed(() => props.history_messages), () => {
    messages.value = props.history_messages
})

const {current} = useGettext()

const messages: any = ref([])

const loading = ref(false)
const ask_buffer = ref('')

async function request() {
    loading.value = true
    const t = ref({
        role: 'assistant',
        content: ''
    })
    const user = useUserStore()

    const {token} = storeToRefs(user)

    console.log('fetching...')

    let res = await fetch(urlJoin(window.location.pathname, '/api/chat_gpt'), {
        method: 'POST',
        headers: {'Accept': 'text/event-stream', Authorization: token.value},
        body: JSON.stringify({messages: messages.value})
    })

    messages.value.push(t.value)
    // read body as stream
    console.log('reading...')
    let reader = res.body!.getReader()

    // read stream
    console.log('reading stream...')

    let buffer = ''

    while (true) {
        let {done, value} = await reader.read()
        if (done) {
            console.log('done')
            loading.value = false
            store_record()
            break
        }

        apply(value)
    }

    function apply(input: any) {
        const decoder = new TextDecoder('utf-8')
        const raw = decoder.decode(input)

        const regex = /{"content":"(.+?)"}/g
        const matches = raw.match(regex)

        matches?.forEach(v => {
            const content = JSON.parse(v).content
            for (let c of content) {
                buffer += c
                if (isCodeBlockComplete(buffer)) {
                    t.value.content = buffer
                } else {
                    t.value.content = buffer + '\n```'
                }
            }
        })
    }

    function isCodeBlockComplete(text: string) {
        const codeBlockRegex = /```/g
        const matches = text.match(codeBlockRegex)
        if (matches) {
            return matches.length % 2 === 0
        } else {
            return true
        }
    }

}

async function send() {
    if (messages.value.length === 0) {
        messages.value.push({
            role: 'user',
            content: props.content + '\n\nCurrent Language Code: ' + current
        })
    } else {
        messages.value.push({
            role: 'user',
            content: ask_buffer.value
        })
        ask_buffer.value = ''
    }
    await request()
}

const renderer = new marked.Renderer()
renderer.code = (code, lang: string) => {
    const language = hljs.getLanguage(lang) ? lang : 'nginx'
    const highlightedCode = hljs.highlight(code, {language}).value
    return `<pre><code class="hljs ${language}">${highlightedCode}</code></pre>`
}

marked.setOptions({
    renderer: renderer,
    langPrefix: 'hljs language-', // highlight.js css expects a top-level 'hljs' class.
    pedantic: false,
    gfm: true,
    breaks: false,
    sanitize: false,
    smartypants: true,
    xhtml: false
})

function store_record() {
    openai.store_record({
        file_name: props.path,
        messages: messages.value
    })
}

function clear_record() {
    openai.store_record({
        file_name: props.path,
        messages: []
    })
    messages.value = []
}

async function regenerate(index: number) {
    editing_idx.value = -1
    messages.value = messages.value.slice(0, index)
    await request()
}

const editing_idx = ref(-1)
</script>

<template>
    <a-card title="ChatGPT">
        <div class="chatgpt-container">
            <template v-if="messages?.length>0">
                <a-list
                    class="chatgpt-log"
                    item-layout="horizontal"
                    :data-source="messages"
                >
                    <template #renderItem="{ item, index }">
                        <a-list-item>
                            <a-comment :author="item.role" :avatar="item.avatar">
                                <template #content>
                                    <div class="content" v-if="item.role==='assistant'||editing_idx!=index"
                                         v-html="marked.parse(item.content)"></div>
                                    <a-input style="padding: 0" v-else v-model:value="item.content"
                                             :bordered="false"/>
                                </template>
                                <template #actions>
                                    <span v-if="item.role==='user'&&editing_idx!==index" @click="editing_idx=index">
                                        {{ $gettext('Modify') }}
                                    </span>
                                    <template v-else-if="editing_idx==index">
                                        <span @click="regenerate(index+1)">{{ $gettext('Save') }}</span>
                                        <span @click="editing_idx=-1">{{ $gettext('Cancel') }}</span>
                                    </template>
                                    <span v-else-if="!loading" @click="regenerate(index)" :disabled="loading">
                                        {{ $gettext('Reload') }}
                                    </span>
                                </template>
                            </a-comment>
                        </a-list-item>
                    </template>
                </a-list>
                <div class="input-msg">
                    <div class="control-btn">
                        <a-space v-show="!loading">
                            <a-popconfirm
                                :cancelText="$gettext('No')"
                                :okText="$gettext('OK')"
                                :title="$gettext('Are you sure you want to clear the record of chat?')"
                                @confirm="clear_record">
                                <a-button type="text">{{ $gettext('Clear') }}</a-button>
                            </a-popconfirm>
                            <a-button type="text" @click="regenerate(messages?.length-1)">
                                {{ $gettext('Regenerate response') }}
                            </a-button>
                        </a-space>
                    </div>
                    <a-textarea auto-size v-model:value="ask_buffer"/>
                    <div class="sned-btn">
                        <a-button size="small" type="text" :loading="loading" @click="send">
                            <send-outlined/>
                        </a-button>
                    </div>
                </div>
            </template>
            <template v-else>
                <a-button @click="send">{{ $gettext('Chat with ChatGPT') }}</a-button>
            </template>
        </div>
    </a-card>
</template>

<style lang="less" scoped>
.chatgpt-container {
    margin: 0 auto;
    max-width: 800px;

    .chatgpt-log {
        .content {
            width: 100%;

            :deep(.hljs) {
                border-radius: 5px;
            }
        }

        :deep(.ant-comment-content) {
            width: 100%;
        }

        :deep(.ant-comment) {
            width: 100%;
        }

        :deep(.ant-comment-content-detail) {
            width: 100%;

            p {
                margin-bottom: 10px;
            }
        }

        :deep(.ant-list-item:first-child) {
            display: none;
        }
    }

    .input-msg {
        position: relative;

        .control-btn {
            display: flex;
            justify-content: center;
        }

        .sned-btn {
            position: absolute;
            right: 0;
            bottom: 3px;
        }
    }
}
</style>
