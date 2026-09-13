<script setup>
import { computed } from 'vue'
import { auth } from '../api'
import { i18n } from '../i18n'

const zh = computed(() => i18n.lang === 'zh')

const features = [
  ['题库', '题目浏览、难度与标签筛选、题解讨论入口'],
  ['训练计划', '按主题组织的练习路线，逐题记录进度'],
  ['比赛', '限时比赛、报名制私有赛、独立排行榜'],
  ['作业', '教师布置的单题作业与截止时限'],
  ['测评', 'C++ 代码提交，自动判题并回传详细状态'],
  ['错题本', '自动收集未通过的题目，方便专项订正'],
  ['排行榜', '按通过题数与总用时排名的全站榜单'],
  ['个人主页', '做题热力图、活跃度、算力占用与成就'],
  ['积分商城', '签到与做题累积积分，兑换虚拟商品'],
  ['等级成就', '十级成长体系与里程碑成就解锁'],
  ['讨论区', '题目解答讨论、点赞与回复'],
  ['管理后台', '题目维护、测评节点、IP 封禁与操作日志'],
]

const stack = [
  ['后端', 'Go 1.22 + SQLite（modernc.org/sqlite 纯 Go 驱动）'],
  ['测评', 'go-judge，支持 C++ 提交，独立队列并发执行'],
  ['前端', 'Vue 3 + Vite，单页应用'],
  ['登录', '校园墙 OAuth 授权，Cookie 会话 + CSRF 防护'],
]
</script>

<template>
  <div class="panel">
    <h2>关于 Swoj</h2>
    <p class="muted" style="line-height:1.8">
      Swoj（School Wall Online Judge）是一套面向校园算法学习的在线评测系统。它把题库、训练、比赛、作业、
      讨论与个人成长串联成完整闭环：提交 C++ 代码后由独立测评进程自动判题，结果写入测评记录，
      未通过的题目自动进入错题本，通过后的题数推进行程与排行榜。
    </p>
    <div class="row" style="margin-top:14px">
      <span class="chip small">纯 Go 后端</span>
      <span class="chip small">零 C 编译依赖</span>
      <span class="chip small">单库部署</span>
      <span class="chip small">校园墙单点登录</span>
    </div>
  </div>

  <div class="panel">
    <h3>功能模块</h3>
    <div class="grid grid--3">
      <div v-for="[k, v] in features" :key="k" class="card">
        <div style="font-weight:700;color:var(--accent)">{{ k }}</div>
        <div class="small muted" style="margin-top:6px;line-height:1.6">{{ v }}</div>
      </div>
    </div>
  </div>

  <div class="panel">
    <h3>技术栈</h3>
    <table class="tbl">
      <tbody>
        <tr v-for="[k, v] in stack" :key="k">
          <th style="width:100px">{{ k }}</th><td class="mono small">{{ v }}</td>
        </tr>
      </tbody>
    </table>
  </div>

  <div class="panel">
    <h3>使用提示</h3>
    <ul class="muted" style="line-height:2;margin:0;padding-left:20px">
      <li>首次通过校园墙登录后，请补齐真实姓名与个人简介以解锁提交权限。</li>
      <li>目前仅支持 C++ 提交，编译命令由 go-judge 统一封装。</li>
      <li>每天可签到一次获得积分，签到连续天数影响成就解锁。</li>
      <li>排行榜按去重 AC 题数降序、总用时升序排列。</li>
    </ul>
    <div class="small muted" style="margin-top:14px">
      当前语言：{{ zh ? '简体中文' : 'English' }} ·
      <a href="javascript:void(0)" @click="i18n.toggle()">{{ zh ? '切换 English' : '切换到中文' }}</a>
    </div>
  </div>
</template>
