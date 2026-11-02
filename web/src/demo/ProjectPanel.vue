<script setup>
import { computed, ref, watch } from 'vue'
import { ElDialog, ElMessage } from 'element-plus'
import { Check, CopyDocument, Link, Plus, Refresh, Remove, UserFilled } from '@element-plus/icons-vue'

const props = defineProps({
  aliases: { type: Array, default: () => [] },
  projects: { type: Array, default: () => [] },
  accounts: { type: Array, default: () => [] }
})

const emit = defineEmits(['action'])
const selectedProjectId = ref(props.projects[0]?.id || '')
const dialogVisible = ref(false)
const projectName = ref('')
const projectDescription = ref('')

const selectedProject = computed(() => props.projects.find((project) => project.id === selectedProjectId.value) || null)
const projectAliases = computed(() => props.aliases.filter((alias) => alias.projectId === selectedProjectId.value))
const readyAccountIds = computed(() => new Set(props.accounts.filter((account) => account.receivingStatus === 'ready').map((account) => account.id)))
const availableAliases = computed(() => props.aliases.filter((alias) => alias.enabled && alias.poolState === 'available' && !alias.projectId && readyAccountIds.value.has(alias.accountId) && selectedProject.value?.accountIds.includes(alias.accountId)))
const personalPoolAliases = computed(() => props.aliases.filter((alias) => alias.enabled && alias.poolState === 'personal' && !alias.projectId))
const inventory = computed(() => availableAliases.value.length)
const occupied = computed(() => projectAliases.value.filter((alias) => alias.poolState === 'leased').length)
const used = computed(() => projectAliases.value.filter((alias) => alias.poolState === 'used').length)
const pending = computed(() => projectAliases.value.filter((alias) => alias.poolState === 'review').length)

const accountName = (accountId) => props.accounts.find((account) => account.id === accountId)?.name || '未知账号'
const stateText = (state) => ({ leased: '已占用 · 待确认', used: '已确认使用', review: '到期待核实', available: '空闲库存', personal: '个人地址' }[state] || state)
const stateClass = (state) => ({ leased: 'pill-warning', used: 'pill-success', review: 'pill-warning', available: 'pill-success', personal: '' }[state] || '')
watch(() => props.projects.length, (length, previous) => {
  const projects = props.projects
  if (previous !== undefined && length > previous) selectedProjectId.value = projects[length - 1].id
  else if (!projects.some((project) => project.id === selectedProjectId.value)) selectedProjectId.value = projects[0]?.id || ''
}, { immediate: true })

function send(type, payload = {}) { emit('action', { type, ...payload }) }
function enroll(aliasId) { send('enroll', { aliasId }) }
function openCreate() { projectName.value = ''; projectDescription.value = ''; dialogVisible.value = true }
function createProject() {
  if (!projectName.value.trim()) return ElMessage.warning('请填写项目名称')
  send('create-project', { name: projectName.value.trim(), description: projectDescription.value.trim() })
  dialogVisible.value = false
}
async function copyKey(id) {
  const value = `pool_demo_${id}`
  try {
    await navigator.clipboard.writeText(value)
    ElMessage.success('演示 Key 已复制')
  } catch {
    ElMessage.error('复制失败，请手动记录演示 Key')
  }
}
</script>

