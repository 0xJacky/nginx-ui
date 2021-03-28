<template>
    <a-textarea
        :value="current_value"
        @keydown="updateValue($event)"
        @input="change($event)"
        :rows="36"
    />
</template>


<script>
export default {
    name: 'vue-itextarea',
    props: {
        value: {},
        TAB_SIZE: {default: 4},
    },
    model: {
        prop: 'value',
        event: 'changeValue'
    },
    watch: {
        value() {
            this.current_value = this.value ? this.value : ''
        }
    },
    data() {
        return {
            current_value: this.value ? this.value : ''
        };
    },
    methods: {
        /**
         * Keyboard shortcuts support, like <ctrl-v>
         */
        change(event) {
            this.$emit('input', event.target.value);
        },
        updateValue(event) {
            let target = event.target;
            let value = target.value;
            let start = target.selectionStart;
            let end = target.selectionEnd;
            if (event.key === 'Escape') {
                if (event.target.nextElementSibling) event.target.nextElementSibling.focus();
                else (event.target.blur());
                return;
            }
            if (event.key === 'Tab' && !event.metaKey) {
                event.preventDefault();
                value = value.substring(0, start) + ' '.repeat(this.TAB_SIZE) + value.substring(end);
                event.target.value = value;
                setTimeout(() => target.selectionStart = target.selectionEnd = start + this.TAB_SIZE, 0);
            }
            if (event.key === 'Backspace' && !event.metaKey) {
                let chars_before_cursor = value.substr(start - this.TAB_SIZE, this.TAB_SIZE);
                if (chars_before_cursor === ' '.repeat(this.TAB_SIZE)) {
                    event.preventDefault();
                    value = value.substring(0, start - this.TAB_SIZE) + value.substring(end);
                    event.target.value = value;
                    setTimeout(() => target.selectionStart = target.selectionEnd = start - this.TAB_SIZE, 0)
                }
            }
            if (event.key === 'Enter') {
                let current_line = value.substr(0, start).split("\n").pop(); // line, we are currently on
                if (current_line && current_line.startsWith(' '.repeat(this.TAB_SIZE))) { // type tab
                    event.preventDefault();
                    // detect how many tabs in the begining and apply
                    let spaces_count = current_line.search(/\S|$/); // position of first non white-space character
                    let tabs_count = spaces_count ? spaces_count / this.TAB_SIZE : 0;
                    value = value.substring(0, start) + '\n' + ' '.repeat(this.TAB_SIZE).repeat(tabs_count) + this.current_value.substring(end);
                    event.target.value = value;
                    setTimeout(() => target.selectionStart = target.selectionEnd = start + this.TAB_SIZE * tabs_count + 1, 0);
                }
            }
            setTimeout(() => {
                this.current_value = event.target.value;
                this.$emit('input', event.target.value);
            }, 0);
        },
    },
};
</script>
