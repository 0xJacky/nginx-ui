<script setup lang="ts">
import type { Component } from 'vue'
import { ArrowLeftOutlined, ArrowRightOutlined, SaveOutlined } from '@ant-design/icons-vue'
import { storeToRefs } from 'pinia'
import { computed, onBeforeUnmount } from 'vue'
import useSystemSettingsStore from '../../store'
import StepDetectPlatform from './steps/StepDetectPlatform.vue'
import StepInstall from './steps/StepInstall.vue'
import StepSshTarget from './steps/StepSshTarget.vue'
import StepTrustAndTest from './steps/StepTrustAndTest.vue'
import StepVerify from './steps/StepVerify.vue'
import {
  createHostSetupWizard,
  hostSetupStepOrder,
  provideHostSetupWizard,
} from './useHostSetupWizard'

const props = defineProps<{ saving?: boolean }>()
const emit = defineEmits<{ save: [] }>()

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
const wizard = createHostSetupWizard(data)
provideHostSetupWizard(wizard)

const stepComponents: Record<typeof wizard.currentStepId.value, Component> = {
  'ssh-target': StepSshTarget,
  'trust-and-test': StepTrustAndTest,
  'detect-platform': StepDetectPlatform,
  'install': StepInstall,
  'verify': StepVerify,
}
const activeStepComponent = computed(() => stepComponents[wizard.currentStepId.value])

const steps = computed(() => [
  { title: $gettext('SSH Target'), description: $gettext('Address, user and private key') },
  { title: $gettext('Trust & Test'), description: $gettext('Host key and connectivity') },
  { title: $gettext('Detect Platform'), description: $gettext('Service manager and nginx paths') },
  { title: $gettext('Install'), description: $gettext('Container and host snippets') },
  { title: $gettext('Verify'), description: $gettext('Run checks and save') },
].map((step, index) => ({
  ...step,
  disabled: index > wizard.furthestReachableIndex.value && index > wizard.currentStepIndex.value,
})))

const isLastStep = computed(() => wizard.currentStepId.value === hostSetupStepOrder[hostSetupStepOrder.length - 1])

function saveToSettings() {
  if (wizard.applyToSettings())
    emit('save')
}

onBeforeUnmount(wizard.clearSensitiveState)
</script>

<template>
  <section class="host-setup-wizard">
    <ASteps
      :current="wizard.currentStepIndex.value"
      class="wizard-steps"
      size="small"
      @change="wizard.goToStep"
    >
      <AStep
        v-for="step in steps"
        :key="step.title"
        :title="step.title"
        :description="step.description"
        :disabled="step.disabled"
      />
    </ASteps>

    <AForm layout="vertical" class="wizard-step-content">
      <KeepAlive>
        <component
          :is="activeStepComponent"
          :key="wizard.currentStepId.value"
        />
      </KeepAlive>
    </AForm>

    <AAlert
      v-if="wizard.blockedReason.value"
      type="info"
      show-icon
      class="mt-4"
      :message="wizard.blockedReason.value"
    />

    <div class="wizard-actions">
      <ASpace wrap>
        <AButton :disabled="wizard.currentStepIndex.value === 0" @click="wizard.previous">
          <ArrowLeftOutlined />
          {{ $gettext('Previous') }}
        </AButton>
      </ASpace>
      <ASpace wrap>
        <AButton
          v-if="!isLastStep"
          type="primary"
          :disabled="!wizard.canAdvance.value"
          @click="wizard.next"
        >
          {{ $gettext('Next') }}
          <ArrowRightOutlined />
        </AButton>
        <AButton
          v-else
          type="primary"
          :disabled="!wizard.isVerificationPassed.value"
          :loading="props.saving"
          @click="saveToSettings"
        >
          <SaveOutlined />
          {{ $gettext('Save configuration') }}
        </AButton>
      </ASpace>
    </div>
  </section>
</template>

<style lang="less" scoped>
.wizard-steps {
  margin-bottom: 28px;
}

.wizard-step-content {
  min-height: 360px;
}

.wizard-actions {
  position: sticky;
  z-index: 5;
  bottom: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: space-between;
  margin-top: 32px;
  padding: 16px 0;
  border-top: 1px solid #e8e8e8;
  background: #fff;
}

// The theme class is set on <body> by the settings store. Ant design tokens are
// not exposed as CSS variables in this project. The whole selector must sit
// inside :global, otherwise the scoped transform drops the rule.
:global(body.dark .wizard-actions) {
  border-color: #303030;
  background: #141414;
}

@media (max-width: 600px) {
  .wizard-steps {
    margin-bottom: 20px;
  }

  .wizard-step-content {
    min-height: 280px;
  }

  .wizard-actions {
    position: static;
  }
}
</style>
