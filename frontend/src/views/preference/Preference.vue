<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {reactive} from 'vue'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import {useSettingsStore} from '@/pinia'
import {dark_mode} from '@/lib/theme'

const {$gettext} = useGettext()

const settingsStore = useSettingsStore()

const data = reactive({
    theme: settingsStore.theme
})

function save() {
    settingsStore.set_theme(data.theme)
    settingsStore.set_preference_theme(data.theme)
    dark_mode(data.theme === 'dark')
}
</script>

<template>
    <a-card :title="$gettext('Preference')">
        <div class="preference-container">
            <a-form layout="vertical">
                <a-form-item :label="$gettext('Theme')">
                    <a-select v-model:value="data.theme">
                        <a-select-option value="auto">
                            {{ $gettext('Auto') }}
                        </a-select-option>
                        <a-select-option value="light">
                            {{ $gettext('Light') }}
                        </a-select-option>
                        <a-select-option value="dark">
                            {{ $gettext('Dark') }}
                        </a-select-option>
                    </a-select>
                </a-form-item>
            </a-form>
        </div>
    </a-card>
    <footer-tool-bar>
        <a-button type="primary" @click="save">
            {{ $gettext('Save') }}
        </a-button>
    </footer-tool-bar>
</template>

<style lang="less" scoped>
.preference-container {
    width: 100%;
    max-width: 600px;
    margin: 0 auto;
    padding: 0 10px;
}
</style>
