<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import * as Icons from '@element-plus/icons-vue'
import * as echarts from 'echarts'

type Locale = 'zh-CN' | 'en-US'
type DrawerType = 'user' | 'group' | 'channel' | 'active' | 'item' | 'field' | 'rule' | ''
type DrawerMode = 'create' | 'edit'

interface Setting {
  key: string
  value: string
  value_type: string
  description: string
}

interface User {
  id: number
  username: string
  role: string
  display_name: string
  enabled: boolean
  last_login_at?: string
}

interface Group {
  id: number
  code: string
  name: string
  icon: string
  description: string
  default_interval_seconds: number
  missed_times_threshold: number
  alert_enabled: boolean
  enabled: boolean
  response_settings_json: string
  sort_order: number
}

interface Item {
  id: number
  group_id: number
  source_type: string
  name: string
  description: string
  interval_seconds: number
  missed_times_threshold: number
  alert_enabled: boolean
  enabled: boolean
  response_settings_json: string
  ref_item_id?: number
  ref_item_name?: string
  last_seen_at?: string
  created_at?: string
}

interface ActiveRequest {
  id: number
  group_id: number
  item_id: number
  name: string
  url: string
  method: string
  headers_json: string
  body_type: string
  body_json: string
  interval_seconds: number
  timeout_seconds: number
  expected_status_code: number
  enabled: boolean
  last_seen_at?: string
}

interface FieldDefinition {
  id: number
  scope_type: 'group' | 'item'
  group_id: number
  item_id?: number
  field_path: string
  display_name: string
  value_type: string
  unit: string
  required: boolean
  enabled: boolean
  ref_group_id?: number
  ref_name_path?: string
}

interface Channel {
  id: number
  code: string
  name: string
  channel_type: string
  config_json: string
  enabled: boolean
  is_default: boolean
}

interface AlertRule {
  id: number
  name: string
  scope_type: string
  group_id?: number
  item_id?: number
  field_definition_id?: number
  source_type: string
  rule_type: string
  field_path: string
  value_type: string
  operator: string
  threshold_value: string
  aggregate_func: string
  aggregate_window_seconds?: number
  aggregate_sample_count?: number
  consecutive_count: number
  recovery_count: number
  severity: string
  message_template: string
  combine_group: string
  continuous_alert?: boolean
  enabled: boolean
  channel_ids?: number[]
}

interface SampleValue {
  field_path: string
  value_type: string
  string_value?: string
  integer_value?: number
  float_value?: number
  boolean_value?: boolean
  numeric_value?: number
  raw_value?: unknown
}

interface Sample {
  id: number
  group_id: number
  item_id: number
  source_type: string
  name: string
  received_at: string
  status: string
  http_status_code?: number
  latency_ms?: number
  error_message?: string
  values?: SampleValue[]
}

interface AlertEvent {
  id: number
  rule_id: number
  group_id?: number
  item_id?: number
  sample_id?: number
  event_type: string
  severity: string
  title: string
  message: string
  field_path: string
  current_value?: string
  threshold_value?: string
  occurred_at: string
}

interface StatPoint {
  time: string
  value?: number
}

interface StatResult {
  group_id: number
  item_id: number
  field_path: string
  count: number
  avg?: number
  max?: number
  min?: number
  median?: number
  latest?: unknown
  latest_at?: string
  series?: StatPoint[]
}

const locale = ref<Locale>((localStorage.getItem('locale') as Locale) || 'zh-CN')
const installed = ref(false)
const currentUser = ref<User | null>(null)
const activeMenu = ref('overview')
const sidebarCollapsed = ref(localStorage.getItem('sidebarCollapsed') === 'true')
const groupActiveTab = ref('items')
const itemPage = ref(1)
const itemPageSize = ref(20)
const itemNameSearch = ref('')
const itemStatusFilter = ref('')
const itemSortProp = ref('')
const itemSortOrder = ref('')

const state = reactive({
  dashboard: { groups: 0, items: 0, samples_24h: 0, alerting_rules: 0, recent_events: [] as AlertEvent[] },
  settings: [] as Setting[],
  users: [] as User[],
  groups: [] as Group[],
  items: [] as Item[],
  activeRequests: [] as ActiveRequest[],
  fields: [] as FieldDefinition[],
  channels: [] as Channel[],
  rules: [] as AlertRule[],
  samples: [] as Sample[],
  events: [] as AlertEvent[],
})

const activeRequestsCount = ref(0)
const loading = computed(() => activeRequestsCount.value > 0)

const authForm = reactive({ username: '', password: '' })
const drawer = reactive({ visible: false, type: '' as DrawerType, mode: 'create' as DrawerMode, title: '' })
const fieldDetail = reactive({
  visible: false,
  item: null as Item | null,
  field: null as FieldDefinition | null,
  stats: null as StatResult | null,
})

const userForm = reactive({ id: 0, username: '', password: '', display_name: '', role: 'viewer', enabled: true })
const groupForm = reactive({
  id: 0,
  code: '',
  name: '',
  icon: 'Monitor',
  description: '',
  default_interval_seconds: 60,
  missed_times_threshold: 3,
  alert_enabled: true,
  enabled: true,
  response_settings_json: '{}',
  sort_order: 0,
})
const activeForm = reactive({
  id: 0,
  group_id: 0,
  name: '',
  url: '',
  method: 'GET',
  headers_json: '{}',
  body_type: 'none',
  body_json: '{}',
  interval_seconds: 60,
  timeout_seconds: 10,
  expected_status_code: 200,
  enabled: true,
})
const itemForm = reactive({
  id: 0,
  group_id: 0,
  source_type: 'passive',
  name: '',
  description: '',
  interval_seconds: 60,
  missed_times_threshold: 3,
  alert_enabled: true,
  enabled: true,
  response_settings_json: '{}',
})
const fieldForm = reactive({
  id: 0,
  scope_type: 'group' as 'group' | 'item',
  group_id: 0,
  item_id: undefined as number | undefined,
  field_path: '',
  display_name: '',
  value_type: 'float',
  unit: '',
  required: false,
  enabled: true,
  ref_group_id: undefined as number | undefined,
  ref_name_path: '',
})
const ruleForm = reactive({
  id: 0,
  name: '',
  scope_type: 'group',
  group_id: undefined as number | undefined,
  item_id: undefined as number | undefined,
  source_type: 'any',
  rule_type: 'field_condition',
  field_path: '',
  value_type: 'float',
  operator: 'gt',
  threshold_value: '',
  aggregate_func: 'avg',
  aggregate_window_seconds: 300,
  aggregate_sample_count: undefined as number | undefined,
  consecutive_count: 1,
  recovery_count: 1,
  severity: 'warning',
  message_template: '',
  combine_group: '',
  continuous_alert: false,
  enabled: true,
  channel_ids: [] as number[],
})
const channelForm = reactive({
  id: 0,
  code: '',
  name: '',
  channel_type: 'dingding',
  webhook: '',
  secret: '',
  bot_token: '',
  chat_id: '',
  enabled: true,
  is_default: false,
})

const iconOptions = [
  'Monitor', 'Platform', 'Cpu', 'Connection', 'DataLine', 'TrendCharts', 'Bell', 'Folder', 'Histogram',
  'Coin', 'Box', 'Cloudy', 'Document', 'SetUp', 'Odometer', 'Link', 'Key', 'Share', 'Tickets'
]