<template>
  <section class="project-panel workspace-page">
    <div class="page-heading panel-heading">
      <div>
        <div class="page-eyebrow">项目与 API</div>
        <h1>把隐私地址分配给程序</h1>
        <p class="page-description">从现有隐私地址中分配给程序。这里展示的是演示数据，当前为本地模拟。</p>
      </div>
      <button class="btn btn-primary" type="button" @click="openCreate"><Plus :size="16" /> 新建项目</button>
    </div>

    <div class="project-layout">
      <aside class="project-list" aria-label="项目列表">
        <div class="section-label">项目</div>
        <button v-for="project in projects" :key="project.id" class="project-item" :class="{ active: project.id === selectedProjectId }" type="button" @click="selectedProjectId = project.id">
          <span class="project-dot" :style="{ background: project.color || 'var(--primary)' }">{{ project.initial || project.name.slice(0, 1) }}</span>
          <span class="project-item-copy"><strong>{{ project.name }}</strong><small>{{ project.enabled ? '运行中' : '已停用' }}</small></span>
          <span class="project-count">{{ aliases.filter((alias) => alias.projectId === project.id).length }}</span>
        </button>
        <div v-if="!projects.length" class="empty-state">还没有项目</div>
      </aside>

      <main class="project-main">
        <div v-if="selectedProject" class="project-title-row">
          <div><h2>{{ selectedProject.name }}</h2><p>{{ selectedProject.description || '暂无项目说明' }}</p></div>
          <div class="title-actions">
            <button class="icon-button" type="button" title="复制演示 Key" @click="copyKey(selectedProject.id)"><CopyDocument :size="16" /></button>
            <button class="btn btn-quiet" type="button" @click="send('toggle-project', { projectId: selectedProject.id })"><Refresh :size="15" /> {{ selectedProject.enabled ? '停用' : '恢复' }}</button>
          </div>
        </div>

        <div class="stats-row" aria-label="地址库存统计">
          <div><span>库存</span><strong>{{ inventory }}</strong><small>本项目可领取的地址</small></div>
          <div><span>已占用</span><strong>{{ occupied }}</strong><small>已分配待确认</small></div>
          <div><span>已使用</span><strong>{{ used }}</strong><small>项目已确认使用</small></div>
          <div><span>待确认</span><strong>{{ pending }}</strong><small>需要项目方核实</small></div>
        </div>

        <section class="panel-section">
          <div class="section-heading"><div><h3>本项目分配</h3><p>按项目分配地址，已使用的地址不再发放。当前为本地模拟。</p></div></div>
          <div class="table-scroll"><table class="data-table"><thead><tr><th>地址</th><th>来源</th><th>状态</th><th>操作</th></tr></thead><tbody>
            <tr v-for="alias in projectAliases" :key="alias.id"><td><strong>{{ alias.address }}</strong><small>{{ alias.purpose }}</small></td><td>{{ accountName(alias.accountId) }}</td><td><span class="pill" :class="stateClass(alias.poolState)">{{ stateText(alias.poolState) }}</span></td><td><div class="row-actions"><button v-if="alias.poolState === 'leased'" class="btn btn-primary btn-small" type="button" @click="send('commit', { aliasId: alias.id })"><Check :size="14" /> 确认使用</button><button v-if="alias.poolState === 'leased'" class="btn btn-quiet btn-small" type="button" @click="send('release', { aliasId: alias.id })"><Remove :size="14" /> 归还未使用</button><button v-if="alias.poolState === 'leased'" class="btn btn-quiet btn-small" type="button" @click="send('review', { aliasId: alias.id })">模拟到期</button><button v-if="alias.poolState === 'review'" class="btn btn-quiet btn-small" type="button" @click="send('release', { aliasId: alias.id })">核实未使用后归还</button></div></td></tr>
            <tr v-if="!projectAliases.length"><td colspan="4" class="empty-state">这个项目还没有分配地址</td></tr>
          </tbody></table></div>
        </section>

        <section class="panel-section">
          <div class="section-heading"><div><h3>从个人地址入池</h3><p>只可加入启用中的个人地址，且地址不能已经归属其他项目。</p></div></div>
          <div class="enroll-list"><div v-for="alias in personalPoolAliases" :key="alias.id" class="enroll-row"><span class="enroll-icon"><UserFilled :size="15" /></span><div><strong>{{ alias.address }}</strong><small>{{ accountName(alias.accountId) }} · {{ alias.purpose }}</small></div><button class="btn btn-quiet btn-small" type="button" @click="enroll(alias.id)"><Link :size="14" /> 加入库存</button></div><div v-if="!personalPoolAliases.length" class="empty-state">没有符合条件的个人地址</div></div>
        </section>

        <section class="panel-section"><div class="section-heading"><div><h3>空闲库存</h3><p>仅显示本项目允许使用、且收件正常的地址。</p></div><button class="btn btn-primary btn-small" type="button" :disabled="!selectedProject?.enabled || !availableAliases.length" @click="send('claim', { projectId: selectedProjectId })">模拟领取</button></div><div class="inventory-chips"><span v-for="alias in availableAliases" :key="alias.id" class="inventory-chip"><span><strong>{{ alias.address }}</strong><small>{{ alias.purpose }}</small></span></span><div v-if="!availableAliases.length" class="empty-state">当前没有空闲库存</div></div></section>
      </main>
    </div>

    <el-dialog v-model="dialogVisible" title="新建项目" width="min(440px, calc(100vw - 32px))"><div class="dialog-form"><label>项目名称<input v-model="projectName" maxlength="60" placeholder="例如：市场调研" /></label><label>项目说明<textarea v-model="projectDescription" rows="3" maxlength="160" placeholder="说明这个项目会如何使用隐私地址" /></label></div><template #footer><button class="btn btn-quiet" type="button" @click="dialogVisible = false">取消</button><button class="btn btn-primary" type="button" @click="createProject">创建项目</button></template></el-dialog>
  </section>
