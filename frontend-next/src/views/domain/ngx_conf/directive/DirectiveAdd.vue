<template>
    <div>
        <div class="add-directive-temp" v-if="adding">
            <a-select v-model="mode" default-value="default" style="min-width: 150px">
                <a-select-option value="default">
                    {{ $gettext('Single Directive') }}
                </a-select-option>
                <a-select-option value="if">
                    if
                </a-select-option>
            </a-select>
            <vue-itextarea v-if="mode===If" :default-text-height="100" v-model="directive.params"/>
            <a-input-group compact v-else>
                <a-input style="width: 30%" :placeholder="$gettext('Directive')" v-model="directive.directive"/>
                <a-input style="width: 70%" :placeholder="$gettext('Params')" v-model="directive.params">
                    <a-icon slot="suffix" type="close" style="color: rgba(0,0,0,.45);font-size: 10px;"
                            @click="adding=false"/>
                </a-input>
            </a-input-group>
        </div>
        <a-button block v-if="!adding" @click="add">{{ $gettext('Add Directive Below') }}</a-button>
        <a-button type="primary" v-else block @click="save"
                  :disabled="!directive.directive&&!directive.params">{{ $gettext('Save Directive') }}
        </a-button>
    </div>
</template>

<script>
import {If} from '@/views/domain/ngx_conf/ngx_constant'
import VueItextarea from '@/components/VueItextarea/VueItextarea'

export default {
    name: 'DirectiveAdd',
    components: {
        VueItextarea
    },
    props: {
        ngx_directives: Array,
        idx: Number,
    },
    data() {
        return {
            adding: false,
            directive: {},
            mode: 'default',
            If
        }
    },
    methods: {
        add() {
            this.adding = true
            this.directive = {}
        },
        save() {
            this.adding = false
            if (this.mode === If) {
                this.directive.directive = If
            }

            if (this.idx) {
                this.ngx_directives.splice(this.idx + 1, 0, this.directive)
            } else {
                this.ngx_directives.push(this.directive)
            }

            this.$emit('save', this.idx)
        }
    }
}
</script>

<style lang="less" scoped>

</style>
