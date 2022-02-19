<template>
    <div>
        <template v-for="(item, index) in items">
            <div class="divider" v-if="item.type === 'divider'" :key="index"/>
            <menu-item v-else :key="index" v-bind="item"/>
        </template>
    </div>
</template>

<script>
import MenuItem from './MenuItem.vue'

export default {
    components: {
        MenuItem,
    },

    props: {
        editor: {
            type: Object,
            required: true,
        },
    },

    data() {
        return {
            items: [
                {
                    icon: 'bold',
                    title: '加粗',
                    action: () => this.editor.chain().focus().toggleBold().run(),
                    isActive: () => this.editor.isActive('bold'),
                },
                {
                    icon: 'italic',
                    title: '斜体',
                    action: () => this.editor.chain().focus().toggleItalic().run(),
                    isActive: () => this.editor.isActive('italic'),
                },
                {
                    icon: 'strikethrough',
                    title: '删除线',
                    action: () => this.editor.chain().focus().toggleStrike().run(),
                    isActive: () => this.editor.isActive('strike'),
                },
                {
                    icon: 'code-view',
                    title: '行内代码',
                    action: () => this.editor.chain().focus().toggleCode().run(),
                    isActive: () => this.editor.isActive('code'),
                },
                {
                    icon: 'mark-pen-line',
                    title: '高亮',
                    action: () => this.editor.chain().focus().toggleHighlight().run(),
                    isActive: () => this.editor.isActive('highlight'),
                },
                {
                    type: 'divider',
                },
                {
                    icon: 'h-1',
                    title: '一级标题',
                    action: () => this.editor.chain().focus().toggleHeading({level: 1}).run(),
                    isActive: () => this.editor.isActive('heading', {level: 1}),
                },
                {
                    icon: 'h-2',
                    title: '二级标题',
                    action: () => this.editor.chain().focus().toggleHeading({level: 2}).run(),
                    isActive: () => this.editor.isActive('heading', {level: 2}),
                },
                {
                    icon: 'paragraph',
                    title: '段落',
                    action: () => this.editor.chain().focus().setParagraph().run(),
                    isActive: () => this.editor.isActive('paragraph'),
                },
                {
                    icon: 'list-unordered',
                    title: '无序列表',
                    action: () => this.editor.chain().focus().toggleBulletList().run(),
                    isActive: () => this.editor.isActive('bulletList'),
                },
                {
                    icon: 'list-ordered',
                    title: '有序列表',
                    action: () => this.editor.chain().focus().toggleOrderedList().run(),
                    isActive: () => this.editor.isActive('orderedList'),
                },
                {
                    icon: 'list-check-2',
                    title: '任务列表',
                    action: () => this.editor.chain().focus().toggleTaskList().run(),
                    isActive: () => this.editor.isActive('taskList'),
                },
                {
                    icon: 'code-box-line',
                    title: '代码块',
                    action: () => this.editor.chain().focus().toggleCodeBlock().run(),
                    isActive: () => this.editor.isActive('codeBlock'),
                },
                {
                    type: 'divider',
                },
                {
                    icon: 'double-quotes-l',
                    title: '引用',
                    action: () => this.editor.chain().focus().toggleBlockquote().run(),
                    isActive: () => this.editor.isActive('blockquote'),
                },
                {
                    icon: 'separator',
                    title: '水平分割线',
                    action: () => this.editor.chain().focus().setHorizontalRule().run(),
                },
                {
                    type: 'divider',
                },
                {
                    icon: 'text-wrap',
                    title: '换行',
                    action: () => this.editor.chain().focus().setHardBreak().run(),
                },
                {
                    icon: 'format-clear',
                    title: '清除格式',
                    action: () => this.editor.chain()
                        .focus()
                        .clearNodes()
                        .unsetAllMarks()
                        .run(),
                },
                {
                    type: 'divider',
                },
                {
                    icon: 'arrow-go-back-line',
                    title: '撤回',
                    action: () => this.editor.chain().focus().undo().run(),
                },
                {
                    icon: 'arrow-go-forward-line',
                    title: '重做',
                    action: () => this.editor.chain().focus().redo().run(),
                },
            ],
        }
    },
}
</script>

<style lang="less" scoped>
.divider {
    width: 2px;
    height: 1.25rem;
    background-color: rgba(#999999, 0.1);
    margin-left: 0.5rem;
    margin-right: 0.75rem;
    display: inline-block;
}
</style>