const text = {
  'zh-CN': {
    invalidJSON: '无效的 JSON 格式',
    install: '初始化系统',
    login: '登录',
    loginFailed: '账号或密码错误',
    username: '账号',
    password: '密码',
    submit: '提交',
    language: '语言',
    refresh: '刷新',
    logout: '退出',
    collapse: '收起菜单',
    expand: '展开菜单',
    add: '添加',
    search: '搜索',
    edit: '编辑',
    delete: '删除',
    cancel: '取消',
    save: '保存',
    saved: '已保存',
    deleted: '已删除',
    confirmDelete: '确认删除该记录？',
    emptyFields: '未设定',
    inherited: '继承分组字段',
    overridden: '该条目已单独设定字段，分组字段不生效。',
    noGroup: '请先创建或选择一个分组。',
    groupMenu: '分组',
    messageVars: '可用变量：{{item}}、{{field}}、{{current}}、{{threshold}}、{{severity}}。',
    thresholdHelp: '支持设置 JSON 字段作为动态阈值，格式如 "json:data.field1"',
    noEvents: '暂无报警记录。',
    refreshed: '已刷新',
    nav: { overview: '总览', groups: '分组', users: '用户', settings: '系统', channels: '通知渠道', alertRecords: '报警记录' },
    labels: {
      monitorGroups: '监控分组',
      groupSettings: '分组设定',
      items: '监控条目',
      activeItems: '主动监控条目',
      groupFields: '分组监控字段',
      itemFields: '条目监控字段',
      groupRules: '分组报警规则',
      itemRules: '条目报警规则',
      latestFields: '最新字段',
      fieldDetail: '字段详情',
      trend: '统计曲线',
      rules: '报警设置',
      events: '报警记录',
      users: '用户列表',
      channels: '通知渠道',
      settings: '系统参数',
      dataRetentionDays: '数据保留天数',
      defaultLocale: '默认语言',
      sessionTimeoutMinutes: '会话超时分钟',
      uploadToken: '上报接口密钥',
      appTimezone: '系统时区',
      channelTemplate: '报警方式',
      hourlyAlerts: '最近24小时报警',
      normalItems: '成功条目',
      totalItems: '总条目',
      inheritedFields: '自定义字段',
      ownFields: '单独设定字段',
      timeRange: '时间范围',
    },
    field: {
      continuousAlert: '持续报警',
      createdDate: '添加日期',
      searchPlaceholder: '搜索名称',
      statusAll: '所有状态',
      statusOk: '正常',
      statusError: '异常',
      action: '操作',
      code: '标记',
      name: '名称',
      icon: '图标',
      description: '描述',
      status: '状态',
      type: '类型',
      source: '来源',
      group: '分组',
      item: '条目',
      displayName: '显示名',
      role: '角色',
      lastLogin: '登录时间',
      enabled: '启用',
      alert: '报警',
      intervalSeconds: '间隔秒',
      missedTimes: '缺失次数',
      lastSeen: '最后成功',
      url: 'URL',
      method: '方法',
      bodyType: '请求体',
      timeout: '超时秒',
      expectedStatus: '期望状态码',
      headers: 'Header JSON',
      body: 'Body JSON',
      fieldPath: 'JSON 字段',
      valueType: '值类型',
      unit: '单位',
      required: '必填',
      ruleType: '规则',
      operator: '操作符',
      threshold: '阈值',
      aggregate: '聚合',
      window: '窗口秒',
      sampleCount: '样本数',
      consecutive: '连续次数',
      recovery: '恢复次数',
      severity: '级别',
      messageTemplate: '通知模板',
      channel: '通知渠道',
      channels: '通知渠道',
      combineGroup: '合并报警组别',
      webhook: '机器人 Webhook',
      secret: '签名密钥 (Secret)(可选)',
      botToken: 'Bot Token',
      chatID: 'Chat ID',
      value: '值',
      time: '时间',
      latest: '最新',
      count: '数量',
      avg: '平均值',
      max: '最大值',
      min: '最小值',
      median: '中位数',
      title: '标题',
      message: '消息',
      responseSettings: '被动返回参数 (JSON)',
      refGroup: '对应分组',
      refNamePath: '条目名称json字段',
      refItem: '上级条目',
      sortOrder: '排序',
    },
    yes: '是',
    no: '否',
    roles: { admin: '管理员', viewer: '查看者' },
    sourceTypes: { passive: '被动', active: '主动', any: '任意' },
    valueTypes: { string: '字符串', integer: '整数', float: '浮点', boolean: '布尔', object_array: '对象数组', string_array: '字符串数组' },
    ruleTypes: { missing_data: '缺失数据', request_failed: '请求失败', field_condition: '字段条件', aggregate_condition: '聚合条件' },
    operators: { gt: '>', gte: '>=', lt: '<', lte: '<=', eq: '=', ne: '!=', contains: '包含', not_contains: '不包含', exists: '字段存在', not_exists: '字段不存在', len_eq: '长度=', len_gt: '长度>', len_lt: '长度<', len_ne: '长度!=' },
    severities: { info: '信息', warning: '警告', critical: '严重', recovered: '恢复' },
    channelTemplates: { dingding: '钉钉机器人报警', feishu: '飞书机器人报警', telegram: 'Telegram 报警' },
  },
  'en-US': {
    invalidJSON: 'Invalid JSON format',
    install: 'Initialize',
    login: 'Login',
    loginFailed: 'Invalid username or password',
    username: 'Username',
    password: 'Password',
    submit: 'Submit',
    language: 'Language',
    refresh: 'Refresh',
    logout: 'Log out',
    collapse: 'Collapse menu',
    expand: 'Expand menu',
    add: 'Add',
    search: 'Search',
    edit: 'Edit',
    delete: 'Delete',
    cancel: 'Cancel',
    save: 'Save',
    saved: 'Saved',
    deleted: 'Deleted',
    confirmDelete: 'Delete this record?',
    emptyFields: 'Not Set',
    inherited: 'Using group fields',
    overridden: 'This item has its own fields. Group fields are disabled for it.',
    noGroup: 'Create or select a group first.',
    groupMenu: 'Groups',
    messageVars: 'Available variables: {{item}}, {{field}}, {{current}}, {{threshold}}, {{severity}}.',
    thresholdHelp: 'Supports setting a JSON field as dynamic threshold, format like "json:data.field1"',
    noEvents: 'No alert events yet.',
    refreshed: 'Refreshed',
    nav: { overview: 'Overview', groups: 'Groups', users: 'Users', settings: 'System', channels: 'Notification Channels', alertRecords: 'Alert Records' },
    labels: {
      monitorGroups: 'Monitor Groups',
      groupSettings: 'Group Settings',
      items: 'Monitor Items',
      activeItems: 'Active Monitor Items',
      groupFields: 'Group Fields',
      itemFields: 'Item Fields',
      groupRules: 'Group Alert Rules',
      itemRules: 'Item Alert Rules',
      latestFields: 'Latest Fields',
      fieldDetail: 'Field Detail',
      trend: 'Trend',
      rules: 'Alert Settings',
      events: 'Alert Events',
      users: 'Users',
      channels: 'Notification Channels',
      settings: 'System Settings',
      dataRetentionDays: 'Data Retention Days',
      defaultLocale: 'Default Locale',
      sessionTimeoutMinutes: 'Session Timeout Minutes',
      uploadToken: 'Upload Token',
      appTimezone: 'System Time Zone',
      channelTemplate: 'Alert Channel',
      hourlyAlerts: 'Alerts in Last 24 Hours',
      normalItems: 'Successful Items',
      totalItems: 'Total Items',
      inheritedFields: 'Custom Fields',
      ownFields: 'Own Fields',
      timeRange: 'Time Range',
    },
    field: {
      continuousAlert: 'Continuous Alert',
      createdDate: 'Created Date',
      searchPlaceholder: 'Search by name',
      statusAll: 'All statuses',
      statusOk: 'Success',
      statusError: 'Alerting',
      action: 'Action',
      code: 'Code',
      name: 'Name',
      icon: 'Icon',
      description: 'Description',
      status: 'Status',
      type: 'Type',
      source: 'Source',
      group: 'Group',
      item: 'Item',
      displayName: 'Display Name',
      role: 'Role',
      lastLogin: 'Last Login',
      enabled: 'Enabled',
      alert: 'Alert',
      intervalSeconds: 'Interval Seconds',
      missedTimes: 'Missed Times',
      lastSeen: 'Last Success',
      url: 'URL',
      method: 'Method',
      bodyType: 'Body Type',
      timeout: 'Timeout Seconds',
      expectedStatus: 'Expected Status',
      headers: 'Header JSON',
      body: 'Body JSON',
      fieldPath: 'JSON Field',
      valueType: 'Value Type',
      unit: 'Unit',
      required: 'Required',
      ruleType: 'Rule',
      operator: 'Operator',
      threshold: 'Threshold',
      aggregate: 'Aggregate',
      window: 'Window Seconds',
      sampleCount: 'Sample Count',
      consecutive: 'Consecutive Count',
      recovery: 'Recovery Count',
      severity: 'Severity',
      messageTemplate: 'Message Template',
      channel: 'Channels',
      channels: 'Channels',
      combineGroup: 'Combine Alert Group',
      webhook: 'Robot Webhook',
      secret: 'Signature Secret (Optional)',
      botToken: 'Bot Token',
      chatID: 'Chat ID',
      value: 'Value',
      time: 'Time',
      latest: 'Latest',
      count: 'Count',
      avg: 'Avg',
      max: 'Max',
      min: 'Min',
      median: 'Median',
      title: 'Title',
      message: 'Message',
      responseSettings: 'Passive Return Settings (JSON)',
      refGroup: 'Target Group',
      refNamePath: 'Item Name JSON Field',
      refItem: 'Parent Item',
      sortOrder: 'Sort Order',
    },
    yes: 'Yes',
    no: 'No',
    roles: { admin: 'Admin', viewer: 'Viewer' },
    sourceTypes: { passive: 'Passive', active: 'Active', any: 'Any' },
    valueTypes: { string: 'String', integer: 'Integer', float: 'Float', boolean: 'Boolean', object_array: 'Object Array', string_array: 'String Array' },
    ruleTypes: { missing_data: 'Missing Data', request_failed: 'Request Failed', field_condition: 'Field Condition', aggregate_condition: 'Aggregate Condition' },
    operators: { gt: '>', gte: '>=', lt: '<', lte: '<=', eq: '=', ne: '!=', contains: 'Contains', not_contains: 'Does Not Contain', exists: 'Field Exists', not_exists: 'Field Missing', len_eq: 'Length =', len_gt: 'Length >', len_lt: 'Length <', len_ne: 'Length !=' },
    severities: { info: 'Info', warning: 'Warning', critical: 'Critical', recovered: 'Recovered' },
    channelTemplates: { dingding: 'DingTalk Robot Alert', feishu: 'Feishu Robot Alert', telegram: 'Telegram Alert' },
  },
}

const t = computed(() => text[locale.value])
const elementLocale = computed(() => locale.value === 'zh-CN' ? zhCn : en)
const isAdmin = computed(() => currentUser.value?.role === 'admin')
const selectedGroupID = computed(() => activeMenu.value.startsWith('group:') ? Number(activeMenu.value.slice(6)) : 0)
const selectedGroup = computed(() => state.groups.find((group) => group.id === selectedGroupID.value))
const selectedGroupItems = computed(() => state.items.filter((item) => item.group_id === selectedGroupID.value))
const filteredAndSortedGroupItems = computed(() => {
  return [...selectedGroupItems.value]
})
const pagedSelectedGroupItems = computed(() => {
  const start = (itemPage.value - 1) * itemPageSize.value
  return filteredAndSortedGroupItems.value.slice(start, start + itemPageSize.value)
})
const selectedGroupFields = computed(() => state.fields.filter((field) => field.scope_type === 'group' && field.group_id === selectedGroupID.value))
const selectedGroupRules = computed(() => state.rules.filter((rule) => rule.scope_type === 'group' && rule.group_id === selectedGroupID.value))
const selectedGroupEvents = computed(() => state.events.filter((event) => event.group_id === selectedGroupID.value))
const hasObjectArrayFields = computed(() => {
  return state.fields.some(field => field.value_type === 'object_array' && field.ref_group_id === selectedGroupID.value)
})
const existingCombineGroups = computed(() => {
  const groups = new Set<string>()
  state.rules.forEach((rule) => {
    if (!rule.combine_group) return
    if (ruleForm.scope_type === 'group') {
      if (rule.scope_type === 'group' && rule.group_id === ruleForm.group_id) {
        groups.add(rule.combine_group)
      }
    } else if (ruleForm.scope_type === 'item' || ruleForm.scope_type === 'field') {
      if ((rule.scope_type === 'item' || rule.scope_type === 'field') && rule.item_id === ruleForm.item_id) {
        groups.add(rule.combine_group)
      }
    }
  })
  return Array.from(groups)
})
const alertPage = ref(1)
const alertPageSize = ref(20)
const totalAlertEvents = ref(0)
const groupEventPage = ref(1)
const groupEventPageSize = ref(20)
const fieldEventPage = ref(1)
const fieldEventPageSize = ref(20)
const fieldStatHours = ref(24)
const fieldTrendChartEl = ref<HTMLElement | null>(null)
let fieldTrendChart: echarts.ECharts | null = null
const overviewChartRefs = new Map<number, HTMLElement>()
const overviewCharts = new Map<number, echarts.ECharts>()
const pagedAlertEvents = computed(() => {
  return state.events
})
const pagedSelectedGroupEvents = computed(() => {
  const start = (groupEventPage.value - 1) * groupEventPageSize.value
  return selectedGroupEvents.value.slice(start, start + groupEventPageSize.value)
})
const selectedGroupNormalItems = computed(() => selectedGroupItems.value.filter((item) => latestSample(item.id)?.status === 'ok').length)
const overviewGroups = computed(() => state.groups.map((group) => ({
  ...group,
  totalItems: totalItemsForGroup(group.id),
  normalItems: normalItemsForGroup(group.id),
})))
const selectedTimeZone = computed(() => state.settings.find((setting) => setting.key === 'app_timezone')?.value || 'Asia/Shanghai')

const localeOptions = [
  { label: '简体中文', value: 'zh-CN' },
  { label: 'English', value: 'en-US' },
]
const timeZoneOptions = [
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Taipei',
  'Asia/Tokyo',
  'Asia/Seoul',
  'Asia/Singapore',
  'Asia/Bangkok',
  'Asia/Dubai',
  'Asia/Kolkata',
  'Australia/Sydney',
  'Pacific/Auckland',
  'UTC',
  'Europe/London',
  'Europe/Paris',
  'Europe/Berlin',
  'Europe/Moscow',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Sao_Paulo',
]
const statHourOptions = [1, 6, 12, 24, 72, 168]
const aggregateOperatorOptions = ['gt', 'gte', 'lt', 'lte', 'eq', 'ne']

const systemMenus = computed(() => [
  { key: 'overview', label: t.value.nav.overview, icon: 'Odometer' },
  { key: 'groups', label: t.value.nav.groups, icon: 'Folder' },
  { key: 'users', label: t.value.nav.users, icon: 'User' },
  { key: 'settings', label: t.value.nav.settings, icon: 'Setting' },
  { key: 'channels', label: t.value.nav.channels, icon: 'Bell' },
  { key: 'alert-records', label: t.value.nav.alertRecords, icon: 'Warning' },
])

function iconFor(name?: string) {
  return (Icons as Record<string, unknown>)[name || 'Monitor'] || Icons.Folder
}

function local(map: Record<string, string>, key?: string) {
  return key ? map[key] || key : ''
}

async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  activeRequestsCount.value++
  try {
    const res = await fetch(path, {
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
      ...options,
    })
    const body = await res.json().catch(() => ({ code: res.status, msg: res.statusText }))
    if (res.status === 401 || body.code === 401) {
      currentUser.value = null
      throw new Error('unauthorized')
    }
    if (!res.ok || body.code !== 0) throw new Error(body.msg || 'request failed')
    return body.data
  } finally {
    activeRequestsCount.value--
  }
}

async function initialize() {
  try {
    await api('/api/install', { method: 'POST', body: JSON.stringify(authForm) })
    installed.value = true
    ElMessage.success(t.value.saved)
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : t.value.loginFailed)
  }
}

