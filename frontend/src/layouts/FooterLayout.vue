<template>
    <div class="footer center">
        Copyright © 2020 - {{ thisYear }} Nginx UI
    </div>
</template>

<script>
export default {
    name: 'FooterComponent',
    data() {
        return {
            thisYear: new Date().getFullYear()
        }
    }
}
</script>

<style scoped>
.footer {

}
</style>