</template>

<style scoped>
.project-panel { color: var(--text); font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", sans-serif; font-size: 12px; }
.panel-heading { align-items: flex-start; gap: 20px; }
.panel-heading h1 { margin: 4px 0 7px; font-size: 24px; letter-spacing: -.02em; }
.panel-heading .btn { flex: none; margin-top: 5px; }
.project-layout { display: grid; grid-template-columns: 180px minmax(0, 1fr); gap: 16px; max-width: 1100px; }
.project-list, .project-main, .panel-section { background: var(--surface); border: 1px solid var(--border); border-radius: 14px; }
.project-list { padding: 12px; align-self: start; }
.section-label { padding: 6px 9px 9px; color: var(--text-secondary); font-size: 12px; font-weight: 700; }
.project-item { display: flex; align-items: center; width: 100%; gap: 10px; border: 0; border-radius: 10px; padding: 9px; background: transparent; color: var(--text); text-align: left; cursor: pointer; }
.project-item:hover, .project-item.active { background: var(--primary-soft); }
.project-dot { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 9px; color: white; font-size: 13px; font-weight: 700; }
.project-item-copy { min-width: 0; flex: 1; } .project-item-copy strong, .project-item-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; } .project-item-copy small, td small, .enroll-row small, .inventory-chip small { margin-top: 3px; color: var(--text-secondary); font-size: 12px; }
.project-count { color: var(--text-secondary); font-size: 12px; }
.project-main { min-width: 0; padding: 20px; }
.project-title-row, .section-heading { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
h2, h3 { margin: 0; } h2 { font-size: 21px; } h3 { font-size: 16px; } .project-title-row p, .section-heading p { margin: 5px 0 0; color: var(--text-secondary); font-size: 13px; }
.title-actions, .row-actions { display: flex; align-items: center; gap: 7px; flex-wrap: wrap; justify-content: flex-end; }
.stats-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; margin: 20px 0; }
.stats-row > div { border: 1px solid var(--border); border-radius: 10px; padding: 12px; background: var(--surface-subtle); } .stats-row span, .stats-row small { display: block; color: var(--text-secondary); font-size: 12px; } .stats-row strong { display: block; margin: 5px 0 2px; font-size: 23px; }
.panel-section { margin-top: 14px; padding: 16px; } .table-scroll { margin-top: 14px; }
table { min-width: 650px; } td strong, td small { display: block; } td { vertical-align: middle; }
.btn-small { padding: 6px 9px; font-size: 12px; }
.enroll-list, .inventory-chips { display: grid; gap: 8px; margin-top: 14px; }
.enroll-row, .inventory-chip { display: flex; align-items: center; gap: 10px; border: 1px solid var(--border); border-radius: 10px; padding: 10px; } .enroll-row > div, .inventory-chip > span { min-width: 0; flex: 1; } .enroll-icon { display: grid; place-items: center; width: 28px; height: 28px; border-radius: 8px; color: var(--primary); background: var(--primary-soft); }
.inventory-chip { justify-content: space-between; } .inventory-chip strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dialog-form { display: grid; gap: 14px; } .dialog-form label { display: grid; gap: 6px; color: var(--text-secondary); font-size: 13px; font-weight: 600; } .dialog-form input, .dialog-form textarea { width: 100%; box-sizing: border-box; border: 1px solid var(--border-strong); border-radius: 8px; padding: 9px 10px; background: var(--surface); color: var(--text); font: inherit; resize: vertical; }
@media (max-width: 1100px) { .project-layout { grid-template-columns: 1fr; } .project-list { display: flex; gap: 4px; overflow-x: auto; padding: 8px; } .section-label { display: none; } .project-item { min-width: 150px; } .stats-row { grid-template-columns: repeat(2, 1fr); } .project-main { padding: 14px; } }
@media (max-width: 390px) { .panel-heading { display: block; } .panel-heading .btn { margin-top: 12px; } .panel-heading h1 { font-size: 21px; } .stats-row { gap: 6px; } .stats-row > div { padding: 9px; } .stats-row strong { font-size: 20px; } .panel-section { padding: 12px; } .project-title-row { display: block; } .title-actions { justify-content: flex-start; margin-top: 12px; } .enroll-row { align-items: flex-start; flex-wrap: wrap; } .enroll-row .btn { margin-left: 38px; } }
</style>
