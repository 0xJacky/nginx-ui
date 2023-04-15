<template>
    <div class="ant-pro-footer-toolbar">
        <div style="float: left">
            <slot name="extra">{{ extra }}</slot>
        </div>
        <div style="float: right">
            <slot></slot>
        </div>
    </div>
</template>

<script>
export default {
    name: 'FooterToolBar',
    props: {
        prefixCls: {
            type: String,
            default: 'ant-pro-footer-toolbar'
        },
        extra: {
            type: [String, Object],
            default: ''
        }
    }
}
</script>

<style lang="less" scoped>
.dark {
    .ant-pro-footer-toolbar {
        background: rgba(24, 24, 24, 1);
        border-top: unset;
    }
}

.ant-pro-footer-toolbar {
    position: fixed;
    width: 100%;
    bottom: 0;
    right: 0;
    height: 56px;
    line-height: 56px;
    box-shadow: 0 -1px 2px rgba(0, 0, 0, 0.03);
    background: #ffffff8c;
    border-top: 1px solid #e8e8e8;
    padding: 0 24px;
    z-index: 9;

    &:after {
        content: "";
        display: block;
        clear: both;
    }
}
</style>
