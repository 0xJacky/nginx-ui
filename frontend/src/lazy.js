import Vue from 'vue'
import {
    Alert,
    Avatar,
    Badge,
    Breadcrumb,
    Button,
    Card,
    Checkbox,
    Col,
    Collapse,
    Comment,
    ConfigProvider,
    DatePicker,
    Descriptions,
    Divider,
    Drawer,
    Dropdown,
    Empty,
    Form,
    Icon,
    Input,
    InputNumber,
    Layout,
    List,
    Menu,
    message,
    Modal,
    notification,
    pageHeader,
    Pagination,
    Popconfirm,
    Popover,
    Progress,
    Radio,
    Result,
    Row,
    Select,
    Skeleton,
    Slider,
    Space,
    Spin,
    Statistic,
    Steps,
    Switch,
    Table,
    Tabs,
    Tooltip,
    Transfer,
    Upload
} from 'ant-design-vue'

Vue.use(ConfigProvider)
Vue.use(Layout)
Vue.use(Input)
Vue.use(InputNumber)
Vue.use(Button)
Vue.use(Radio)
Vue.use(Checkbox)
Vue.use(Select)
Vue.use(Collapse)
Vue.use(Card)
Vue.use(Form)
Vue.use(Row)
Vue.use(Col)
Vue.use(Modal)
Vue.use(Table)
Vue.use(Tabs)
Vue.use(Icon)
Vue.use(Badge)
Vue.use(Popover)
Vue.use(Dropdown)
Vue.use(List)
Vue.use(Avatar)
Vue.use(Breadcrumb)
Vue.use(Steps)
Vue.use(Spin)
Vue.use(Menu)
Vue.use(Drawer)
Vue.use(Tooltip)
Vue.use(Alert)
Vue.use(Divider)
Vue.use(DatePicker)
Vue.use(Upload)
Vue.use(Progress)
Vue.use(Skeleton)
Vue.use(Popconfirm)
Vue.use(notification)
Vue.use(Empty)
Vue.use(Statistic)
Vue.use(Pagination)
Vue.use(Slider)
Vue.use(Transfer)
Vue.use(Comment)
Vue.use(Descriptions)
Vue.use(Result)
Vue.use(pageHeader)
Vue.use(Switch)
Vue.use(Space)

Vue.prototype.$confirm = Modal.confirm
Vue.prototype.$message = message
Vue.prototype.$notification = notification
Vue.prototype.$info = Modal.info
Vue.prototype.$success = Modal.success
Vue.prototype.$error = Modal.error
Vue.prototype.$warning = Modal.warning
