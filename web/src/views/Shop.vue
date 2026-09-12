<script setup>
import { onMounted, ref } from 'vue'
import { api, fmtDate, toast } from '../api'
import { useRouterGuard } from '../composables'

useRouterGuard()
const balance = ref(0)
const items = ref([])
const orders = ref([])

const load = async () => {
  items.value = await api.get('/api/shop').catch(() => [])
  orders.value = await api.get('/api/shop/orders').catch(() => [])
  balance.value = (await api.get('/api/points').catch(() => ({ balance: 0 }))).balance ?? 0
}
onMounted(load)

async function redeem(it) {
  if (!confirm(`使用 ${it.price} 积分兑换「${it.name}」？`)) return
  try {
    const r = await api.post('/api/shop/redeem', { item_id: it.id })
    toast(`兑换成功，当前余额 ${r.balance}`)
    await load()
  } catch (e) { toast(e.message, false) }
}
</script>

<template>
  <div class="panel">
    <div class="row">
      <h2 style="margin:0">积分商城</h2>
      <span class="spacer" />
      <span class="chip" style="color:var(--accent)">余额 {{ balance }} 积分</span>
    </div>
  </div>

  <div class="panel">
    <div class="grid grid--3">
      <div v-for="it in items" :key="it.id" class="card">
        <h3 style="margin:0">{{ it.name }}</h3>
        <div class="small muted" style="margin:6px 0">{{ it.description }}</div>
        <div class="row" style="margin-bottom:10px">
          <span class="chip" style="color:var(--accent)">{{ it.price }} 积分</span>
          <span class="chip">库存 {{ it.stock }}</span>
          <span class="chip small">发布者 {{ it.publisher }}</span>
        </div>
        <button class="btn btn--primary" :disabled="balance < it.price" @click="redeem(it)">兑换</button>
      </div>
      <div v-if="!items.length" class="muted">暂无在售商品</div>
    </div>
  </div>

  <div class="panel">
    <h3>我的兑换记录</h3>
    <table class="tbl">
      <thead><tr><th>ID</th><th>商品</th><th>花费</th><th>时间</th></tr></thead>
      <tbody>
        <tr v-for="o in orders" :key="o.id">
          <td class="mono">{{ o.id }}</td>
          <td>{{ o.item_name }}</td>
          <td style="color:var(--err)">-{{ o.price }}</td>
          <td class="muted small">{{ fmtDate(o.created_at) }}</td>
        </tr>
        <tr v-if="!orders.length"><td colspan="4" class="muted">暂无兑换记录</td></tr>
      </tbody>
    </table>
  </div>
</template>