async function login() {
  try {
    currentUser.value = await api<User>('/api/auth/login', { method: 'POST', body: JSON.stringify(authForm) })
    await loadInitialData()
    await loadAll()
  } catch {
    ElMessage.error(t.value.loginFailed)
  }
}

async function logout() {
  await api('/api/auth/logout', { method: 'POST' })
  currentUser.value = null
}

async function loadInitialData() {
  const [settings, channels, groups] = await Promise.all([
    api<Setting[]>('/api/settings'),
    api<Channel[]>('/api/channels'),
    api<Group[]>('/api/groups'),
  ])
  state.settings = settings || []
  state.channels = channels || []
  state.groups = groups || []
}

async function loadPageData(menu: string) {
  if (!currentUser.value) return

  if (menu === 'overview') {
    const [dashboard, groups, items, samples, events] = await Promise.all([
      api<typeof state.dashboard>('/api/dashboard'),
      api<Group[]>('/api/groups'),
      api<Item[]>('/api/items'),
      api<Sample[]>('/api/samples?latest=true'),
      api<AlertEvent[]>('/api/events?since_hours=24'),
    ])
    state.dashboard = dashboard || state.dashboard
    state.groups = groups || []
    state.items = items || []
    state.samples = samples || []
    state.events = events || []
    await nextTick()
    renderOverviewCharts()
  } else if (menu === 'groups') {
    state.groups = await api<Group[]>('/api/groups') || []
  } else if (menu === 'users') {
    state.users = await api<User[]>('/api/users') || []
  } else if (menu === 'settings') {
    state.settings = await api<Setting[]>('/api/settings') || []
  } else if (menu === 'channels') {
    state.channels = await api<Channel[]>('/api/channels') || []
  } else if (menu === 'alert-records') {
    if (alertPage.value !== 1) {
      alertPage.value = 1
    } else {
      const offset = (alertPage.value - 1) * alertPageSize.value
      const res = await api<{ events: AlertEvent[], total: number }>(`/api/events?limit=${alertPageSize.value}&offset=${offset}`)
      state.events = res.events || []
      totalAlertEvents.value = res.total || 0
    }
  } else if (menu.startsWith('group:')) {
    const groupID = Number(menu.slice(6))
    const [items, activeRequests, fields, rules, samples, events] = await Promise.all([
      api<Item[]>(`/api/items?group_id=${groupID}&q=${itemNameSearch.value}&status=${itemStatusFilter.value}&sort_prop=${itemSortProp.value}&sort_order=${itemSortOrder.value}`),
      api<ActiveRequest[]>(`/api/active-requests?group_id=${groupID}`),
      api<FieldDefinition[]>(`/api/fields?group_id=${groupID}`),
      api<AlertRule[]>(`/api/rules?group_id=${groupID}`),
      api<Sample[]>(`/api/samples?group_id=${groupID}&latest=true`),
      api<AlertEvent[]>(`/api/events?since_hours=24&group_id=${groupID}`),
    ])
    state.items = items || []
    state.activeRequests = activeRequests || []
    state.fields = fields || []
    state.rules = rules || []
    state.samples = samples || []
    state.events = events || []
  }
}

async function loadAll() {
  await loadPageData(activeMenu.value)
  if (!menuExists(activeMenu.value)) activeMenu.value = 'overview'
  syncHash()
}

async function refreshAll() {
  await loadAll()
  ElMessage.success(t.value.refreshed)
}

function handleMenuSelect(key: string) {
  activeMenu.value = key
  syncHash()
}

function handleItemPageSize() {
  itemPage.value = 1
}

function handleAlertPageSize() {
  alertPage.value = 1
}

function handleGroupEventPageSize() {
  groupEventPage.value = 1
}

function handleFieldEventPageSize() {
  fieldEventPage.value = 1
}

function menuExists(key: string) {
  if (['overview', 'groups', 'users', 'settings', 'channels', 'alert-records'].includes(key)) return true
  if (!key.startsWith('group:')) return false
  const id = Number(key.slice(6))
  return state.groups.some((group) => group.id === id)
}

function menuFromHash() {
  const value = decodeURIComponent(window.location.hash.replace(/^#\/?/, ''))
  return value || 'overview'
}

function syncHash() {
  const next = `#${activeMenu.value}`
  if (window.location.hash !== next) {
    window.history.replaceState(null, '', next)
  }
}

function toggleLocale(value: Locale) {
  locale.value = value
  localStorage.setItem('locale', value)
  document.documentElement.lang = value
}

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  localStorage.setItem('sidebarCollapsed', String(sidebarCollapsed.value))
}

function drawerTitle(type: DrawerType, mode: DrawerMode) {
  const prefix = mode === 'edit' ? t.value.edit : t.value.add
  const names: Record<DrawerType, string> = {
    user: t.value.labels.users,
    group: t.value.labels.monitorGroups,
    channel: t.value.labels.channels,
    active: t.value.labels.activeItems,
    item: t.value.labels.items,
    field: fieldForm.scope_type === 'item' ? t.value.labels.itemFields : t.value.labels.groupFields,
    rule: ruleForm.scope_type === 'item' ? t.value.labels.itemRules : t.value.labels.groupRules,
    '': '',
  }
  return `${prefix} ${names[type]}`
}

function openDrawer(type: DrawerType, mode: DrawerMode, row?: unknown, context: Record<string, unknown> = {}) {
  drawer.type = type
  drawer.mode = mode
  resetForm(type, context)
  if (row) fillForm(type, row)
  drawer.title = drawerTitle(type, mode)
  drawer.visible = true
}

function resetForm(type: DrawerType, context: Record<string, unknown>) {
  if (type === 'user') Object.assign(userForm, { id: 0, username: '', password: '', display_name: '', role: 'viewer', enabled: true })
  if (type === 'group') Object.assign(groupForm, { id: 0, code: '', name: '', icon: 'Monitor', description: '', default_interval_seconds: 60, missed_times_threshold: 3, alert_enabled: true, enabled: true, response_settings_json: '{}', sort_order: 0 })
  if (type === 'active') Object.assign(activeForm, { id: 0, group_id: Number(context.group_id || selectedGroupID.value), name: '', url: '', method: 'GET', headers_json: '{}', body_type: 'none', body_json: '{}', interval_seconds: 60, timeout_seconds: 10, expected_status_code: 200, enabled: true })
  if (type === 'item') Object.assign(itemForm, { id: 0, group_id: Number(context.group_id || selectedGroupID.value), source_type: 'passive', name: '', description: '', interval_seconds: 60, missed_times_threshold: 3, alert_enabled: true, enabled: true, response_settings_json: '{}' })
  if (type === 'field') Object.assign(fieldForm, { id: 0, scope_type: context.scope_type || 'group', group_id: Number(context.group_id || selectedGroupID.value), item_id: context.item_id as number | undefined, field_path: '', display_name: '', value_type: 'float', unit: '', required: false, enabled: true, ref_group_id: undefined, ref_name_path: '' })
  if (type === 'rule') {
    const fields = fieldsForRuleContext(context.scope_type === 'item' ? 'item' : 'group', Number(context.group_id || selectedGroupID.value), context.item_id as number | undefined)
    Object.assign(ruleForm, { id: 0, name: '', scope_type: context.scope_type || 'group', group_id: context.group_id || selectedGroupID.value || undefined, item_id: context.item_id as number | undefined, source_type: 'any', rule_type: 'field_condition', field_path: String(context.field_path || fields[0]?.field_path || ''), value_type: fields[0]?.value_type || 'float', operator: 'gt', threshold_value: '', aggregate_func: 'avg', aggregate_window_seconds: 300, aggregate_sample_count: undefined, consecutive_count: 1, recovery_count: 1, severity: 'warning', message_template: locale.value === 'zh-CN' ? '{{item}} {{field}} 当前值={{current}} 阈值={{threshold}}' : '{{item}} {{field}} current={{current}} threshold={{threshold}}', combine_group: '', enabled: true, channel_ids: [] })
  }
  if (type === 'channel') Object.assign(channelForm, { id: 0, code: '', name: '', channel_type: 'dingding', webhook: '', secret: '', bot_token: '', chat_id: '', enabled: true, is_default: false })
}

function fillForm(type: DrawerType, row: unknown) {
  const data = row as Record<string, unknown>
  if (type === 'user') Object.assign(userForm, data, { password: '' })
  if (type === 'group') Object.assign(groupForm, data)
  if (type === 'active') Object.assign(activeForm, data)
  if (type === 'item') Object.assign(itemForm, data)
  if (type === 'field') Object.assign(fieldForm, data)
  if (type === 'rule') Object.assign(ruleForm, { ...data, channel_ids: (data.channel_ids as number[]) || [] })
  if (type === 'channel') {
    const config = parseJSON(String(data.config_json || '{}'))
    Object.assign(channelForm, data, {
      webhook: String(config.webhook || ''),
      secret: String(config.secret || ''),
      bot_token: String(config.bot_token || ''),
      chat_id: String(config.chat_id || ''),
    })
  }
}

async function submitDrawer() {
  if (drawer.type === 'user') await submitUser()
  if (drawer.type === 'group') await submitGroup()
  if (drawer.type === 'active') await submitActive()
  if (drawer.type === 'item') await submitItem()
  if (drawer.type === 'field') await submitField()
  if (drawer.type === 'rule') await submitRule()
  if (drawer.type === 'channel') await submitChannel()
  drawer.visible = false
  await loadAll()
  ElMessage.success(t.value.saved)
}

async function submitUser() {
  const body = JSON.stringify(userForm)
  if (drawer.mode === 'edit') await api(`/api/users/${userForm.id}`, { method: 'PUT', body })
  else await api('/api/users', { method: 'POST', body })
}

async function submitGroup() {
  try {
    JSON.parse(groupForm.response_settings_json || '{}')
  } catch {
    ElMessage.error(t.value.invalidJSON || 'Invalid JSON format')
    return
  }
  groupForm.alert_enabled = true
  const body = JSON.stringify(groupForm)
  if (drawer.mode === 'edit') await api(`/api/groups/${groupForm.id}`, { method: 'PUT', body })
  else await api('/api/groups', { method: 'POST', body })
}

async function submitActive() {
  const body = JSON.stringify(activeForm)
  if (drawer.mode === 'edit') await api(`/api/active-requests/${activeForm.id}`, { method: 'PUT', body })
  else await api('/api/active-requests', { method: 'POST', body })
}

async function submitItem() {
  try {
    JSON.parse(itemForm.response_settings_json || '{}')
  } catch {
    ElMessage.error(t.value.invalidJSON || 'Invalid JSON format')
    return
  }
  const body = JSON.stringify(itemForm)
  if (drawer.mode === 'edit') await api(`/api/items/${itemForm.id}`, { method: 'PUT', body })
  else await api('/api/items', { method: 'POST', body })
}

async function submitField() {
  await api('/api/fields', { method: 'POST', body: JSON.stringify(fieldForm) })
}

async function submitRule() {
  const payload: Partial<AlertRule> & { channel_ids: number[] } = { ...ruleForm }
  if (isFieldRuleType(payload.rule_type) && (!payload.field_path || !ruleFieldOptions.value.some((field) => field.field_path === payload.field_path))) {
    ElMessage.warning(t.value.emptyFields)
    return
  }
  if (!isFieldRuleType(payload.rule_type)) {
    payload.field_path = ''
    payload.value_type = ''
    payload.operator = ''
    payload.threshold_value = ''
    payload.aggregate_func = ''
    payload.aggregate_window_seconds = undefined
    payload.aggregate_sample_count = undefined
    if (payload.rule_type === 'request_failed') {
      payload.source_type = 'active'
    }
  }
  await api('/api/rules', { method: 'POST', body: JSON.stringify(payload) })
}

async function submitChannel() {
  const config = channelForm.channel_type === 'telegram'
    ? { bot_token: channelForm.bot_token, chat_id: channelForm.chat_id }
    : { webhook: channelForm.webhook, secret: channelForm.secret }
  await api('/api/channels', {
    method: 'POST',
    body: JSON.stringify({
      id: channelForm.id,
      code: channelForm.code,
      name: channelForm.name,
      channel_type: channelForm.channel_type,
      config_json: JSON.stringify(config),
      enabled: channelForm.enabled,
      is_default: channelForm.is_default,
    }),
  })
}

async function saveSettings() {
  state.settings = await api<Setting[]>('/api/settings', { method: 'PUT', body: JSON.stringify(state.settings) })
  ElMessage.success(t.value.saved)
}

async function deleteEntity(type: string, id: number) {
  await ElMessageBox.confirm(t.value.confirmDelete, t.value.delete, { type: 'warning' })
  const endpoints: Record<string, string> = { user: 'users', group: 'groups', item: 'items', active: 'active-requests', field: 'fields', channel: 'channels', rule: 'rules' }
  await api(`/api/${endpoints[type]}/${id}`, { method: 'DELETE' })
  await loadAll()
  ElMessage.success(t.value.deleted)
}

function parseJSON(textValue: string) {
  try {
    return JSON.parse(textValue) as Record<string, unknown>
  } catch {
    return {}
  }
}

function groupItems(groupID: number) {
  return state.items.filter((item) => item.group_id === groupID)
}

function activeForItem(itemID: number) {
  return state.activeRequests.find((request) => request.item_id === itemID)
}

function groupName(groupID?: number) {
  if (!groupID) return '-'
  const group = state.groups.find((candidate) => candidate.id === groupID)
  return group ? `${group.name} / ${group.code}` : String(groupID)
}

function itemName(itemID?: number) {
  if (!itemID) return '-'
  return state.items.find((candidate) => candidate.id === itemID)?.name || String(itemID)
}

function ownItemFields(itemID: number) {
  return state.fields.filter((field) => field.scope_type === 'item' && field.item_id === itemID)
}

function effectiveFieldsForItem(item: Item) {
  const own = ownItemFields(item.id)
  if (own.length > 0) return own
  return state.fields.filter((field) => field.scope_type === 'group' && field.group_id === item.group_id)
}

function itemRules(itemID: number) {
  return state.rules.filter((rule) => rule.scope_type === 'item' && rule.item_id === itemID)
}

function fieldsForRuleContext(scopeType = ruleForm.scope_type, groupID = Number(ruleForm.group_id || selectedGroupID.value), itemID = ruleForm.item_id) {
  if (scopeType === 'item' && itemID) {
    const item = state.items.find((candidate) => candidate.id === itemID)
    return item ? effectiveFieldsForItem(item).filter((field) => field.enabled) : []
  }
  return state.fields.filter((field) => field.scope_type === 'group' && field.group_id === groupID && field.enabled)
}

const ruleFieldOptions = computed(() => fieldsForRuleContext())

const fieldOperatorOptions = computed(() => {
  const fieldPath = ruleForm.field_path
  const field = ruleFieldOptions.value.find((f) => f.field_path === fieldPath)
  if (field && field.value_type === 'string_array') {
    return ['len_eq', 'len_gt', 'len_lt', 'len_ne', 'contains', 'not_contains', 'exists', 'not_exists']
  }
  return ['gt', 'gte', 'lt', 'lte', 'eq', 'ne', 'contains', 'not_contains', 'exists', 'not_exists']
})

function isFieldRuleType(ruleType = ruleForm.rule_type) {
  return ruleType === 'field_condition' || ruleType === 'aggregate_condition'
}

function latestSample(itemID: number) {
  return state.samples.find((sample) => sample.item_id === itemID)
}

function parseTime(value?: string) {
  if (!value) return null
  if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/.test(value)) {
    const utc = new Date(value.replace(' ', 'T') + 'Z')
    if (!Number.isNaN(utc.getTime())) return utc
  }
  const parsed = new Date(value)
  if (!Number.isNaN(parsed.getTime())) return parsed
  const fallback = new Date(value.replace(' ', 'T'))
  return Number.isNaN(fallback.getTime()) ? null : fallback
}

