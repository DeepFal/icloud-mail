<script setup>
import { onBeforeUnmount, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { apiRequest } from "../api/client.js";
import { useAuth } from "../stores/auth.js";
const emit=defineEmits(["change"]);const auth=useAuth();const items=ref([]),error=ref(""),busy=ref(""),open=ref(false);let controller,timer,disposed=false,loading=false;
async function load(){if(loading||disposed)return;loading=true;controller=new AbortController();try{const result=await apiRequest('/pool/release-reviews',{signal:controller.signal});if(!disposed){items.value=result.items;error.value=""}}catch(cause){if(cause.name!=="AbortError"&&!disposed)error.value=cause.message||"待确认记录加载失败"}finally{loading=false}}
async function resolve(item,unused){if(busy.value)return;try{await ElMessageBox.confirm(unused?'请确认该地址没有绑定任何业务。确认后它会重新进入可领取库存。':'确认该地址实际已经使用，继续保留给原项目，不再重新发放。',unused?'确认未使用并归还':'确认已使用',{confirmButtonText:unused?'确认未使用，归还':'确认已使用',cancelButtonText:'暂不处理',type:'warning'})}catch{return}busy.value=item.lease_id;try{await apiRequest(`/pool/release-reviews/${encodeURIComponent(item.lease_id)}/resolve`,{method:'POST',body:{unused},csrfToken:auth.state.csrfToken});ElMessage.success('分配记录已核实');await load();emit('change')}catch(cause){ElMessage.error(cause.message||'核实失败，请刷新记录')}finally{busy.value=''}}
function schedule(){clearTimeout(timer);if(disposed||document.hidden||!navigator.onLine)return;timer=setTimeout(async()=>{await load();schedule()},30000)}
function wake(){load();schedule()}
onMounted(()=>{load();schedule();document.addEventListener('visibilitychange',wake);window.addEventListener('online',wake);window.addEventListener('offline',schedule)});onBeforeUnmount(()=>{disposed=true;controller?.abort();clearTimeout(timer);document.removeEventListener('visibilitychange',wake);window.removeEventListener('online',wake);window.removeEventListener('offline',schedule)});
</script>
<template>
  <el-button
    v-if="items.length || error"
    class="release-reviews__trigger"
    :class="{ 'is-error': error && !items.length }"
    aria-haspopup="dialog"
    @click="open = true"
  >
    <span class="pg-dot" :class="items.length ? 'is-warning' : 'is-danger'">{{ items.length ? `${items.length} 个待核实` : "待核实读取失败" }}</span>
  </el-button>
  <el-drawer v-model="open" title="到期地址核实" size="min(480px, 100vw)" append-to-body class="pg-drawer release-reviews">
    <p class="release-reviews__hint">程序没有确认使用，系统无法判断业务是否成功。这些地址已暂停分配，旧访问凭据已失效。</p>
    <p v-if="error" class="release-reviews__error" role="alert">{{ error }}</p>
    <p v-if="!items.length && !error" class="release-reviews__hint">没有需要核实的地址。</p>
    <ul class="release-reviews__list">
      <li v-for="item in items" :key="item.lease_id" class="review-row">
        <div class="review-row__main">
          <strong class="pg-ellipsis" :title="item.address" translate="no">{{ item.address }}</strong>
          <span class="pg-muted">领取项目 {{ item.project }}</span>
        </div>
        <div class="review-row__actions">
          <el-button size="small" :disabled="Boolean(busy)" @click="resolve(item, false)">确认已使用</el-button>
          <el-button size="small" :disabled="Boolean(busy)" @click="resolve(item, true)">未使用，归还</el-button>
        </div>
      </li>
    </ul>
  </el-drawer>
</template>
<style scoped>
.release-reviews__trigger .pg-dot { color: var(--text); font-size: 12px; }
.release-reviews__hint { margin: 0 0 12px; color: var(--text-secondary); font-size: 12px; line-height: 18px; }
.release-reviews__error { margin: 0 0 12px; color: var(--danger); font-size: 12px; overflow-wrap: anywhere; }
.release-reviews__list { margin: 0; padding: 0; list-style: none; border-top: 1px solid var(--border); }
.review-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 0; border-bottom: 1px solid var(--border); }
.review-row__main { display: grid; gap: 2px; min-width: 0; font-size: 13px; }
.review-row__main strong { color: var(--text); font-weight: 500; }
.review-row__main span { font-size: 12px; }
.review-row__actions { display: flex; flex: 0 0 auto; gap: 4px; }
@media (max-width: 520px) { .review-row { align-items: flex-start; flex-direction: column; } }
</style>
