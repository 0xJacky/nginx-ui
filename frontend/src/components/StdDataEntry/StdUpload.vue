<template>
    <div v-if="type==='img'">
        <a-upload
            :before-upload="beforeUpload"
            :show-upload-list="false"
            class="avatar-uploader"
            list-type="picture-card"
        >
            <img v-if="fileUrl" :src="getFileUrl()" width="100">
            <div v-else>
                <a-icon :type="uploading ? 'loading' : 'plus'"/>
                <div class="ant-upload-text">
                    上传图片
                </div>
            </div>
        </a-upload>

        <a-modal
            v-if="crop"
            v-model="visible"
            cancelText="取消上传"
            class="cropper"
            okText="裁切"
            title="图片裁切"
            @cancel="visible=false;$emit('changeFileUrl', orig)"
            @ok="handleCropSuccess"
        >
            <div class="vue-cropper" v-if="fileUrl.substring(0,5) === 'data:'">
                <VueCropper
                    ref="cropper"
                    :autoCrop="true"
                    :autoCropHeight="cropOptions.autoCropHeight"
                    :autoCropWidth="cropOptions.autoCropWidth"
                    :fixed="cropOptions.fixed"
                    :fixedNumber="cropOptions.fixedNumber"
                    :img="getFileUrl()"
                    outputType="png"
                />
            </div>
            <div style="margin: 10px 0">
                <a-button @click="handleSingleUpload">不剪裁</a-button>
            </div>
        </a-modal>
    </div>

    <div v-else-if="type==='file'">
        <std-single-file-upload
            :file-url="fileUrl"
            :id="id"
            :api="api"
            :auto-upload="autoUpload"
            @changeFileUrl="url => {$emit('changeFileUrl', url)}"
            :disabled="disabled"
            ref="single-file"
        />
    </div>

    <div v-else-if="type==='multi-file'">
        <std-multi-files-upload
            :file-list="M_list"
            :id="id"
            :api="api"
            :auto-upload="autoUpload"
            :api_delete="api_delete"
            @changeFileUrl="url => {$emit('changeFileUrl', url)}"
            :disabled="disabled"
            ref="multi-file"
        />
    </div>

</template>

<script>
import Vue from 'vue'
import VueCropper from 'vue-cropper'
import StdSingleFileUpload from "@/components/StdDataEntry/StdSingleFileUpload";
import StdMultiFilesUpload from "@/components/StdDataEntry/StdMultiFilesUpload";
import { v4 as uuidv4 } from 'uuid';

Vue.use(VueCropper)

export default {
    name: 'StdUpload',
    components: {StdMultiFilesUpload, StdSingleFileUpload},
    props: {
        id: {
            type: Number,
            default: null
        },
        api: Function,
        api_delete: {
            type: Function,
            default: null
        },
        fileUrl: {
            default: ''
        },
        autoUpload: {
            type: Boolean,
            default: false
        },
        type: {
            default: 'img',
            validator: value => {
                return ['img', 'file', 'multi-file'].indexOf(value) !== -1
            }
        },
        crop: {
            type: Boolean,
            default: false
        },
        cropOptions: {
            type: Object,
            default: () => {
                return {
                    fixed: true,
                    autoCropWidth: 200,
                    autoCropHeight: 200,
                }
            }
        },
        list: {
            default: null
        },
        disabled: {
            type: Boolean,
            default: false
        }
    },
    data() {
        return {
            uploading: false,
            orig: '',
            visible: false,
            fileList: [],
            M_list: this.list,
            server: process.env["VUE_APP_API_UPLOAD_ROOT"]
        }
    },
    created() {
        this.orig = this.fileUrl
    },
    model: {
        prop: 'fileUrl',
        event: 'changeFileUrl'
    },
    watch: {
        list() {
            this.M_list = this.list
        }
    },
    methods: {
        getFileUrl() {
            return this.fileUrl.substring(0,5) === 'data:' ? this.fileUrl :
                this.server + '/' + this.fileUrl
        },
        async upload() {
            if (this.type === 'multi-file') {
                return await this.$refs["multi-file"].upload()
            }
            if (this.orig && this.fileUrl !== this.orig) {
                return this.handleSingleUpload()
            }
            if (this.$refs['single-file']) {
                return await this.$refs["single-file"].upload()
            }
        },
        handleSingleUpload() {
            const formData = new FormData()
            formData.append('file', this.fileList[0])
            this.visible = false
            this.uploading = true
            this.$message.info('正在上传附件, 请不要关闭本页')

            return this.api(this.id, formData).then(r => {
                this.$emit('uploaded', r.url)
                this.$emit('changeFileUrl', r.url)
                this.uploading = false
                this.$message.success('上传成功')
                this.orig = r.url
            })

        },
        beforeUpload(file) {
            // 赋予新值之前做个备份 emm 生气了哼!!!
            this.orig = this.fileUrl ? this.fileUrl : 'orig_is_empty'
            this.fileList = [file]
            if (this.type === 'img') {
                this.visible = true
                const r = new FileReader()
                r.readAsDataURL(file)
                r.onload = e => {
                    file.thumbUrl = e.target.result
                    this.$emit('changeFileUrl', e.target.result)
                }
            } else {
                this.$emit('changeFileUrl', file.name)
            }
            return false
        },
        afterCropUpload(file) {
            this.visible = true
            const r = new FileReader()
            r.readAsDataURL(file)
            r.onload = e => {
                file.thumbUrl = e.target.result
                this.$emit('changeFileUrl', e.target.result)
            }
            this.fileList = [file]
            this.$nextTick(() => {
                this.handleSingleUpload()
            })
        },
        handleCropSuccess() {
            this.$refs.cropper.getCropBlob((data) => {
                let file = new window.File([data], uuidv4() + '.png', {type: data.type})
                this.afterCropUpload(file)
                this.visible = false
            })
        },
        remove(r) {
            this.fileList = this.fileList.filter(value => {
                return value !== r
            })
        },
    }
}
</script>

<style lang="less" scoped>
.upload-picture-btn {
    font-size: 20px;
    color: #999999;
}

.cropper {
    .ant-modal-body {
        min-height: 256px;
    }
}

.vue-cropper {
    min-height: 200px;
    background-image: unset;
}

.img-preview {
    float: left;
    border: 1px solid #8e8e904d;
    border-radius: 5px;
    margin: 5px;
    padding: 5px;

    img {
        height: 90px;
        width: 90px;
        object-fit: cover;
    }
}
</style>