function padNumber(value: number) {
  return String(value).padStart(2, '0')
}

function formatDateTime(value?: string) {
  const date = parseTime(value)
  if (!date) return '-'
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: selectedTimeZone.value,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).formatToParts(date).reduce((acc, part) => {
    acc[part.type] = part.value
    return acc
  }, {} as Record<string, string>)
  const hour = parts.hour === '24' ? '00' : parts.hour
  return `${parts.year}-${parts.month}-${parts.day} ${hour}:${parts.minute}:${parts.second}`
}

function normalItemsForGroup(groupID: number) {
  return state.items.filter((item) => item.group_id === groupID && latestSample(item.id)?.status === 'ok').length
}

function totalItemsForGroup(groupID: number) {
  return state.items.filter((item) => item.group_id === groupID).length
}

function recentHourStarts() {
  const current = new Date()
  current.setMinutes(0, 0, 0)
  return Array.from({ length: 24 }, (_, index) => new Date(current.getTime() - (23 - index) * 60 * 60 * 1000))
}

function hourKey(date: Date) {
  return formatDateTime(date.toISOString()).slice(0, 13)
}

function hourLabel(date: Date) {
  return `${formatDateTime(date.toISOString()).slice(11, 13)}:00`
}

function hourlyAlertSeries(groupID: number) {
  const hours = recentHourStarts()
  const counts = new Map(hours.map((hour) => [hourKey(hour), 0]))
  state.events.forEach((event) => {
    if (event.group_id !== groupID) return
    const occurredAt = parseTime(event.occurred_at)
    if (!occurredAt) return
    occurredAt.setMinutes(0, 0, 0)
    const key = hourKey(occurredAt)
    if (counts.has(key)) counts.set(key, (counts.get(key) || 0) + 1)
  })
  return hours.map((hour) => ({ time: hourLabel(hour), value: counts.get(hourKey(hour)) || 0 }))
}

function hourlyAlertPolyline(groupID: number) {
  const points = hourlyAlertSeries(groupID)
  const values = points.map((point) => point.value)
  const max = Math.max(...values, 1)
  return points.map((point, index) => {
    const x = points.length === 1 ? 300 : (index / (points.length - 1)) * 600
    const y = 150 - (point.value / max) * 120
    return `${x.toFixed(1)},${y.toFixed(1)}`
  }).join(' ')
}

function hourlyAlertPoints(groupID: number) {
  const points = hourlyAlertSeries(groupID)
  const values = points.map((point) => point.value)
  const max = Math.max(...values, 1)
  return points.map((point, index) => ({
    ...point,
    x: Number((points.length === 1 ? 300 : (index / (points.length - 1)) * 600).toFixed(1)),
    y: Number((150 - (point.value / max) * 120).toFixed(1)),
  }))
}

function hourlyAlertTotal(groupID: number) {
  return hourlyAlertSeries(groupID).reduce((sum, point) => sum + point.value, 0)
}

function sampleValue(sample: Sample | undefined, fieldPath: string) {
  return sample?.values?.find((value) => value.field_path === fieldPath)
}

function valueText(value?: SampleValue) {
  if (!value) return '-'
  if (value.value_type === 'string_array' || value.value_type === 'object_array') {
    try {
      const arr = JSON.parse(value.string_value || '[]')
      return Array.isArray(arr) ? String(arr.length) : '0'
    } catch {
      return '0'
    }
  }
  if (value.raw_value !== undefined && value.raw_value !== null) return String(value.raw_value)
  if (value.string_value !== undefined) return value.string_value
  if (value.integer_value !== undefined) return String(value.integer_value)
  if (value.float_value !== undefined) return String(value.float_value)
  if (value.boolean_value !== undefined) return value.boolean_value ? t.value.yes : t.value.no
  return '-'
}

function fieldLabel(field: FieldDefinition) {
  return field.display_name || field.field_path
}

function fieldRules(item: Item | null, field: FieldDefinition | null) {
  if (!field) return []
  return state.rules.filter((rule) => {
    if (rule.field_path !== field.field_path) return false
    if (item) return rule.item_id === item.id || (ownItemFields(item.id).length === 0 && rule.scope_type === 'group' && rule.group_id === item.group_id)
    return rule.scope_type === 'group' && rule.group_id === field.group_id
  })
}

function fieldEvents(item: Item | null, field: FieldDefinition | null) {
  if (!field) return []
  return state.events.filter((event) => event.field_path === field.field_path && (!item || event.item_id === item.id))
}

function pagedFieldEvents(item: Item | null, field: FieldDefinition | null) {
  const start = (fieldEventPage.value - 1) * fieldEventPageSize.value
  return fieldEvents(item, field).slice(start, start + fieldEventPageSize.value)
}

async function openFieldDetail(item: Item, field: FieldDefinition) {
  fieldDetail.item = item
  fieldDetail.field = field
  fieldDetail.visible = true
  fieldEventPage.value = 1
  fieldStatHours.value = 24
  await reloadFieldStats()
}

async function reloadFieldStats() {
  if (!fieldDetail.item || !fieldDetail.field) return
  fieldDetail.stats = await api<StatResult>(`/api/stats?group_id=${fieldDetail.item.group_id}&item_id=${fieldDetail.item.id}&field_path=${encodeURIComponent(fieldDetail.field.field_path)}&hours=${fieldStatHours.value}`)
  await nextTick()
  renderFieldTrendChart()
}

function openFieldRule(field: FieldDefinition, item?: Item) {
  openDrawer('rule', 'create', undefined, {
    scope_type: item ? 'item' : 'group',
    group_id: field.group_id,
    item_id: item?.id,
    field_path: field.field_path,
  })
}

function applyRuleField(fieldPath: string) {
  const field = ruleFieldOptions.value.find((candidate) => candidate.field_path === fieldPath)
  ruleForm.field_path = fieldPath
  if (field) {
    ruleForm.value_type = field.value_type
    const allowed = field.value_type === 'string_array'
      ? ['len_eq', 'len_gt', 'len_lt', 'len_ne', 'contains', 'not_contains', 'exists', 'not_exists']
      : ['gt', 'gte', 'lt', 'lte', 'eq', 'ne', 'contains', 'not_contains', 'exists', 'not_exists']
    if (!allowed.includes(ruleForm.operator)) {
      ruleForm.operator = allowed[0]
    }
  }
}

function handleRuleTypeChange(ruleType: string) {
  ruleForm.rule_type = ruleType
  if (!isFieldRuleType(ruleType)) {
    ruleForm.field_path = ''
    ruleForm.value_type = ''
    if (ruleType === 'request_failed') {
      ruleForm.source_type = 'active'
      ruleForm.message_template = locale.value === 'zh-CN' ? '{{item}} 请求失败：{{current}}' : '{{item}} request failed: {{current}}'
    }
    if (ruleType === 'missing_data') {
      ruleForm.message_template = locale.value === 'zh-CN' ? '{{item}} 数据缺失' : '{{item}} data missing'
    }
    return
  }
  const fields = ruleFieldOptions.value
  if (!fields.some((field) => field.field_path === ruleForm.field_path)) {
    applyRuleField(fields[0]?.field_path || '')
  }
  if (!ruleForm.message_template) {
    ruleForm.message_template = locale.value === 'zh-CN' ? '{{item}} {{field}} 当前值={{current}} 阈值={{threshold}}' : '{{item}} {{field}} current={{current}} threshold={{threshold}}'
  }
}

function settingLabel(setting: Setting) {
  const map: Record<string, string> = {
    data_retention_days: t.value.labels.dataRetentionDays,
    default_locale: t.value.labels.defaultLocale,
    session_timeout_minutes: t.value.labels.sessionTimeoutMinutes,
    upload_token: t.value.labels.uploadToken,
    app_timezone: t.value.labels.appTimezone,
  }
  return map[setting.key] || setting.key
}

