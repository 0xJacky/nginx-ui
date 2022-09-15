<script setup lang="ts">
import {computed, ref} from 'vue'

const props = defineProps(['value', 'generate', 'placeholder'])
const emit = defineEmits(['update:value'])

const M_value = computed({
    get() {
        return props.value
    },
    set(v) {
        emit('update:value', v)
    }
})
const visibility = ref(false)

function handle_generate() {
    visibility.value = true
    M_value.value = 'xxxx'

    const chars = '0123456789abcdefghijklmnopqrstuvwxyz!@#$%^&*()ABCDEFGHIJKLMNOPQRSTUVWXYZ'
    const passwordLength = 12
    let password = ''
    for (let i = 0; i <= passwordLength; i++) {
        const randomNumber = Math.floor(Math.random() * chars.length)
        password += chars.substring(randomNumber, randomNumber + 1)
    }

    M_value.value = password

}
</script>

<template>
    <a-input-group compact>
        <a-input-password
                v-if="!visibility"
                :class="{compact: generate}"
                v-model:value="M_value" :placeholoder="placeholder"/>
        <a-input v-else :class="{compact: generate}" v-model:value="M_value" :placeholoder="placeholder"/>
        <a-button @click="handle_generate" v-if="generate" type="primary">
            <translate>Generate</translate>
        </a-button>
    </a-input-group>
</template>

<style scoped>
.compact {
    width: calc(100% - 91px)
}
</style>