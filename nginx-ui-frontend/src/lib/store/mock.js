export const mock = {
    namespace: true,
    state: {
        user: {
            name: 'mock 用户',
            school_id: '201904020209',
            superuser: true,
            // 0学生 1企业 2教师 3学院
            power: 2,
            gender: 1,
            phone: "10086",
            email: 'me@jackyu.cn',
            description: '前端、后端、系统架构',
            college_id: 1,
            college_name: "大数据与互联网学院",
            major: 1,
            major_name: "物联网工程",
            position: 'HR'
        }
    },
    mutations: {
        update_mock_user(state, payload) {
            for (const k in payload) {
                state.user[k] = payload[k]
            }
        }
    },
    actions: {
        async update_mock_user({commit}, data) {
            commit('update_mock_user', data)
        }
    },
    getters: {
        user(state) {
            return state.user
        },
    }
}
