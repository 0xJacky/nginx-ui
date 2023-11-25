<template>
  <button class="VPSwitch" type="button" role="switch">
    <span class="check">
      <span class="icon" v-if="$slots.default">
        <slot/>
      </span>
    </span>
  </button>
</template>

<style lang="less">
.light {
  --vp-c-border: #c2c2c4;
  --vp-c-gray-soft: rgba(142, 150, 170, 0.14);
  --vp-c-indigo-1: #3451b2;
  --vp-c-white: #ffffff;
  --vp-c-black: #000000;
  --vp-c-text-1: rgba(60, 60, 67);
  --vp-c-text-2: rgba(60, 60, 67, 0.78);
  --vp-shadow-1: 0 1px 2px rgba(0, 0, 0, 0.04), 0 1px 2px rgba(0, 0, 0, 0.06);
  --vp-c-text-3: rgba(235, 235, 245, 0.38);
}

.dark {
  --vp-c-border: #3c3f44;
  --vp-c-gray-soft: rgba(101, 117, 133, 0.16);
  --vp-c-indigo-1: #a8b1ff;
  --vp-c-neutral: var(--vp-c-white);
  --vp-c-neutral-inverse: var(--vp-c-black);
  --vp-c-text-1: rgba(255, 255, 245, 0.86);
}

* {
  --vp-c-neutral: var(--vp-c-black);
  --vp-c-neutral-inverse: var(--vp-c-white);
  --vp-input-border-color: var(--vp-c-border);
  --vp-input-switch-bg-color: var(--vp-c-gray-soft);
  --vp-c-brand-1: var(--vp-c-indigo-1);
}

.VPSwitch {
  position: relative;
  border-radius: 11px;
  display: block;
  width: 40px;
  height: 22px;
  flex-shrink: 0;
  border: 1px solid var(--vp-input-border-color);
  background-color: var(--vp-input-switch-bg-color);
  transition: border-color 0.25s !important;
}

.VPSwitch:hover {
  border-color: var(--vp-c-brand-1);
}

.check {
  position: absolute;
  top: 1px;
  /*rtl:ignore*/
  left: 1px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background-color: var(--vp-c-neutral-inverse);
  box-shadow: var(--vp-shadow-1);
  transition: transform 0.25s !important;
}

.icon {
  position: relative;
  display: block;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  overflow: hidden;
}

.icon svg {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 12px;
  height: 12px;
  fill: var(--vp-c-text-2);
}

.dark .icon svg {
  fill: var(--vp-c-text-1);
  transition: opacity 0.25s !important;
}
</style>