function settingDescription(setting: Setting) {
  const map: Record<string, string> = {
    data_retention_days: locale.value === 'zh-CN' ? '原始监控样本和报警日志保留天数。' : 'Days to keep raw monitor samples and alert logs.',
    default_locale: locale.value === 'zh-CN' ? '管理界面的默认语言。' : 'Default UI language.',
    session_timeout_minutes: locale.value === 'zh-CN' ? '管理登录会话的超时时间，单位为分钟。' : 'Management session timeout in minutes.',
    upload_token: locale.value === 'zh-CN' ? '被动上报接口需要携带的全局密钥。' : 'Global token required by passive data receiver.',
    app_timezone: locale.value === 'zh-CN' ? '界面所有时间展示使用的时区。' : 'Time zone used for all displayed timestamps.',
  }
  return map[setting.key] || setting.description
}

function chartPolyline(points?: StatPoint[]) {
  const valid = (points || []).filter((point) => typeof point.value === 'number') as Array<{ time: string; value: number }>
  if (valid.length === 0) return ''
  const values = valid.map((point) => point.value)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1
  return valid.map((point, index) => {
    const x = valid.length === 1 ? 300 : (index / (valid.length - 1)) * 600
    const y = 180 - ((point.value - min) / range) * 150
    return `${x.toFixed(1)},${y.toFixed(1)}`
  }).join(' ')
}

function setOverviewChartRef(groupID: number, el: Element | null | unknown) {
  const existing = overviewCharts.get(groupID)
  if (!el) {
    overviewChartRefs.delete(groupID)
    if (existing) {
      existing.dispose()
      overviewCharts.delete(groupID)
    }
    return
  }
  overviewChartRefs.set(groupID, el as HTMLElement)
  renderOverviewCharts()
}

function renderOverviewCharts() {
  nextTick(() => {
    overviewGroups.value.forEach((group) => {
      const el = overviewChartRefs.get(group.id)
      if (!el) return
      const chart = overviewCharts.get(group.id) || echarts.init(el)
      overviewCharts.set(group.id, chart)
      const series = hourlyAlertSeries(group.id)
      chart.setOption({
        grid: { left: 8, right: 8, top: 12, bottom: 24, containLabel: true },
        tooltip: { trigger: 'axis' },
        xAxis: { type: 'category', data: series.map((point) => point.time), boundaryGap: false },
        yAxis: { type: 'value', minInterval: 1 },
        series: [{ type: 'line', smooth: true, symbol: 'circle', symbolSize: 7, data: series.map((point) => point.value), areaStyle: { opacity: 0.08 } }],
      }, true)
      chart.resize()
    })
  })
}

function renderFieldTrendChart() {
  if (!fieldTrendChartEl.value) return
  const chart = fieldTrendChart || echarts.init(fieldTrendChartEl.value)
  fieldTrendChart = chart
  const series = fieldDetail.stats?.series || []
  chart.setOption({
    grid: { left: 12, right: 16, top: 18, bottom: 28, containLabel: true },
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: series.map((point) => formatDateTime(point.time).slice(5, 16)), boundaryGap: false },
    yAxis: { type: 'value' },
    series: [{ type: 'line', smooth: true, symbol: 'circle', symbolSize: 7, data: series.map((point) => point.value ?? null), areaStyle: { opacity: 0.08 } }],
  }, true)
  chart.resize()
}

function ruleOperator(rule: AlertRule) {
  return local(t.value.operators, rule.operator)
}

function operatorNeedsThreshold(operator: string) {
  return operator !== 'exists' && operator !== 'not_exists'
}

function ruleChannels(rule: AlertRule) {
  const names = (rule.channel_ids || [])
    .map((id) => state.channels.find((channel) => channel.id === id)?.name)
    .filter(Boolean)
  return names.length ? names.join(', ') : '-'
}

function itemFieldMode(itemID: number) {
  return ownItemFields(itemID).length ? t.value.labels.ownFields : t.value.labels.inheritedFields
}

async function handleItemSortChange({ prop, order }: { prop: string | null, order: string | null }) {
  itemSortProp.value = prop || ''
  itemSortOrder.value = order || ''
  await fetchGroupItems()
}

async function fetchGroupItems() {
  await loadPageData(activeMenu.value)
}

onMounted(async () => {
  document.documentElement.lang = locale.value
  window.addEventListener('resize', resizeCharts)
  activeMenu.value = menuFromHash()
  window.addEventListener('hashchange', () => {
    activeMenu.value = menuFromHash()
  })
  const status = await api<{ installed: boolean }>('/api/install/status')
  installed.value = status.installed
  if (installed.value) {
    try {
      currentUser.value = await api<User>('/api/auth/me')
      await loadInitialData()
      await loadAll()
    } catch {
      currentUser.value = null
    }
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeCharts)
  overviewCharts.forEach((chart) => chart.dispose())
  overviewCharts.clear()
  fieldTrendChart?.dispose()
  fieldTrendChart = null
})

function resizeCharts() {
  overviewCharts.forEach((chart) => chart.resize())
  fieldTrendChart?.resize()
}

watch(() => ruleForm.scope_type, () => {
  const fields = ruleFieldOptions.value
  if (isFieldRuleType() && !fields.some((field) => field.field_path === ruleForm.field_path)) {
    applyRuleField(fields[0]?.field_path || '')
  }
})

watch(selectedGroupID, () => {
  itemPage.value = 1
  groupEventPage.value = 1
  groupActiveTab.value = 'items'
  itemNameSearch.value = ''
  itemStatusFilter.value = ''
  itemSortProp.value = ''
  itemSortOrder.value = ''
})

watch([itemNameSearch, itemStatusFilter], () => {
  itemPage.value = 1
})

watch([selectedTimeZone, locale], () => {
  renderOverviewCharts()
  renderFieldTrendChart()
})

watch(() => fieldDetail.visible, (visible) => {
  if (visible) nextTick(() => renderFieldTrendChart())
})

watch(activeMenu, async () => {
  if (currentUser.value) {
    try {
      await loadAll()
    } catch (err) {
      console.error('auto refresh failed:', err)
    }
  }
})

watch([alertPage, alertPageSize], async () => {
  if (activeMenu.value === 'alert-records') {
    try {
      const offset = (alertPage.value - 1) * alertPageSize.value
      const res = await api<{ events: AlertEvent[], total: number }>(`/api/events?limit=${alertPageSize.value}&offset=${offset}`)
      state.events = res.events || []
      totalAlertEvents.value = res.total || 0
    } catch (err) {
      console.error('failed to load paginated events:', err)
    }
  }
})
</script>

