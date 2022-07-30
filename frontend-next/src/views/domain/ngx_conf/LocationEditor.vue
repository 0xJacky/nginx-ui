<template>
    <a-form-item :label="$gettext('Locations')" :key="update">
        <a-empty v-if="!locations"/>
        <a-card v-for="(v,k) in locations" :key="k"
                :title="$gettext('Location')" size="small">
            <a-form-item :label="$gettext('Comments')" v-if="v.comments">
                <p style="white-space: pre-wrap;">{{ v.comments }}</p>
            </a-form-item>
            <a-form-item :label="$gettext('Path')">
                <a-input addon-before="location" v-model="v.path"/>
            </a-form-item>
            <a-form-item :label="$gettext('Content')">
                <vue-itextarea v-model="v.content" :default-text-height="200"/>
            </a-form-item>
        </a-card>

        <a-modal :title="$gettext('Add Location')" v-model="adding" @ok="save">
            <a-form-item :label="$gettext('Comments')">
                <a-textarea v-model="location.comments"></a-textarea>
            </a-form-item>
            <a-form-item :label="$gettext('Path')">
                <a-input addon-before="location" v-model="location.path"/>
            </a-form-item>
            <a-form-item :label="$gettext('Content')">
                <vue-itextarea v-model="location.content" :default-text-height="200"/>
            </a-form-item>
        </a-modal>

        <div>
            <a-button block @click="add">{{ $gettext('Add Location') }}</a-button>
        </div>
    </a-form-item>
</template>

<script>
import VueItextarea from '@/components/VueItextarea/VueItextarea'

export default {
    name: 'LocationEditor',
    components: {VueItextarea},
    props: {
        locations: Array
    },
    data() {
        return {
            adding: false,
            location: {},
            update: 0
        }
    },
    methods: {
        add() {
            this.adding = true
            this.location = {}
        },
        save() {
            this.adding = false
            if (this.locations) {
                this.locations.push(this.location)
            } else {
                this.locations = [this.location]
            }
            this.update++
        },
        remove(index) {
            this.update++
            this.locations.splice(index, 1)
        }
    }
}
</script>

<style lang="less" scoped>
.ant-card {
    margin: 10px 0;
    box-shadow: unset;
}
</style>
