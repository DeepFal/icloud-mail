export const demoGroups = ['工作', '生活', '订阅', '项目']

export function createDemoData() {
  const accounts = [
    {
      id: 'account-icloud-main',
      name: '林岚 · 主账号',
      email: 'lan@example.test',
      initial: '岚',
      color: '#4f7cff',
      managementStatus: 'ready',
      receivingStatus: 'ready',
      lastSync: '2026-10-29 09:42',
      aliasLimit: 100,
      detail: '主工作账号，收件与别名管理均已连接。'
    },
    {
      id: 'account-icloud-personal',
      name: '林岚 · 个人账号',
      email: 'lan.personal@example.test',
      initial: '林',
      color: '#d97757',
      managementStatus: 'ready',
      receivingStatus: 'attention',
      lastSync: '2026-10-28 18:16',
      aliasLimit: 50,
      detail: '个人生活账号，管理连接正常；收件凭据待更新。'
    }
  ]

  const aliases = [
    { id: 'alias-001', accountId: 'account-icloud-main', address: 'lan.design@example.test', purpose: '设计项目对外联络', group: '工作', enabled: true, poolState: 'personal', projectId: null, createdAt: '2026-08-12' },
    { id: 'alias-002', accountId: 'account-icloud-main', address: 'lan.team@example.test', purpose: '团队协作与会议通知', group: '工作', enabled: true, poolState: 'personal', projectId: null, createdAt: '2026-08-20' },
    { id: 'alias-003', accountId: 'account-icloud-main', address: 'lan.research@example.test', purpose: '研究资料和访谈预约', group: '项目', enabled: true, poolState: 'leased', projectId: 'project-research', createdAt: '2026-09-03' },
    { id: 'alias-004', accountId: 'account-icloud-main', address: 'lan.invoices@example.test', purpose: '供应商账单与发票', group: '工作', enabled: true, poolState: 'used', projectId: 'project-redesign', createdAt: '2026-09-18' },
    { id: 'alias-005', accountId: 'account-icloud-main', address: 'lan.news@example.test', purpose: '行业周报和阅读订阅', group: '订阅', enabled: true, poolState: 'personal', projectId: null, createdAt: '2026-07-29' },
    { id: 'alias-006', accountId: 'account-icloud-main', address: 'lan.travel@example.test', purpose: '旅行预订和行程提醒', group: '生活', enabled: true, poolState: 'personal', projectId: null, createdAt: '2026-08-05' },
    { id: 'alias-007', accountId: 'account-icloud-personal', address: 'lan.shopping@example.test', purpose: '购物订单与售后', group: '生活', enabled: true, poolState: 'personal', projectId: null, createdAt: '2026-08-30' },
    { id: 'alias-008', accountId: 'account-icloud-personal', address: 'lan.family@example.test', purpose: '家庭事务和亲友联系', group: '生活', enabled: true, poolState: 'personal', projectId: null, createdAt: '2026-09-10' },
    { id: 'alias-009', accountId: 'account-icloud-main', address: 'lan.docs@example.test', purpose: '协作文档共享邀请', group: '项目', enabled: true, poolState: 'leased', projectId: 'project-research', createdAt: '2026-10-02' },
    { id: 'alias-010', accountId: 'account-icloud-main', address: 'lan.available@example.test', purpose: '待分配的备用地址', group: '订阅', enabled: true, poolState: 'available', projectId: null, createdAt: '2026-10-11' },
    { id: 'alias-011', accountId: 'account-icloud-main', address: 'lan.reserve@example.test', purpose: '待分配的备用地址', group: '项目', enabled: true, poolState: 'available', projectId: null, createdAt: '2026-10-12' },
    { id: 'alias-012', accountId: 'account-icloud-personal', address: 'lan.unused@example.test', purpose: '待分配的备用地址', group: '生活', enabled: true, poolState: 'available', projectId: null, createdAt: '2026-10-13' }
  ]

  const messages = [
    {
      id: 'message-001', aliasId: 'alias-001', sender: '周宁', senderEmail: 'ning@example.test', initial: '周', color: '#8b5cf6',
      subject: '红杉路改造项目：本周评审材料', preview: '我把方案中的动线、照明和无障碍部分重新整理了一遍，附件里是可以直接评审的版本。', time: '09:18', date: '今天', unread: true, starred: true, category: '普通邮件', otp: null,
      paragraphs: ['林岚，你好：', '我把红杉路改造项目中的动线、照明和无障碍部分重新整理了一遍，重点标注了需要在周五评审会上确认的两处取舍。', '附件中的版本已经同步到共享文档，现场如果对入口坡道的宽度有不同意见，我们可以直接在第二页的平面图上修改。', '周宁'],
      link: { label: '打开项目文档', url: 'https://example.test/docs/redwood-review' }, attachments: [{ name: '红杉路评审材料.txt', size: '8 KB', content: `红杉路改造项目评审材料（模拟附件）
1. 入口坡道宽度：建议 1.8 米
2. 夜间照明：新增两处低位灯
3. 待确认：雨水口位置` }]
    },
    {
      id: 'message-002', aliasId: 'alias-007', sender: '云集商店', senderEmail: 'order@example.test', initial: '云', color: '#f59e0b', subject: '订单 #EX-2048 已发货', preview: '你的订单已经交给快递，预计周六送达。', time: '08:46', date: '今天', unread: true, starred: false, category: '普通邮件', otp: null,
      paragraphs: ['你好，林岚：', '订单 #EX-2048 中的两件商品已经发货，承运商为示例速递，预计 10 月 31 日送达。', '如需修改收货信息，请在包裹进入派送前联系商家。感谢你的购买。'], attachments: []
    },
    {
      id: 'message-003', aliasId: 'alias-006', sender: '北岸航空', senderEmail: 'trip@example.test', initial: '北', color: '#0ea5e9', subject: '你的上海行程已确认', preview: '航班、酒店和机场接送已经整理到同一份行程单。', time: '21:03', date: '昨天', unread: false, starred: true, category: '普通邮件', otp: null,
      paragraphs: ['林岚，您好：', '你的上海行程已确认：11 月 6 日 CA0001 航班，11 月 6 日至 9 日入住江畔酒店。', '机场接送司机会在起飞前一天通过此地址发送集合地点。请提前检查证件和行李额度。'], link: { label: '查看行程单', url: 'https://example.test/travel/itinerary-2048' }, attachments: []
    },
    {
      id: 'message-004', aliasId: 'alias-009', sender: '白板协作', senderEmail: 'workspace@example.test', initial: '白', color: '#10b981', subject: '有人在协作文档中提到你', preview: '周宁在“用户访谈整理”文档中标记了你的评论。', time: '18:40', date: '昨天', unread: true, starred: false, category: '普通邮件', otp: null,
      paragraphs: ['林岚：', '周宁在“用户访谈整理”文档中回复了你的评论：“请把第三位受访者的原话放到结论前面”。', '你可以打开文档查看上下文并继续回复。'], link: { label: '打开协作文档', url: 'https://example.test/workspaces/interviews' }, attachments: []
    },
    {
      id: 'message-005', aliasId: 'alias-005', sender: '慢读周刊', senderEmail: 'weekly@example.test', initial: '慢', color: '#64748b', subject: '本周刊：把复杂问题写清楚', preview: '本期从三个真实案例出发，讨论如何减少会议中的信息损耗。', time: '07:30', date: '昨天', unread: false, starred: false, category: '普通邮件', otp: null,
      paragraphs: ['本周我们讨论如何把复杂问题写清楚：先给结论，再给证据，最后说明仍然不确定的部分。', '另有一篇关于团队决策记录的短文，欢迎在周末阅读。'], attachments: []
    },
    {
      id: 'message-006', aliasId: 'alias-003', sender: '资料库', senderEmail: 'library@example.test', initial: '资', color: '#a855f7', subject: '你订阅的资料已更新', preview: '研究资料库新增了 14 篇关于公共空间的论文摘要。', time: '16:24', date: '10月27日', unread: false, starred: false, category: '普通邮件', otp: null,
      paragraphs: ['研究资料库已更新，新增 14 篇关于公共空间和步行体验的论文摘要。', '你保存的“城市边界”主题也有两条新记录。'], attachments: []
    },
    {
      id: 'message-007', aliasId: 'alias-002', sender: '会议助手', senderEmail: 'calendar@example.test', initial: '会', color: '#2563eb', subject: '明天 10:00 项目评审', preview: '会议链接和议程已更新，请提前查看材料。', time: '10:00', date: '10月27日', unread: true, starred: false, category: '普通邮件', otp: null,
      paragraphs: ['提醒：明天 10:00 进行项目评审。', '议程包括范围确认、预算变更和上线风险。会议预计 45 分钟。'], link: { label: '查看会议议程', url: 'https://example.test/calendar/review' }, attachments: []
    },
    {
      id: 'message-008', aliasId: 'alias-008', sender: '家庭云相册', senderEmail: 'photos@example.test', initial: '家', color: '#ec4899', subject: '新的相册共享给你', preview: '“秋天的周末”相册已共享，包含 24 张照片。', time: '18:30', date: '10月26日', unread: false, starred: true, category: '普通邮件', otp: null,
      paragraphs: ['“秋天的周末”相册已共享给你，共有 24 张照片。', '你可以查看、下载或添加备注。'], link: { label: '查看相册', url: 'https://example.test/photos/autumn-weekend' }, attachments: []
    },
    {
      id: 'message-009', aliasId: 'alias-001', sender: '设计工具', senderEmail: 'verify@example.test', initial: '设', color: '#f97316', subject: '登录验证码：482913', preview: '验证码 482913，10 分钟内有效。', time: '15:06', date: '10月26日', unread: true, starred: false, category: '验证码', otp: '482913', paragraphs: ['你正在登录设计工具。', '验证码：482913', '验证码 10 分钟内有效，请勿转发给他人。'], attachments: []
    },
    {
      id: 'message-010', aliasId: 'alias-007', sender: '云集商店', senderEmail: 'verify@example.test', initial: '云', color: '#f59e0b', subject: '确认你的邮箱地址', preview: '点击链接完成邮箱确认，链接 24 小时内有效。', time: '11:20', date: '10月26日', unread: true, starred: false, category: '验证链接', otp: null, paragraphs: ['请确认 lan.shopping@example.test 是你的邮箱地址。', '如果你没有发起此操作，可以忽略这封邮件。'], link: { label: '确认邮箱地址', url: 'https://example.test/account/verify/EX-2048' }, attachments: []
    },
    {
      id: 'message-011', aliasId: 'alias-003', sender: '研究门户', senderEmail: 'verify@example.test', initial: '研', color: '#14b8a6', subject: '安全码 731064', preview: '使用安全码 731064 完成研究门户登录。', time: '20:46', date: '10月25日', unread: false, starred: false, category: '验证码', otp: '731064', paragraphs: ['研究门户收到一次新的登录请求。', '安全码：731064', '如果不是你本人操作，请尽快修改密码。'], attachments: []
    },
    {
      id: 'message-012', aliasId: 'alias-005', sender: '慢读周刊', senderEmail: 'unsubscribe@example.test', initial: '慢', color: '#64748b', subject: '订阅偏好已更新', preview: '你已将周刊频率调整为每周一次。', time: '08:30', date: '10月25日', unread: false, starred: false, category: '验证链接', otp: null, paragraphs: ['你的订阅偏好已更新为每周一次。', '如需恢复每日摘要，可以在偏好设置中修改。'], link: { label: '管理订阅偏好', url: 'https://example.test/preferences/weekly' }, attachments: []
    }
  ]

  const projects = [
    { id: 'project-redesign', name: '红杉路改造', description: '整理评审材料、外部联络与现场反馈。', initial: '红', color: '#4f7cff', enabled: true, keyHint: 'redwood', accountIds: ['account-icloud-main'] },
    { id: 'project-research', name: '用户访谈整理', description: '集中管理研究资料、访谈预约和协作文档。', initial: '研', color: '#10b981', enabled: true, keyHint: 'research', accountIds: ['account-icloud-main', 'account-icloud-personal'] }
  ]

  const activities = [
    { id: 'activity-001', time: '09:42', title: '主账号同步完成', description: '主账号的邮件与别名数据已更新。', type: 'success' },
    { id: 'activity-002', time: '09:15', title: '发现收件凭据待更新', description: '个人账号需要重新验证收件连接。', type: 'warning' },
    { id: 'activity-003', time: '昨天 18:40', title: '协作文档收到提及', description: '“用户访谈整理”中有一条新的评论。', type: 'info' },
    { id: 'activity-004', time: '昨天 16:20', title: '别名已分配到项目', description: 'lan.docs@example.test 已加入用户访谈整理。', type: 'success' },
    { id: 'activity-005', time: '周一 10:05', title: '完成安全码登录', description: '研究门户登录验证码已使用。', type: 'info' }
  ]

  return { accounts, aliases, messages, projects, activities }
}