<template>
  <el-config-provider :locale="elementLocale">
  <main v-if="!installed || !currentUser" class="auth-shell" v-loading="loading">
    <el-card class="auth-card" shadow="never">
      <div class="auth-head">
        <h1>lightMonitor</h1>
        <el-select class="auth-language" :model-value="locale" size="small" @update:model-value="toggleLocale">
          <el-option v-for="option in localeOptions" :key="option.value" :label="option.label" :value="option.value" />
        </el-select>
      </div>
      <p v-if="!installed">{{ t.install }}</p>
      <el-form label-position="top" @submit.prevent="installed ? login() : initialize()">
        <el-form-item :label="t.username">
          <el-input v-model="authForm.username" autocomplete="username" />
        </el-form-item>
        <el-form-item :label="t.password">
          <el-input v-model="authForm.password" type="password" autocomplete="current-password" show-password />
        </el-form-item>
        <el-button type="primary" native-type="submit" class="full-button">{{ t.submit }}</el-button>
      </el-form>
    </el-card>
  </main>

  <el-container v-else class="app-shell" :class="{ collapsed: sidebarCollapsed }">
    <el-aside class="sidebar">
      <div class="brand">
        <div class="brand-main">
          <span class="brand-mark">LM</span>
          <span class="brand-name">lightMonitor</span>
        </div>
        <el-button text :title="sidebarCollapsed ? t.expand : t.collapse" @click="toggleSidebar">
          <el-icon><component :is="sidebarCollapsed ? Icons.Expand : Icons.Fold" /></el-icon>
        </el-button>
      </div>
      <el-menu :collapse="sidebarCollapsed" :default-active="activeMenu" @select="handleMenuSelect">
        <el-menu-item v-for="menu in systemMenus" :key="menu.key" :index="menu.key">
          <el-icon><component :is="iconFor(menu.icon)" /></el-icon>
          <template #title>{{ menu.label }}</template>
        </el-menu-item>
        <el-menu-item-group v-if="state.groups.length" :title="t.groupMenu">
          <el-menu-item v-for="group in state.groups" :key="group.id" :index="`group:${group.id}`">
            <el-icon><component :is="iconFor(group.icon)" /></el-icon>
            <template #title>{{ group.name || group.code }}</template>
          </el-menu-item>
        </el-menu-item-group>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="topbar">
        <div class="top-actions">
          <el-tag>{{ currentUser.username }} / {{ local(t.roles, currentUser.role) }}</el-tag>
          <label class="locale-inline">
            <el-select :model-value="locale" size="small" @update:model-value="toggleLocale">
              <el-option v-for="option in localeOptions" :key="option.value" :label="option.label" :value="option.value" />
            </el-select>
          </label>
          <el-button circle :title="t.refresh" @click="refreshAll"><el-icon><component :is="Icons.Refresh" /></el-icon></el-button>
          <el-button circle :title="t.logout" @click="logout"><el-icon><component :is="Icons.SwitchButton" /></el-icon></el-button>
        </div>
      </el-header>

      <el-main class="content" v-loading="loading">
        <section v-if="activeMenu === 'overview'" class="page">
          <el-card shadow="never">
            <template #header>
              <div class="card-header">
                <span>{{ t.nav.overview }}</span>
              </div>
            </template>
            <div class="overview-grid">
              <button
                v-for="group in overviewGroups"
                :key="group.id"
                :class="['overview-card', group.normalItems === group.totalItems ? 'is-success' : 'is-warning']"
                type="button"
                @click="handleMenuSelect(`group:${group.id}`)"
              >
                <span class="overview-card-title">
                  <el-icon><component :is="iconFor(group.icon)" /></el-icon>
                  <strong>{{ group.name || group.code }}</strong>
                  <el-tag size="small">{{ group.code }}</el-tag>
                </span>
                <span class="overview-card-count">
                  <strong>{{ group.normalItems }}</strong>
                  <span>/ {{ group.totalItems }}</span>
                </span>
                <span class="overview-card-label">{{ t.labels.normalItems }} / {{ t.labels.totalItems }}</span>
              </button>
            </div>
          </el-card>

          <el-card shadow="never">
            <template #header>
              <div class="card-header">
                <span>{{ t.labels.hourlyAlerts }}</span>
              </div>
            </template>
            <div class="overview-chart-grid">
              <div v-for="group in overviewGroups" :key="group.id" class="overview-chart-card">
                <div class="overview-chart-head">
                  <span>
                    <el-icon><component :is="iconFor(group.icon)" /></el-icon>
                    {{ group.name || group.code }}
                  </span>
                  <strong>{{ hourlyAlertTotal(group.id) }}</strong>
                </div>
                <div :ref="(el) => setOverviewChartRef(group.id, el)" class="mini-chart"></div>
                <div class="chart-axis">
                  <span>{{ hourlyAlertSeries(group.id)[0]?.time }}</span>
                  <span>{{ hourlyAlertSeries(group.id)[23]?.time }}</span>
                </div>
              </div>
            </div>
          </el-card>
        </section>

        <section v-else-if="activeMenu === 'groups'" class="page">
          <el-card shadow="never">
            <template #header>
              <div class="card-header">
                <span>{{ t.labels.monitorGroups }}</span>
                <el-button v-if="isAdmin" type="primary" :icon="Icons.Plus" @click="openDrawer('group', 'create')">{{ t.add }}</el-button>
              </div>
            </template>
            <el-table :data="state.groups">
              <el-table-column :label="t.field.icon" width="70"><template #default="{ row }"><el-icon><component :is="iconFor(row.icon)" /></el-icon></template></el-table-column>
              <el-table-column prop="code" :label="t.field.code" />
              <el-table-column prop="name" :label="t.field.name" />
              <el-table-column prop="default_interval_seconds" :label="t.field.intervalSeconds" />
              <el-table-column prop="missed_times_threshold" :label="t.field.missedTimes" />
              <el-table-column :label="t.field.enabled"><template #default="{ row }">{{ row.enabled ? t.yes : t.no }}</template></el-table-column>
              <el-table-column v-if="isAdmin" :label="t.field.action" width="150">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openDrawer('group', 'edit', row)">{{ t.edit }}</el-button>
                  <el-button link type="danger" @click="deleteEntity('group', row.id)">{{ t.delete }}</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </section>

        <section v-else-if="activeMenu === 'users'" class="page">
          <el-card shadow="never">
            <template #header>
              <div class="card-header">
                <span>{{ t.labels.users }}</span>
                <el-button v-if="isAdmin" type="primary" :icon="Icons.Plus" @click="openDrawer('user', 'create')">{{ t.add }}</el-button>
              </div>
            </template>
            <el-table :data="state.users">
              <el-table-column prop="username" :label="t.username" />
              <el-table-column prop="display_name" :label="t.field.displayName" />
              <el-table-column :label="t.field.role"><template #default="{ row }">{{ local(t.roles, row.role) }}</template></el-table-column>
              <el-table-column :label="t.field.enabled"><template #default="{ row }">{{ row.enabled ? t.yes : t.no }}</template></el-table-column>
                  <el-table-column :label="t.field.lastLogin"><template #default="{ row }">{{ formatDateTime(row.last_login_at) }}</template></el-table-column>
              <el-table-column v-if="isAdmin" :label="t.field.action" width="150">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openDrawer('user', 'edit', row)">{{ t.edit }}</el-button>
                  <el-button link type="danger" @click="deleteEntity('user', row.id)">{{ t.delete }}</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </section>

        <section v-else-if="activeMenu === 'settings'" class="page">
          <el-card shadow="never">
            <template #header>
              <div class="card-header">
                <span>{{ t.labels.settings }}</span>
              </div>
            </template>
            <el-table :data="state.settings">
              <el-table-column :label="t.field.name"><template #default="{ row }">{{ settingLabel(row) }}</template></el-table-column>
              <el-table-column :label="t.field.value">
                <template #default="{ row }">
                  <el-select v-if="row.key === 'default_locale'" v-model="row.value" :disabled="!isAdmin">
                    <el-option v-for="option in localeOptions" :key="option.value" :label="option.label" :value="option.value" />
                  </el-select>
                  <el-select v-else-if="row.key === 'app_timezone'" v-model="row.value" :disabled="!isAdmin">
                    <el-option v-for="option in timeZoneOptions" :key="option" :label="option" :value="option" />
                  </el-select>
                  <el-input v-else v-model="row.value" :disabled="!isAdmin" />
                </template>
              </el-table-column>
              <el-table-column :label="t.field.description"><template #default="{ row }">{{ settingDescription(row) }}</template></el-table-column>
            </el-table>
            <div class="table-actions">
              <el-button v-if="isAdmin" type="primary" :icon="Icons.Check" @click="saveSettings">{{ t.save }}</el-button>
            </div>
          </el-card>
        </section>

        <section v-else-if="activeMenu === 'channels'" class="page">
          <el-card shadow="never">
            <template #header>
              <div class="card-header">
                <span>{{ t.labels.channels }}</span>
                <el-button v-if="isAdmin" type="primary" :icon="Icons.Plus" @click="openDrawer('channel', 'create')">{{ t.add }}</el-button>
              </div>
            </template>
            <el-table :data="state.channels">
              <el-table-column prop="code" :label="t.field.code" />
              <el-table-column prop="name" :label="t.field.name" />
              <el-table-column :label="t.labels.channelTemplate"><template #default="{ row }">{{ local(t.channelTemplates, row.channel_type) }}</template></el-table-column>
              <el-table-column :label="t.field.enabled"><template #default="{ row }">{{ row.enabled ? t.yes : t.no }}</template></el-table-column>
              <el-table-column v-if="isAdmin" :label="t.field.action" width="150">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openDrawer('channel', 'edit', row)">{{ t.edit }}</el-button>
                  <el-button link type="danger" @click="deleteEntity('channel', row.id)">{{ t.delete }}</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </section>

        <section v-else-if="activeMenu === 'alert-records'" class="page">
          <el-card shadow="never">
            <template #header>
              <div class="card-header">
                <span>{{ t.nav.alertRecords }}</span>
              </div>
            </template>
            <el-table :data="pagedAlertEvents" :empty-text="t.noEvents">
              <el-table-column :label="t.field.time" min-width="170"><template #default="{ row }">{{ formatDateTime(row.occurred_at) }}</template></el-table-column>
              <el-table-column :label="t.field.group" min-width="150"><template #default="{ row }">{{ groupName(row.group_id) }}</template></el-table-column>
              <el-table-column :label="t.field.item" min-width="130"><template #default="{ row }">{{ itemName(row.item_id) }}</template></el-table-column>
              <el-table-column :label="t.field.severity" width="110"><template #default="{ row }">{{ local(t.severities, row.severity) }}</template></el-table-column>
              <el-table-column prop="field_path" :label="t.field.fieldPath" min-width="130" />
              <el-table-column prop="title" :label="t.field.title" min-width="180" />
              <el-table-column prop="message" :label="t.field.message" min-width="280" />
            </el-table>
             <el-pagination
              v-model:current-page="alertPage"
              v-model:page-size="alertPageSize"
              class="table-pagination"
              layout="total, sizes, prev, pager, next"
              :page-sizes="[20, 50, 100, 200]"
              :total="totalAlertEvents"
              @size-change="handleAlertPageSize"
            />
          </el-card>
        </section>

        <section v-else-if="selectedGroup" class="page">
          <div class="group-title">
            <div>
              <h2>
                <el-icon><component :is="iconFor(selectedGroup.icon)" /></el-icon>
                {{ selectedGroup.name }}
                <el-tag size="small">{{ selectedGroup.code }}</el-tag>
              </h2>
            </div>
            <div class="overview-metrics">
              <div class="metric">
                <span>{{ t.labels.normalItems }}</span>
                <strong>{{ selectedGroupNormalItems }}</strong>
              </div>
              <div class="metric">
                <span>{{ t.labels.totalItems }}</span>
                <strong>{{ selectedGroupItems.length }}</strong>
              </div>
            </div>
          </div>

          <el-card shadow="never">
            <el-tabs v-model="groupActiveTab">
              <el-tab-pane :label="t.labels.items" name="items">
                <div class="tab-toolbar">
                  <div class="toolbar-left">
                    <el-input
                      v-model="itemNameSearch"
                      :placeholder="t.field.searchPlaceholder"
                      :prefix-icon="Icons.Search"
                      clearable
                      class="search-input"
                      @keyup.enter="fetchGroupItems"
                    />
                    <el-select
                      v-model="itemStatusFilter"
                      :placeholder="t.field.status"
                      clearable
                      class="status-select"
                    >
                      <el-option :label="t.field.statusAll" value="" />
                      <el-option :label="t.field.statusOk" value="ok" />
                      <el-option :label="t.field.statusError" value="error" />
                    </el-select>
                    <el-button type="primary" :icon="Icons.Search" @click="fetchGroupItems">{{ t.search }}</el-button>
                  </div>
                  <el-button v-if="isAdmin" type="primary" :icon="Icons.Plus" @click="openDrawer('active', 'create', undefined, { group_id: selectedGroup.id })">{{ t.add }}</el-button>
                </div>
                <el-table :data="pagedSelectedGroupItems" @sort-change="handleItemSortChange">
                  <el-table-column type="expand" width="42">
                    <template #default="{ row }">
                      <div class="item-detail">
                        <el-alert v-if="ownItemFields(row.id).length" type="warning" :closable="false" :title="t.overridden" />
                        <el-alert v-else type="info" :closable="false" :title="t.inherited" />
                        <div class="nested-header">
                          <strong>{{ t.labels.itemFields }}</strong>
                          <el-button v-if="isAdmin" size="small" type="primary" :icon="Icons.Plus" @click="openDrawer('field', 'create', undefined, { scope_type: 'item', group_id: row.group_id, item_id: row.id })">{{ t.add }}</el-button>
                        </div>
                        <el-table :data="ownItemFields(row.id)" size="small" empty-text="-">
                          <el-table-column prop="display_name" :label="t.field.displayName" />
                          <el-table-column prop="field_path" :label="t.field.fieldPath" />
                          <el-table-column :label="t.field.valueType"><template #default="{ row: field }">{{ local(t.valueTypes, field.value_type) }}</template></el-table-column>
                          <el-table-column prop="unit" :label="t.field.unit" />
                          <el-table-column v-if="isAdmin" :label="t.field.action" width="220">
                            <template #default="{ row: field }">
                              <el-button link type="primary" @click="openDrawer('field', 'edit', field)">{{ t.edit }}</el-button>
                              <el-button link type="primary" @click="openFieldRule(field, row)">{{ t.labels.rules }}</el-button>
                              <el-button link type="danger" @click="deleteEntity('field', field.id)">{{ t.delete }}</el-button>
                            </template>
                          </el-table-column>
                        </el-table>
                        <div class="nested-header">
                          <strong>{{ t.labels.itemRules }}</strong>
                          <el-button v-if="isAdmin" size="small" type="primary" :icon="Icons.Plus" @click="openDrawer('rule', 'create', undefined, { scope_type: 'item', group_id: row.group_id, item_id: row.id })">{{ t.add }}</el-button>
                        </div>
                        <el-table :data="itemRules(row.id)" size="small" empty-text="-">
                          <el-table-column prop="name" :label="t.field.name">
                            <template #default="{ row: rule }">
                              <span :class="{ 'disabled-item-name': !rule.enabled }">{{ rule.name }}</span>
                            </template>
                          </el-table-column>
                          <el-table-column prop="field_path" :label="t.field.fieldPath" min-width="120" show-overflow-tooltip class-name="nowrap-column" label-class-name="nowrap-column" />
                          <el-table-column :label="t.field.ruleType"><template #default="{ row: rule }">{{ local(t.ruleTypes, rule.rule_type) }}</template></el-table-column>
                          <el-table-column :label="t.field.operator"><template #default="{ row: rule }">{{ ruleOperator(rule) }}</template></el-table-column>
                          <el-table-column prop="threshold_value" :label="t.field.threshold" />
                          <el-table-column prop="consecutive_count" :label="t.field.consecutive" />
                          <el-table-column prop="recovery_count" :label="t.field.recovery" />
                          <el-table-column :label="t.field.severity"><template #default="{ row: rule }">{{ local(t.severities, rule.severity) }}</template></el-table-column>
                          <el-table-column prop="combine_group" :label="t.field.combineGroup" min-width="150" show-overflow-tooltip class-name="nowrap-column" label-class-name="nowrap-column" />
                          <el-table-column :label="t.field.channels"><template #default="{ row: rule }">{{ ruleChannels(rule) }}</template></el-table-column>
                          <el-table-column v-if="isAdmin" :label="t.field.action" width="110">
                            <template #default="{ row: rule }">
                              <el-button link type="primary" @click="openDrawer('rule', 'edit', rule)">{{ t.edit }}</el-button>
                              <el-button link type="danger" @click="deleteEntity('rule', rule.id)">{{ t.delete }}</el-button>
                            </template>
                          </el-table-column>
                        </el-table>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column prop="name" :label="t.field.name" min-width="130" sortable="custom">
                    <template #default="{ row }">
                      <span :class="{ 'disabled-item-name': !row.enabled }">{{ row.name }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column v-if="hasObjectArrayFields" prop="ref_item_name" :label="t.field.refItem" min-width="110" />
                  <el-table-column :label="t.field.source" width="75"><template #default="{ row }">{{ local(t.sourceTypes, row.source_type) }}</template></el-table-column>
                  <el-table-column prop="status" :label="t.field.status" width="100" sortable="custom">
                    <template #default="{ row }"><el-tag :type="latestSample(row.id)?.status === 'ok' ? 'success' : 'warning'">{{ latestSample(row.id)?.status || '-' }}</el-tag></template>
                  </el-table-column>
                  <el-table-column :label="t.labels.latestFields" min-width="150">
                    <template #default="{ row }">
                      <div class="field-vertical-list">
                        <div v-if="ownItemFields(row.id).length" class="field-item-row" style="margin-bottom: 4px;">
                          <el-tag type="warning" effect="dark" size="small">{{ t.labels.inheritedFields }}</el-tag>
                        </div>
                        <template v-if="effectiveFieldsForItem(row).length">
                          <div
                            v-for="field in effectiveFieldsForItem(row)"
                            :key="`${row.id}-${field.id}`"
                            class="field-item-row"
                          >
                            <el-button
                              link
                              type="primary"
                              @click="openFieldDetail(row, field)"
                            >
                              {{ fieldLabel(field) }}: {{ valueText(sampleValue(latestSample(row.id), field.field_path)) }}{{ field.unit }}
                            </el-button>
                          </div>
                        </template>
                        <span v-else-if="!ownItemFields(row.id).length" class="muted">{{ t.emptyFields }}</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column prop="created_at" :label="t.field.createdDate" min-width="145" sortable="custom">
                    <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
                  </el-table-column>
                  <el-table-column prop="last_seen_at" :label="t.field.lastSeen" min-width="145" sortable="custom">
                    <template #default="{ row }">{{ formatDateTime(row.last_seen_at) }}</template>
                  </el-table-column>
                  <el-table-column v-if="isAdmin" :label="t.field.action" width="130">
                    <template #default="{ row }">
                      <el-button link type="primary" @click="activeForItem(row.id) ? openDrawer('active', 'edit', activeForItem(row.id)) : openDrawer('item', 'edit', row)">{{ t.edit }}</el-button>
                      <el-button link type="danger" @click="deleteEntity(row.source_type === 'active' && activeForItem(row.id) ? 'active' : 'item', activeForItem(row.id)?.id || row.id)">{{ t.delete }}</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <el-pagination
                  v-model:current-page="itemPage"
                  v-model:page-size="itemPageSize"
                  class="table-pagination"
                  layout="total, sizes, prev, pager, next"
                  :page-sizes="[20, 50, 100, 200]"
                  :total="filteredAndSortedGroupItems.length"
                  @size-change="handleItemPageSize"
                />
              </el-tab-pane>

              <el-tab-pane :label="t.labels.groupFields" name="fields">
                <div class="tab-toolbar">
                  <span></span>
                  <el-button v-if="isAdmin" type="primary" :icon="Icons.Plus" @click="openDrawer('field', 'create', undefined, { scope_type: 'group', group_id: selectedGroup.id })">{{ t.add }}</el-button>
                </div>
                <el-table :data="selectedGroupFields">
                  <el-table-column prop="display_name" :label="t.field.displayName" />
                  <el-table-column prop="field_path" :label="t.field.fieldPath" />
                  <el-table-column :label="t.field.valueType"><template #default="{ row }">{{ local(t.valueTypes, row.value_type) }}</template></el-table-column>
                  <el-table-column prop="unit" :label="t.field.unit" />
                  <el-table-column :label="t.field.required"><template #default="{ row }">{{ row.required ? t.yes : t.no }}</template></el-table-column>
                  <el-table-column :label="t.field.enabled"><template #default="{ row }">{{ row.enabled ? t.yes : t.no }}</template></el-table-column>
                  <el-table-column v-if="isAdmin" :label="t.field.action" width="220">
                    <template #default="{ row }">
                      <el-button link type="primary" @click="openDrawer('field', 'edit', row)">{{ t.edit }}</el-button>
                      <el-button link type="primary" @click="openFieldRule(row)">{{ t.labels.rules }}</el-button>
                      <el-button link type="danger" @click="deleteEntity('field', row.id)">{{ t.delete }}</el-button>
                    </template>
                  </el-table-column>
                </el-table>
              </el-tab-pane>

              <el-tab-pane :label="t.labels.groupRules" name="rules">
                <div class="tab-toolbar">
                  <span></span>
                  <el-button v-if="isAdmin" type="primary" :icon="Icons.Plus" @click="openDrawer('rule', 'create', undefined, { scope_type: 'group', group_id: selectedGroup.id })">{{ t.add }}</el-button>
                </div>
                 <el-table :data="selectedGroupRules">
                  <el-table-column prop="name" :label="t.field.name">
                    <template #default="{ row }">
                      <span :class="{ 'disabled-item-name': !row.enabled }">{{ row.name }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column prop="field_path" :label="t.field.fieldPath" min-width="120" show-overflow-tooltip class-name="nowrap-column" label-class-name="nowrap-column" />
                  <el-table-column :label="t.field.ruleType"><template #default="{ row }">{{ local(t.ruleTypes, row.rule_type) }}</template></el-table-column>
                  <el-table-column :label="t.field.operator"><template #default="{ row }">{{ ruleOperator(row) }}</template></el-table-column>
                  <el-table-column prop="threshold_value" :label="t.field.threshold" />
                  <el-table-column prop="consecutive_count" :label="t.field.consecutive" />
                  <el-table-column prop="recovery_count" :label="t.field.recovery" />
                  <el-table-column :label="t.field.severity"><template #default="{ row }">{{ local(t.severities, row.severity) }}</template></el-table-column>
                  <el-table-column prop="combine_group" :label="t.field.combineGroup" min-width="150" show-overflow-tooltip class-name="nowrap-column" label-class-name="nowrap-column" />
                  <el-table-column :label="t.field.channels"><template #default="{ row }">{{ ruleChannels(row) }}</template></el-table-column>
                  <el-table-column v-if="isAdmin" :label="t.field.action" width="110">
                    <template #default="{ row }">
                      <el-button link type="primary" @click="openDrawer('rule', 'edit', row)">{{ t.edit }}</el-button>
                      <el-button link type="danger" @click="deleteEntity('rule', row.id)">{{ t.delete }}</el-button>
                    </template>
                  </el-table-column>
                </el-table>
              </el-tab-pane>

              <el-tab-pane :label="t.labels.events" name="events">
                <el-table :data="pagedSelectedGroupEvents" :empty-text="t.noEvents">
                  <el-table-column :label="t.field.time" min-width="170"><template #default="{ row }">{{ formatDateTime(row.occurred_at) }}</template></el-table-column>
                  <el-table-column :label="t.field.item" min-width="130"><template #default="{ row }">{{ itemName(row.item_id) }}</template></el-table-column>
                  <el-table-column :label="t.field.severity" width="110"><template #default="{ row }">{{ local(t.severities, row.severity) }}</template></el-table-column>
                  <el-table-column prop="field_path" :label="t.field.fieldPath" min-width="130" />
                  <el-table-column prop="title" :label="t.field.title" min-width="160" />
                  <el-table-column prop="message" :label="t.field.message" min-width="260" />
                </el-table>
                <el-pagination
                  v-model:current-page="groupEventPage"
                  v-model:page-size="groupEventPageSize"
                  class="table-pagination"
                  layout="total, sizes, prev, pager, next"
                  :page-sizes="[20, 50, 100, 200]"
                  :total="selectedGroupEvents.length"
                  @size-change="handleGroupEventPageSize"
                />
              </el-tab-pane>
            </el-tabs>
          </el-card>
        </section>

        <el-empty v-else :description="t.noGroup" />
      </el-main>
    </el-container>

    <el-drawer v-model="drawer.visible" :title="drawer.title" size="480px">
      <div v-loading="loading" class="drawer-body-content">
        <el-form label-position="top" class="drawer-form">
        <template v-if="drawer.type === 'user'">
          <el-form-item :label="t.username"><el-input v-model="userForm.username" /></el-form-item>
          <el-form-item :label="t.password"><el-input v-model="userForm.password" type="password" show-password /></el-form-item>
          <el-form-item :label="t.field.displayName"><el-input v-model="userForm.display_name" /></el-form-item>
          <el-form-item :label="t.field.role"><el-select v-model="userForm.role"><el-option :label="t.roles.admin" value="admin" /><el-option :label="t.roles.viewer" value="viewer" /></el-select></el-form-item>
          <el-form-item :label="t.field.enabled"><el-switch v-model="userForm.enabled" /></el-form-item>
        </template>

        <template v-if="drawer.type === 'group'">
          <el-form-item :label="t.field.code"><el-input v-model="groupForm.code" /></el-form-item>
          <el-form-item :label="t.field.name"><el-input v-model="groupForm.name" /></el-form-item>
          <el-form-item :label="t.field.icon">
            <el-select v-model="groupForm.icon">
              <el-option v-for="name in iconOptions" :key="name" :label="name" :value="name">
                <el-icon><component :is="iconFor(name)" /></el-icon><span class="icon-option">{{ name }}</span>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item :label="t.field.description"><el-input v-model="groupForm.description" type="textarea" /></el-form-item>
          <el-form-item :label="t.field.intervalSeconds"><el-input-number v-model="groupForm.default_interval_seconds" :min="1" /></el-form-item>
          <el-form-item :label="t.field.missedTimes"><el-input-number v-model="groupForm.missed_times_threshold" :min="1" /></el-form-item>
          <el-form-item :label="t.field.responseSettings"><el-input v-model="groupForm.response_settings_json" type="textarea" /></el-form-item>
          <el-form-item :label="t.field.sortOrder"><el-input-number v-model="groupForm.sort_order" :min="0" /></el-form-item>
          <el-form-item :label="t.field.enabled"><el-switch v-model="groupForm.enabled" /></el-form-item>
        </template>

        <template v-if="drawer.type === 'active'">
          <el-form-item :label="t.field.name"><el-input v-model="activeForm.name" /></el-form-item>
          <el-form-item :label="t.field.url"><el-input v-model="activeForm.url" /></el-form-item>
          <el-form-item :label="t.field.method"><el-select v-model="activeForm.method"><el-option label="GET" value="GET" /><el-option label="POST" value="POST" /></el-select></el-form-item>
          <el-form-item :label="t.field.bodyType"><el-select v-model="activeForm.body_type"><el-option label="none" value="none" /><el-option label="json" value="json" /><el-option label="form-data" value="form-data" /></el-select></el-form-item>
          <el-form-item :label="t.field.intervalSeconds"><el-input-number v-model="activeForm.interval_seconds" :min="1" /></el-form-item>
          <el-form-item :label="t.field.timeout"><el-input-number v-model="activeForm.timeout_seconds" :min="1" /></el-form-item>
          <el-form-item :label="t.field.expectedStatus"><el-input-number v-model="activeForm.expected_status_code" :min="100" /></el-form-item>
          <el-form-item :label="t.field.headers"><el-input v-model="activeForm.headers_json" type="textarea" /></el-form-item>
          <el-form-item :label="t.field.body"><el-input v-model="activeForm.body_json" type="textarea" /></el-form-item>
          <el-form-item :label="t.field.enabled"><el-switch v-model="activeForm.enabled" /></el-form-item>
        </template>

        <template v-if="drawer.type === 'item'">
          <el-form-item :label="t.field.name"><el-input v-model="itemForm.name" /></el-form-item>
          <el-form-item :label="t.field.description"><el-input v-model="itemForm.description" type="textarea" /></el-form-item>
          <el-form-item v-if="drawer.mode !== 'edit'" :label="t.field.source"><el-select v-model="itemForm.source_type"><el-option :label="t.sourceTypes.passive" value="passive" /><el-option :label="t.sourceTypes.active" value="active" /></el-select></el-form-item>
          <el-form-item :label="t.field.intervalSeconds"><el-input-number v-model="itemForm.interval_seconds" :min="1" /></el-form-item>
          <el-form-item :label="t.field.missedTimes"><el-input-number v-model="itemForm.missed_times_threshold" :min="1" /></el-form-item>
          <el-form-item :label="t.field.alert"><el-switch v-model="itemForm.alert_enabled" /></el-form-item>
          <el-form-item v-if="itemForm.source_type === 'passive'" :label="t.field.responseSettings"><el-input v-model="itemForm.response_settings_json" type="textarea" /></el-form-item>
          <el-form-item :label="t.field.enabled"><el-switch v-model="itemForm.enabled" /></el-form-item>
        </template>

        <template v-if="drawer.type === 'field'">
          <el-alert v-if="fieldForm.scope_type === 'item'" type="warning" :closable="false" :title="t.overridden" />
          <el-form-item :label="t.field.displayName"><el-input v-model="fieldForm.display_name" /></el-form-item>
          <el-form-item :label="t.field.fieldPath"><el-input v-model="fieldForm.field_path" /></el-form-item>
          <el-form-item :label="t.field.valueType">
            <el-select v-model="fieldForm.value_type">
              <el-option
                v-for="(label, value) in t.valueTypes"
                v-show="fieldForm.scope_type === 'group' || value !== 'object_array'"
                :key="value"
                :label="label"
                :value="value"
              />
            </el-select>
          </el-form-item>
          <template v-if="fieldForm.value_type === 'object_array'">
            <el-form-item :label="t.field.refGroup" required>
              <el-select v-model="fieldForm.ref_group_id" filterable>
                <el-option v-for="g in state.groups" :key="g.id" :label="g.name" :value="g.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t.field.refNamePath">
              <el-input v-model="fieldForm.ref_name_path" placeholder="e.g. device_name" />
            </el-form-item>
          </template>
          <el-form-item :label="t.field.unit"><el-input v-model="fieldForm.unit" /></el-form-item>
          <el-form-item :label="t.field.required"><el-switch v-model="fieldForm.required" /></el-form-item>
          <el-form-item :label="t.field.enabled"><el-switch v-model="fieldForm.enabled" /></el-form-item>
        </template>

        <template v-if="drawer.type === 'rule'">
          <el-form-item :label="t.field.name"><el-input v-model="ruleForm.name" /></el-form-item>
          <el-form-item :label="t.field.ruleType">
            <el-select :model-value="ruleForm.rule_type" @update:model-value="handleRuleTypeChange">
              <el-option v-for="(label, value) in t.ruleTypes" :key="value" :label="label" :value="value" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="isFieldRuleType()" :label="t.field.fieldPath">
            <el-select :model-value="ruleForm.field_path" @update:model-value="applyRuleField">
              <el-option v-for="field in ruleFieldOptions" :key="field.id" :label="`${fieldLabel(field)} / ${field.field_path}`" :value="field.field_path" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="ruleForm.rule_type === 'field_condition'" :label="t.field.operator"><el-select v-model="ruleForm.operator"><el-option v-for="operator in fieldOperatorOptions" :key="operator" :label="local(t.operators, operator)" :value="operator" /></el-select></el-form-item>
          <el-form-item v-if="ruleForm.rule_type === 'field_condition' && operatorNeedsThreshold(ruleForm.operator)" :label="t.field.threshold">
            <el-input v-model="ruleForm.threshold_value" />
            <div class="form-help">{{ t.thresholdHelp }}</div>
          </el-form-item>
          <el-form-item v-if="ruleForm.rule_type === 'aggregate_condition'" :label="t.field.aggregate"><el-select v-model="ruleForm.aggregate_func"><el-option label="avg" value="avg" /><el-option label="max" value="max" /><el-option label="min" value="min" /><el-option label="median" value="median" /><el-option label="count" value="count" /></el-select></el-form-item>
          <el-form-item v-if="ruleForm.rule_type === 'aggregate_condition'" :label="t.field.operator"><el-select v-model="ruleForm.operator"><el-option v-for="operator in aggregateOperatorOptions" :key="operator" :label="local(t.operators, operator)" :value="operator" /></el-select></el-form-item>
          <el-form-item v-if="ruleForm.rule_type === 'aggregate_condition'" :label="t.field.threshold">
            <el-input v-model="ruleForm.threshold_value" />
            <div class="form-help">{{ t.thresholdHelp }}</div>
          </el-form-item>
          <el-form-item v-if="ruleForm.rule_type === 'aggregate_condition'" :label="t.field.window"><el-input-number v-model="ruleForm.aggregate_window_seconds" :min="1" /></el-form-item>
          <el-form-item :label="t.field.consecutive"><el-input-number v-model="ruleForm.consecutive_count" :min="1" /></el-form-item>
          <el-form-item :label="t.field.recovery"><el-input-number v-model="ruleForm.recovery_count" :min="1" /></el-form-item>
          <el-form-item :label="t.field.severity"><el-select v-model="ruleForm.severity"><el-option v-for="(label, value) in t.severities" v-show="value !== 'recovered'" :key="value" :label="label" :value="value" /></el-select></el-form-item>
          <el-form-item :label="t.field.combineGroup">
            <el-select
              v-model="ruleForm.combine_group"
              filterable
              allow-create
              default-first-option
              clearable
              :placeholder="locale === 'zh-CN' ? '选择或输入合并组别（可选）' : 'Select or enter combine group (optional)'"
            >
              <el-option v-for="g in existingCombineGroups" :key="g" :label="g" :value="g" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t.field.channel"><el-select v-model="ruleForm.channel_ids" multiple><el-option v-for="channel in state.channels" :key="channel.id" :label="channel.name" :value="channel.id" /></el-select></el-form-item>
          <el-form-item :label="t.field.messageTemplate">
            <el-input v-model="ruleForm.message_template" type="textarea" />
            <div class="form-help">{{ t.messageVars }}</div>
          </el-form-item>
          <el-form-item :label="t.field.continuousAlert"><el-switch v-model="ruleForm.continuous_alert" /></el-form-item>
          <el-form-item :label="t.field.enabled"><el-switch v-model="ruleForm.enabled" /></el-form-item>
        </template>

        <template v-if="drawer.type === 'channel'">
          <el-form-item :label="t.field.code"><el-input v-model="channelForm.code" /></el-form-item>
          <el-form-item :label="t.field.name"><el-input v-model="channelForm.name" /></el-form-item>
          <el-form-item :label="t.labels.channelTemplate"><el-select v-model="channelForm.channel_type"><el-option :label="t.channelTemplates.dingding" value="dingding" /><el-option :label="t.channelTemplates.feishu" value="feishu" /><el-option :label="t.channelTemplates.telegram" value="telegram" /></el-select></el-form-item>
          <template v-if="channelForm.channel_type === 'dingding' || channelForm.channel_type === 'feishu'">
            <el-form-item :label="t.field.webhook"><el-input v-model="channelForm.webhook" /></el-form-item>
            <el-form-item :label="t.field.secret"><el-input v-model="channelForm.secret" /></el-form-item>
          </template>
          <template v-else-if="channelForm.channel_type === 'telegram'">
            <el-form-item :label="t.field.botToken"><el-input v-model="channelForm.bot_token" /></el-form-item>
            <el-form-item :label="t.field.chatID"><el-input v-model="channelForm.chat_id" /></el-form-item>
          </template>
          <el-form-item :label="t.field.enabled"><el-switch v-model="channelForm.enabled" /></el-form-item>
        </template>

        <div class="drawer-actions">
          <el-button @click="drawer.visible = false">{{ t.cancel }}</el-button>
          <el-button type="primary" @click="submitDrawer">{{ t.save }}</el-button>
        </div>
        </el-form>
      </div>
    </el-drawer>

    <el-drawer v-model="fieldDetail.visible" :title="t.labels.fieldDetail" size="620px">
      <div v-loading="loading" class="drawer-body-content">
        <template v-if="fieldDetail.field && fieldDetail.item">
        <el-descriptions :column="2" border>
          <el-descriptions-item :label="t.field.item">{{ fieldDetail.item.name }}</el-descriptions-item>
          <el-descriptions-item :label="t.field.fieldPath">{{ fieldDetail.field.field_path }}</el-descriptions-item>
          <el-descriptions-item :label="t.field.latest">{{ valueText(sampleValue(latestSample(fieldDetail.item.id), fieldDetail.field.field_path)) }}</el-descriptions-item>
          <el-descriptions-item :label="t.field.count">{{ fieldDetail.stats?.count || 0 }}</el-descriptions-item>
          <el-descriptions-item :label="t.field.avg">{{ fieldDetail.stats?.avg ?? '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t.field.max">{{ fieldDetail.stats?.max ?? '-' }}</el-descriptions-item>
        </el-descriptions>

        <div class="subsection-title">
          <h3>{{ t.labels.trend }}</h3>
          <el-select v-model="fieldStatHours" size="small" class="range-select" @change="reloadFieldStats">
            <el-option v-for="hours in statHourOptions" :key="hours" :label="`${hours}h`" :value="hours" />
          </el-select>
        </div>
        <div ref="fieldTrendChartEl" class="chart"></div>

        <div class="subsection-title">
          <h3>{{ t.labels.rules }}</h3>
          <el-button v-if="isAdmin" size="small" type="primary" :icon="Icons.Plus" @click="openFieldRule(fieldDetail.field, fieldDetail.item)">{{ t.add }}</el-button>
        </div>
        <el-table :data="fieldRules(fieldDetail.item, fieldDetail.field)" size="small">
          <el-table-column prop="name" :label="t.field.name" />
          <el-table-column :label="t.field.ruleType"><template #default="{ row }">{{ local(t.ruleTypes, row.rule_type) }}</template></el-table-column>
          <el-table-column :label="t.field.operator"><template #default="{ row }">{{ ruleOperator(row) }}</template></el-table-column>
          <el-table-column prop="threshold_value" :label="t.field.threshold" />
          <el-table-column prop="consecutive_count" :label="t.field.consecutive" />
          <el-table-column prop="recovery_count" :label="t.field.recovery" />
          <el-table-column :label="t.field.severity"><template #default="{ row }">{{ local(t.severities, row.severity) }}</template></el-table-column>
          <el-table-column prop="combine_group" :label="t.field.combineGroup" min-width="150" show-overflow-tooltip class-name="nowrap-column" label-class-name="nowrap-column" />
          <el-table-column :label="t.field.channels"><template #default="{ row }">{{ ruleChannels(row) }}</template></el-table-column>
          <el-table-column v-if="isAdmin" :label="t.field.action" width="150">
            <template #default="{ row }">
              <el-button link type="primary" @click="openDrawer('rule', 'edit', row)">{{ t.edit }}</el-button>
              <el-button link type="danger" @click="deleteEntity('rule', row.id)">{{ t.delete }}</el-button>
            </template>
          </el-table-column>
        </el-table>

        <h3>{{ t.labels.events }}</h3>
        <el-table :data="pagedFieldEvents(fieldDetail.item, fieldDetail.field)" :empty-text="t.noEvents" size="small">
          <el-table-column :label="t.field.time"><template #default="{ row }">{{ formatDateTime(row.occurred_at) }}</template></el-table-column>
          <el-table-column :label="t.field.severity"><template #default="{ row }">{{ local(t.severities, row.severity) }}</template></el-table-column>
          <el-table-column prop="message" :label="t.field.message" />
        </el-table>
        <el-pagination
          v-model:current-page="fieldEventPage"
          v-model:page-size="fieldEventPageSize"
          class="table-pagination"
          layout="total, sizes, prev, pager, next"
          :page-sizes="[20, 50, 100, 200]"
          :total="fieldEvents(fieldDetail.item, fieldDetail.field).length"
          @size-change="handleFieldEventPageSize"
        />
        </template>
      </div>
    </el-drawer>
  </el-container>
  </el-config-provider>
</template>
