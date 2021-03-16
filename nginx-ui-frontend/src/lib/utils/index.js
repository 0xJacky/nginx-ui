import scrollPosition from './scroll-position'

export default {
    // eslint-disable-next-line no-unused-vars
    install(Vue, options) {
        Vue.prototype.extend = (target, source) => {
            for (let obj in source) {
                target[obj] = source[obj]
            }
            return target
        }

        Vue.prototype.getClientWidth = () => {
            return document.body.clientWidth
        }

        Vue.prototype.collapse = () => {
            return !(Vue.prototype.getClientWidth() > 768 || Vue.prototype.getClientWidth() < 512)
        }

        Vue.prototype.bytesToSize = (bytes) => {
            if (bytes === 0) return '0 B'

            const k = 1024

            const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']

            const i = Math.floor(Math.log(bytes) / Math.log(k))
            return (bytes / Math.pow(k, i)).toPrecision(3) + ' ' + sizes[i]
        }

        Vue.prototype.transformUserType = (power) => {
            const type = ['学生', '企业', '教师', '学院']
            return type[power]
        }

        Vue.prototype.transformGrade = {
            7: 'A+',
            6: 'A',
            5: 'B+',
            4: 'B',
            3: 'C+',
            2: 'C',
            1: 'D',
            0: 'F'
        }

        Vue.prototype.scrollPosition = scrollPosition
    }
}
