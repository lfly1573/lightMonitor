export type Locale = 'zh-CN' | 'en-US'

export const messages = {
  'zh-CN': {
    title: '监控总览',
    subtitle: '轻量级数据采集、分析与报警中心',
    nav: {
      overview: '总览',
      groups: '监控分组',
      alerts: '报警通知',
      settings: '系统设置',
    },
    install: {
      title: '初始化系统',
      username: '管理员账号',
      password: '管理员密码',
      submit: '创建并进入',
    },
    overview: {
      title: '运行状态',
      empty: '框架已就绪，等待接入监控数据。',
    },
  },
  'en-US': {
    title: 'Overview',
    subtitle: 'Lightweight data collection, analytics, and alerts',
    nav: {
      overview: 'Overview',
      groups: 'Groups',
      alerts: 'Alerts',
      settings: 'Settings',
    },
    install: {
      title: 'Initialize System',
      username: 'Admin username',
      password: 'Admin password',
      submit: 'Create and Enter',
    },
    overview: {
      title: 'Runtime',
      empty: 'The framework is ready and waiting for monitor data.',
    },
  },
} as const
