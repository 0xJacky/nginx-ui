<template>
    <div>
        <a-upload
            :before-upload="beforeUpload"
            :multiple="false"
            :show-upload-list="true"
            :file-list="uploadList"
        >
            <a-button :disabled="disabled"><a-icon type="upload"/>上传</a-button>
        </a-upload>
        <p style="margin: 15px 0" v-show="fileUrl">
            <a-icon type="paper-clip" style="margin-right: 5px"/>
            <a :href="server + '/' + fileUrl" target="_blank" @click="()=>{}">{{ fileUrl }}</a>
        </p>
    </div>
</template>

<script>
export default {
    name: "StdSingleFileUpload",
    props: {
        api: Function,
        id: {
            type: Number,
            default: null
        },
        fileUrl: {
            default: null
        },
        autoUpload: {
            type: Boolean,
            default: false
        },
        disabled: {
            type: Boolean,
            default: false
        }
    },
    data() {
        return {
            uploadList: [],
            server: process.env["VUE_APP_API_UPLOAD_ROOT"],
        }
    },
    model: {
        prop: 'fileUrl',
        event: 'changeFileUrl'
    },
    methods: {
        async upload() {
            if (this.uploadList.length) {
                const formData = new FormData()
                formData.append('file', this.uploadList.shift())
                this.visible = false
                this.uploading = true
                this.$message.info('正在上传附件, 请不要关闭本页')

                return this.api(this.id, formData).then(r => {
                    this.$emit('uploaded', r.url)
                    this.$emit('changeFileUrl', r.url)
                    this.uploading = false
                    this.$message.success('上传成功')
                }).catch(e => {
                    this.$message.error(e.message ? e.message : '上传失败')
                })
            }
        },
       beforeUpload(file) {
            this.uploadList = [file]
            this.$emit('changeFileUrl', file.name)
            // 有自动上传参数就自动上传，没有就看 id, 没有 id 就不上传
            if (this.autoUpload ? this.autoUpload : (!!this.id)) {
                this.upload()
            }
            return false
        },
    }
}
</script>

<style scoped>

</style>
