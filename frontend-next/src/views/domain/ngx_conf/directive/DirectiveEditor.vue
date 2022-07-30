<template>
    <a-form-item :label="$gettext('Directives')">
        <div v-for="(directive,k) in ngx_directives" :key="k" @click="current_idx=k">
            <vue-itextarea v-if="directive.directive === If" v-model="directive.params" :default-text-height="100"/>
            <a-input :addon-before="directive.directive" v-model="directive.params" @click="current_idx=k" v-else>
                <a-popconfirm slot="suffix" @confirm="remove(k)"
                              :title="$gettext('Are you sure you want to remove this directive?')"
                              :ok-text="$gettext('Yes')"
                              :cancel-text="$gettext('No')">
                    <a-icon type="close"
                            style="color: rgba(0,0,0,.45);font-size: 10px;"
                    />
                </a-popconfirm>
            </a-input>
            <transition name="slide">
                <div v-if="current_idx===k" class="extra">
                    <div class="extra-content">
                        <a-form-item :label="$gettext('Comments')">
                            <a-textarea v-model="directive.comments"/>
                        </a-form-item>
                        <directive-add :ngx_directives="ngx_directives" :idx="k" @save="onSave(k)"/>
                    </div>
                </div>
            </transition>
        </div>
        <directive-add :ngx_directives="ngx_directives"/>
    </a-form-item>
</template>

<script>
import VueItextarea from '@/components/VueItextarea/VueItextarea'
import {If} from '../ngx_constant'
import DirectiveAdd from '@/views/domain/ngx_conf/directive/DirectiveAdd'

export default {
    name: 'DirectiveEditor',
    props: {
        ngx_directives: Array
    },
    components: {
        DirectiveAdd,
        VueItextarea
    },
    data() {
        return {
            adding: false,
            directive: {},
            If,
            current_idx: -1,
        }
    },
    methods: {
        add() {
            this.adding = true
            this.directive = {}
        },
        save() {
            this.adding = false
            this.ngx_directives.push(this.directive)
        },
        remove(index) {
            this.ngx_directives.splice(index, 1)
        },
        onSave(idx) {
            const that = this
            setTimeout(() => {
                that.current_idx = idx + 1
            }, 50)
        }
    }
}
</script>

<style lang="less" scoped>
.extra {
    background-color: #fafafa;
    padding: 10px 20px 20px;
    margin-bottom: 10px;
}

.slide-enter-active, .slide-leave-active {
    transition: max-height .5s ease;
}

.slide-enter, .slide-leave-to {
    max-height: 0;
}

.slide-enter-to, .slide-leave {
    max-height: 600px;
}
</style>
