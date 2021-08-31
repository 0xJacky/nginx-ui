<template>
    <div class="editor" v-if="editor">
        <menu-bar class="editor__header" :editor="editor"/>
        <editor-content :editor="editor"/>
    </div>
</template>

<script>
import {Editor, EditorContent, VueNodeViewRenderer} from '@tiptap/vue-2'
import StarterKit from '@tiptap/starter-kit'
import Document from '@tiptap/extension-document'
import Paragraph from '@tiptap/extension-paragraph'
import Highlight from '@tiptap/extension-highlight'
import Text from '@tiptap/extension-text'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import CharacterCount from '@tiptap/extension-character-count'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import CodeBlockComponent from './CodeBlockComponent'
import MenuBar from './MenuBar.vue'

import lowlight from 'lowlight'

export default {
    components: {
        EditorContent,
        MenuBar,
    },

    data() {
        return {
            editor: null,
        }
    },

    props: {
        value: {
            type: String,
            default: '',
        },
    },
    model: {
        prop: 'value',
        event: 'changeValue'
    },
    watch: {
        value(value) {
            // HTML
            const isSame = this.editor.getHTML() === value

            // JSON
            // const isSame = this.editor.getJSON().toString() === value.toString()

            if (isSame) {
                return
            }
            this.editor.commands.setContent(this.value, false)
        },
    },

    created() {
        const that = this
        this.editor = new Editor({
            onUpdate({editor}) {
                that.$emit('changeValue', editor.getHTML())
            },
            content: '',
            extensions: [
                StarterKit,
                Document,
                Paragraph,
                Text,
                TaskList,
                TaskItem,
                CharacterCount,
                Highlight,
                CodeBlockLowlight
                    .extend({
                        addNodeView() {
                            return VueNodeViewRenderer(CodeBlockComponent)
                        },
                    }).configure({lowlight}),
            ],
        })
    },

    mounted() {
        this.editor.commands.setContent(this.value, false)
    },

    beforeDestroy() {
        this.editor.destroy()
    },
}
</script>

<style lang="less">
.ant-affix {
    z-index: 8 !important;
}
</style>

<style lang="less" scoped>
.editor {
    display: flex;
    flex-direction: column;
    border-radius: 0.75rem;
    @gray: rgba(0, 0, 0, 0.2);
    background-color: #FFFFFF;
    @media (prefers-color-scheme: dark) {
        @gray: #666666;
        border: 1px solid @gray;
        background-color: #28292c;
        &__header {
            border-bottom: 1px solid @gray;
        }
    }
    border: 1px solid @gray;
    line-height: 1.5!important;

    &__header {
        display: flex;
        align-items: center;
        flex: 0 0 auto;
        flex-wrap: wrap;
        padding: 0.25rem;
        border-bottom: 1px solid @gray;
    }

    &__content {
        padding: 1.25rem 1rem;
        flex: 1 1 auto;
        overflow-x: hidden;
        overflow-y: auto;
        -webkit-overflow-scrolling: touch;
    }
}
</style>

<style lang="less">
@import "style";
</style>

<style lang="less">
.editor .ProseMirror {
    height: 500px;
    overflow: scroll;
    padding: 15px;
}
</style>

