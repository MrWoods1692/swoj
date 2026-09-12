// 极简中英双语：仅导航与通用词走字典，其余文案以中文为主。
import { reactive } from 'vue'

const dict = {
  zh: { home: '首页', problems: '题库', training: '训练计划', contests: '比赛', assignments: '作业',
    discussions: '讨论', submissions: '测评记录', leaderboard: '排行榜', wrong: '错题本', points: '积分',
    shop: '商城', levels: '等级成就', favorites: '收藏', notices: '公告', materials: '资料',
    status: '服务状态', about: '关于', changelog: '更新日志',
    admin: '管理后台', profile: '个人主页', login: '校园墙登录', logout: '退出', mine: '我的',
    logs: '日志',
    ai: 'AI 问答', queue: '测评队列', terms: '用户协议', privacy: '隐私政策' },
  en: { home: 'Home', problems: 'Problems', training: 'Training', contests: 'Contests', assignments: 'Assignments',
    discussions: 'Discuss', submissions: 'Submissions', leaderboard: 'Ranking', wrong: 'Wrong List', points: 'Points',
    shop: 'Shop', levels: 'Levels', favorites: 'Favorites', notices: 'News', materials: 'Materials',
    status: 'Status', about: 'About', changelog: 'Changelog',
    admin: 'Admin', profile: 'Profile', login: 'Campux Login', logout: 'Logout', mine: 'Mine',
    logs: 'Logs',
    ai: 'AI Ask', queue: 'Judge Queue', terms: 'Terms of Service', privacy: 'Privacy Policy' },
}
const saved = localStorage.getItem('swoj_lang') || 'zh'
export const i18n = reactive({
  lang: saved,
  t(key) { return (dict[this.lang] || dict.zh)[key] || key },
  toggle() { this.lang = this.lang === 'zh' ? 'en' : 'zh'; localStorage.setItem('swoj_lang', this.lang) },
})
