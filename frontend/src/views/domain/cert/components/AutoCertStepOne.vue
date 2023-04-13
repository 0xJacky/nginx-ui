<script setup lang="ts">
import {inject, Ref} from 'vue'
import {useGettext} from 'vue3-gettext'
import DNSChallenge from '@/views/domain/cert/components/DNSChallenge.vue'

const {$gettext} = useGettext()
const no_server_name: Ref = inject('no_server_name')!
const data: Ref = inject('data')!
</script>

<template>
    <template v-if="no_server_name">
        <a-alert
            :message="$gettext('Warning')"
            type="warning"
            show-icon
        >
            <template #description>
                    <span v-if="no_server_name">
                        {{ $gettext('server_name parameter is required') }}
                    </span>
            </template>
        </a-alert>
        <br/>
    </template>

    <a-alert type="info" show-icon :message="$gettext('Note')">
        <template #description>
            <p v-translate>
                The server_name
                in the current configuration must be the domain name you need to get the certificate, support
                multiple domains.
            </p>
            <p v-translate>
                The certificate for the domain will be checked every hour,
                and will be renewed if it has been more than 1 week since it was last issued.
            </p>
            <p v-if="data.challenge_method==='http01'" v-translate>
                Make sure you have configured a reverse proxy for .well-known
                directory to HTTPChallengePort before obtaining the certificate.
            </p>
            <p v-else-if="data.challenge_method==='dns01'" v-translate>
                Please fill in the API authentication credentials provided by your DNS provider.
                We will add a TXT record to the DNS records of your domain for ownership verification.
                Once the verification is complete, the record will be removed.
                Please note that the time configurations below are all in seconds.
            </p>
        </template>
    </a-alert>
    <br/>
    <a-form layout="vertical">
        <a-form-item :label="$gettext('Challenge Method')">
            <a-select v-model:value="data.challenge_method">
                <a-select-option value="http01">
                    {{ $gettext('HTTP01') }}
                </a-select-option>
                <a-select-option value="dns01">
                    {{ $gettext('DNS01') }}
                </a-select-option>
            </a-select>
        </a-form-item>
    </a-form>
    <d-n-s-challenge v-if="data.challenge_method==='dns01'"/>
</template>

<style lang="less" scoped>

</style>
