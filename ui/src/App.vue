<template>
  <main class="cloud-shell">
    <section v-if="editing" class="editor-page">
      <header class="editor-bar">
        <button class="back-link" type="button" @click="closeEditor"><icon-left />取消</button>
        <div>
          <h1>{{ form.metadata.name ? '编辑配置' : '新建配置' }}</h1>
        </div>
        <a-button type="primary" :loading="saving" @click="saveConfig">保存配置</a-button>
      </header>

      <div class="editor-canvas">
        <section class="identity-grid">
          <div class="field-block">
            <label>配置名称 <span class="required">*</span></label>
            <a-input v-model="form.spec.name" placeholder="例如：核心数据库连接" />
            <p>使用业务能识别的名称，不需要包含 namespace。</p>
          </div>
          <div class="field-block">
            <label>继承配置</label>
            <a-select
              v-model="inheritValue"
              allow-clear
              allow-search
              placeholder="搜索配置名称或版本"
              @change="loadInheritedPreview"
            >
              <a-option v-for="option in inheritOptions" :key="option.value" :value="option.value" :label="option.label" />
            </a-select>
            <p>继承项是最底层数据，当前配置中的同名项会覆盖它。</p>
          </div>
        </section>

        <section class="version-board">
          <div class="section-heading compact-heading">
            <h2>版本池</h2>
            <span class="section-note">配置项留空版本时，将作为所有环境共享的公共配置</span>
          </div>
          <a-input-tag
            v-model="formVersions"
            allow-clear
            placeholder="输入版本名并回车，例如 dev、staging、prod"
            @change="guardVersionPool"
          />
        </section>

        <section class="workbench">
          <div class="workbench-head">
            <div>
              <h2>配置工作表</h2>
              <p>{{ form.spec.items.length }} 条当前配置<span v-if="inheritedItems.length">，{{ inheritedItems.length }} 条继承配置</span></p>
            </div>
            <div class="workbench-actions">
              <a-button @click="openImport"><template #icon><icon-import /></template>从文本导入</a-button>
              <a-button type="primary" @click="addItem"><template #icon><icon-plus /></template>添加配置项</a-button>
            </div>
          </div>

          <div class="editor-version-strip">
            <span>查看版本</span>
            <div class="version-tabs">
              <button :class="{ active: editorVersionFilter === null }" type="button" @click="editorVersionFilter = null">全部</button>
              <button :class="{ active: editorVersionFilter === '' }" type="button" @click="editorVersionFilter = ''">公共配置</button>
              <button v-for="version in formVersions" :key="version" :class="{ active: editorVersionFilter === version }" type="button" @click="editorVersionFilter = version">{{ version }}</button>
            </div>
          </div>

          <div class="sheet-wrap">
            <a-table :data="editableItems" :pagination="false" row-key="_editKey" class="config-sheet" :scroll="{ x: 980 }">
              <template #columns>
                <a-table-column title="版本" :width="170">
                  <template #cell="{ record }">
                    <a-select
                      v-model="record.version"
                      allow-clear
                      allow-create
                      allow-search
                      placeholder="公共配置"
                      @change="addVersion(record.version)"
                    >
                      <a-option v-for="version in formVersions" :key="version" :value="version" />
                    </a-select>
                  </template>
                </a-table-column>
                <a-table-column title="配置名" :width="230">
                  <template #cell="{ record }"><a-input v-model="record.name" class="mono-input" placeholder="MYSQL_HOST" /></template>
                </a-table-column>
                <a-table-column title="值" :width="310">
                  <template #cell="{ record }"><a-textarea v-model="record.value" class="mono-input" :auto-size="{ minRows: 1, maxRows: 5 }" placeholder="允许为空" /></template>
                </a-table-column>
                <a-table-column title="备注" :width="230">
                  <template #cell="{ record }"><a-input v-model="record.remark" placeholder="说明用途或来源" /></template>
                </a-table-column>
                <a-table-column title="" :width="70" fixed="right">
                  <template #cell="{ record }"><a-button type="text" status="danger" @click="removeItem(record)"><icon-delete /></a-button></template>
                </a-table-column>
              </template>
            </a-table>
            <div v-if="!editableItems.length" class="sheet-empty">
              <icon-code-square class="empty-icon" />
              <strong>{{ form.spec.items.length ? '当前版本没有配置项' : '还没有当前配置项' }}</strong>
              <span>添加一行，或从现有环境变量文本批量导入。</span>
            </div>

            <div v-if="inheritValue" class="inherit-divider">
              <span>继承配置 · 只读底层</span>
              <a-spin v-if="inheritLoading" :size="16" />
            </div>
            <a-table v-if="inheritedItems.length" :data="inheritedItems" :pagination="false" row-key="_rowKey" class="inherited-sheet" :scroll="{ x: 980 }">
              <template #columns>
                <a-table-column title="版本" :width="170"><template #cell="{ record }"><span class="version-code">{{ record.version || '公共' }}</span></template></a-table-column>
                <a-table-column title="配置名" :width="230"><template #cell="{ record }"><code>{{ record.name }}</code></template></a-table-column>
                <a-table-column title="继承值" :width="310"><template #cell="{ record }"><code class="value-code">{{ record.value }}</code></template></a-table-column>
                <a-table-column title="来源" :width="230"><template #cell="{ record }"><span class="source-label">{{ record.sourceTitle || inheritedTitle }}</span></template></a-table-column>
                <a-table-column title="" :width="120" fixed="right"><template #cell="{ record }"><a-button size="small" @click="overrideInherited(record)">覆盖此项</a-button></template></a-table-column>
              </template>
            </a-table>
            <div v-else-if="inheritValue && !inheritLoading" class="sheet-empty inherited-empty">所选继承版本没有配置项</div>
          </div>
        </section>
      </div>
    </section>

    <template v-else-if="!current">
      <header class="page-hero">
        <div>
          <h1>配置中心</h1>
          <p>统一管理多应用共享配置、环境差异和部署策略。</p>
        </div>
        <div class="hero-actions">
          <a-button :loading="loading" @click="refresh"><template #icon><icon-refresh /></template>刷新</a-button>
          <a-button type="primary" @click="openCreate"><template #icon><icon-plus /></template>新建配置</a-button>
        </div>
      </header>

      <section class="ledger-panel">
        <div class="ledger-tools">
          <a-input v-model="searchText" allow-clear placeholder="搜索配置名称" class="search-box">
            <template #prefix><icon-search /></template>
          </a-input>
          <a-select v-model="updateFilter" class="filter-select">
            <a-option value="all">全部更新时间</a-option>
            <a-option value="recent">24 小时内更新</a-option>
          </a-select>
          <a-select v-model="deployFilter" class="filter-select">
            <a-option value="all">全部部署状态</a-option>
            <a-option value="pending">有待应用策略</a-option>
            <a-option value="failed">有失败策略</a-option>
            <a-option value="ready">全部已应用</a-option>
          </a-select>
          <span class="result-count">{{ filteredRows.length }} / {{ rows.length }} 套配置</span>
        </div>

        <a-alert v-if="loadError" type="error" :show-icon="true">{{ loadError }}</a-alert>
        <a-table v-else :data="filteredRows" :pagination="false" row-key="metadata.name" :loading="loading" class="ledger-table" :scroll="{ x: 1120 }">
          <template #columns>
            <a-table-column title="配置" :width="240" fixed="left">
              <template #cell="{ record }">
                <button class="config-name" type="button" @click="openDetail(record)">
                  <span>{{ record.spec.name }}</span>
                  <small>{{ record.metadata.name }}</small>
                </button>
              </template>
            </a-table-column>
            <a-table-column title="版本" :width="210">
              <template #cell="{ record }">
                <div class="tag-cluster">
                  <a-tag color="arcoblue">公共</a-tag>
                  <a-tag v-for="version in versionsOf([record]).slice(0, 3)" :key="version">{{ version }}</a-tag>
                  <a-tag v-if="versionsOf([record]).length > 3">+{{ versionsOf([record]).length - 3 }}</a-tag>
                </div>
              </template>
            </a-table-column>
            <a-table-column title="继承来源" :width="220"><template #cell="{ record }"><span :class="['lineage-text', { empty: !record.spec.inherit?.configName }]">{{ inheritLabel(record) }}</span></template></a-table-column>
            <a-table-column title="配置项" :width="100"><template #cell="{ record }"><strong class="data-number">{{ record.spec.items?.length || 0 }}</strong></template></a-table-column>
            <a-table-column title="部署状态" :width="170">
              <template #cell="{ record }"><a-tag :color="deployTagColor(deploySummary(record).tone)">{{ deploySummary(record).label }}</a-tag></template>
            </a-table-column>
            <a-table-column title="更新时间" :width="210">
              <template #cell="{ record }">
                <div :class="['update-time', { recent: record.recent }]">
                  <icon-sync v-if="record.recent" />
                  <span>{{ formatDate(record.status.updatedAt || record.metadata.creationTimestamp) }}</span>
                </div>
              </template>
            </a-table-column>
            <a-table-column title="" :width="150" fixed="right">
              <template #cell="{ record }">
                <div class="row-actions">
                  <a-button size="small" type="text" @click="openEdit(record)">编辑</a-button>
                  <a-popconfirm content="删除后无法恢复，确定继续？" @ok="remove(record)"><a-button size="small" type="text" status="danger">删除</a-button></a-popconfirm>
                </div>
              </template>
            </a-table-column>
          </template>
        </a-table>
        <div v-if="!loading && !loadError && !filteredRows.length" class="ledger-empty">
          <a-empty :description="rows.length ? '没有符合筛选条件的配置' : '暂无配置'">
            <template #extra><a-button v-if="!rows.length" type="primary" @click="openCreate">新建配置</a-button></template>
          </a-empty>
        </div>
      </section>
    </template>

    <template v-else>
      <header class="detail-header">
        <button class="back-link" type="button" @click="current = null"><icon-left />返回配置列表</button>
        <div class="detail-title-row">
          <div>
            <h1>{{ current.spec.name }}</h1>
            <p>{{ current.metadata.name }}</p>
          </div>
          <a-button type="primary" @click="openEdit(current)"><template #icon><icon-edit /></template>编辑配置</a-button>
        </div>

        <div class="flow-rail">
          <div class="flow-node source-node">
            <span class="node-icon"><icon-link /></span>
            <div><small>继承来源</small><strong>{{ inheritLabel(current) }}</strong></div>
          </div>
          <span class="rail-segment"><icon-right /></span>
          <div class="flow-node current-node">
            <span class="node-icon"><icon-code-square /></span>
            <div><small>当前配置</small><strong>{{ current.spec.items?.length || 0 }} 项 · {{ versionsOf([current]).length }} 个版本</strong></div>
          </div>
          <span class="rail-segment"><icon-right /></span>
          <div class="flow-node target-node">
            <span class="node-icon"><icon-apps /></span>
            <div><small>部署目标</small><strong>{{ current.spec.strategies?.length || 0 }} 个策略 · {{ detailPendingCount }} 个待应用</strong></div>
          </div>
        </div>
      </header>

      <section class="detail-panel">
        <div class="detail-tabs">
          <button :class="{ active: activeTab === 'data' }" type="button" @click="activeTab = 'data'">配置数据</button>
          <button :class="{ active: activeTab === 'deploy' }" type="button" @click="activeTab = 'deploy'">配置部署 <span>{{ current.spec.strategies?.length || 0 }}</span></button>
        </div>

        <div v-if="activeTab === 'data'" class="tab-workspace">
          <div class="data-toolbar">
            <div class="version-tabs">
              <button :class="{ active: versionFilter === null }" type="button" @click="setVersion(null)">全部</button>
              <button :class="{ active: versionFilter === '' }" type="button" @click="setVersion('')">公共配置</button>
              <button v-for="version in versionsOf([current])" :key="version" :class="{ active: versionFilter === version }" type="button" @click="setVersion(version)">{{ version }}</button>
            </div>
            <span>{{ resolvedItems.length }} 条解析结果</span>
          </div>
          <a-table :data="resolvedItems" :pagination="false" row-key="_rowKey" class="data-table" :loading="resolvedLoading" :scroll="{ x: 880 }">
            <template #columns>
              <a-table-column title="版本" :width="140"><template #cell="{ record }"><span class="version-code">{{ record.version || '公共' }}</span></template></a-table-column>
              <a-table-column title="配置名" :width="240"><template #cell="{ record }"><code>{{ record.name }}</code></template></a-table-column>
              <a-table-column title="值"><template #cell="{ record }"><code class="value-code">{{ record.value }}</code></template></a-table-column>
              <a-table-column title="备注" :width="220" data-index="remark" />
              <a-table-column title="来源" :width="180"><template #cell="{ record }"><span :class="['source-badge', record.source === 'inherit' ? 'inherited' : 'owned']">{{ record.source === 'inherit' ? `继承 · ${record.sourceTitle}` : '当前配置' }}</span></template></a-table-column>
            </template>
          </a-table>
          <div v-if="!resolvedLoading && !resolvedItems.length" class="inline-empty">当前筛选下没有配置项</div>
        </div>

        <div v-else class="tab-workspace">
          <div class="deployment-head">
            <div><h2>部署策略</h2><p>策略保存后不会立即生效，需要手动应用或开启自动部署。</p></div>
            <a-button type="primary" @click="openStrategy()"><template #icon><icon-plus /></template>新增部署策略</a-button>
          </div>
          <a-table v-if="current.spec.strategies?.length" :data="current.spec.strategies" :pagination="false" row-key="id" class="strategy-table" :scroll="{ x: 1080 }">
            <template #columns>
              <a-table-column title="部署应用" :width="280">
                <template #cell="{ record }">
                  <div class="target-cell"><span>{{ record.target.group || record.target.namespace }} / {{ record.target.name }}</span><small>{{ record.target.kind }} · {{ record.target.container }}</small></div>
                </template>
              </a-table-column>
              <a-table-column title="类型" :width="130"><template #cell="{ record }">{{ record.type === 'file' ? '配置文件' : '环境变量' }}</template></a-table-column>
              <a-table-column title="挂载路径" :width="180"><template #cell="{ record }"><code>{{ record.type === 'file' ? record.mountPath : '-' }}</code></template></a-table-column>
              <a-table-column title="配置版本" :width="130"><template #cell="{ record }">{{ record.lastSelectedVersion || '公共配置' }}</template></a-table-column>
              <a-table-column title="自动部署" :width="110"><template #cell="{ record }"><a-tag :color="record.autoDeploy ? 'green' : 'gray'">{{ record.autoDeploy ? '已开启' : '已关闭' }}</a-tag></template></a-table-column>
              <a-table-column title="状态" :width="160">
                <template #cell="{ record }">
                  <a-tooltip v-if="strategyFailed(record)" :content="lastApplyStatus(record)?.error || '应用失败'"><a-tag color="red">应用失败</a-tag></a-tooltip>
                  <a-tag v-else :color="isStale(record) ? 'orange' : 'green'">{{ isStale(record) ? '待应用' : '已应用' }}</a-tag>
                  <small v-if="record.autoDeploy && strategyFailed(record) && lastApplyStatus(record)?.nextRetryAt" class="retry-time">{{ formatDate(lastApplyStatus(record).nextRetryAt) }} 重试</small>
                </template>
              </a-table-column>
              <a-table-column title="操作" :width="190" fixed="right">
                <template #cell="{ record, rowIndex }">
                  <div class="strategy-actions">
                    <a-button size="mini" type="text" @click="openStrategy(record, rowIndex)">编辑</a-button>
                    <a-popconfirm content="确定删除该部署策略？" @ok="removeStrategy(rowIndex)"><a-button size="mini" type="text" status="danger">删除</a-button></a-popconfirm>
                    <a-button size="mini" type="primary" :disabled="!isStale(record)" @click="openApply(record)">应用</a-button>
                  </div>
                </template>
              </a-table-column>
            </template>
          </a-table>
          <div v-else class="ledger-empty compact-empty"><a-empty description="暂无部署策略"><template #extra><a-button type="primary" @click="openStrategy()">新增部署策略</a-button></template></a-empty></div>
        </div>
      </section>
    </template>

    <a-modal v-model:visible="importVisible" title="从文本导入配置项" width="780px" :ok-button-props="{ disabled: !quickPreview.length }" @ok="confirmImport">
      <div class="import-layout">
        <div class="field-block">
          <label>导入到版本</label>
          <a-select v-model="quickVersion" allow-clear allow-create allow-search placeholder="公共配置（留空）">
            <a-option v-for="version in formVersions" :key="version" :value="version" />
          </a-select>
          <p>可选择已有版本或直接输入新版本。</p>
        </div>
        <div class="field-block">
          <label>配置文本</label>
          <a-textarea v-model="quickText" class="import-textarea" :auto-size="{ minRows: 7, maxRows: 12 }" placeholder="MYSQL_HOST=mysql&#10;MYSQL_PORT=3306" />
          <p>每行一个 name=value；文本只用于解析，不会单独保存。</p>
        </div>
        <div class="import-preview">
          <div><strong>解析预览</strong><span>{{ quickPreview.length }} 条</span></div>
          <a-table v-if="quickPreview.length" :data="quickPreview.slice(0, 6)" :pagination="false" size="small">
            <template #columns><a-table-column title="版本"><template #cell="{ record }">{{ record.version || '公共' }}</template></a-table-column><a-table-column title="配置名" data-index="name" /><a-table-column title="值" data-index="value" /></template>
          </a-table>
          <span v-if="quickPreview.length > 6" class="preview-more">另有 {{ quickPreview.length - 6 }} 条将在确认后添加</span>
        </div>
      </div>
    </a-modal>

    <a-modal v-model:visible="strategyVisible" :title="strategyIndex > -1 ? '编辑部署策略' : '新增部署策略'" width="760px" @ok="saveStrategy">
      <a-form :model="strategyForm" layout="vertical">
        <a-form-item label="部署方式"><a-radio-group v-model="strategyForm.type" type="button"><a-radio value="env">环境变量</a-radio><a-radio value="file">配置文件</a-radio></a-radio-group></a-form-item>
        <div class="modal-grid">
          <a-form-item label="Namespace"><a-input v-model="targetNamespace" @press-enter="reloadTargets"><template #suffix><icon-refresh class="clickable" @click="reloadTargets" /></template></a-input></a-form-item>
          <a-form-item label="工作负载与容器"><a-select v-model="targetValue" allow-search placeholder="选择已部署应用的容器"><a-option v-for="target in targetOptions" :key="target.value" :value="target.value" :label="target.label" /></a-select></a-form-item>
        </div>
        <a-form-item v-if="strategyForm.type === 'file'" label="容器内挂载路径"><a-input v-model="strategyForm.mountPath" class="mono-input" placeholder="/app/config" /></a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:visible="applyVisible" title="应用部署策略" width="620px" :ok-loading="applying" @ok="doApply">
      <div class="apply-target"><span><icon-send /></span><div><small>部署应用</small><strong>{{ applyTargetLabel }}</strong></div></div>
      <a-form :model="applyForm" layout="vertical">
        <a-form-item label="配置版本"><a-select v-model="applyForm.version" allow-clear placeholder="公共配置（留空）"><a-option v-for="version in versionsOf([current])" :key="version" :value="version" /></a-select></a-form-item>
        <a-form-item><div class="switch-row"><div><strong>自动部署</strong><span>配置更新后自动执行该策略；失败时按退避时间重试。</span></div><a-switch v-model="applyForm.autoDeploy" /></div></a-form-item>
      </a-form>
    </a-modal>
  </main>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { formatDate, inheritOptionValue, isRecent, isStrategyStale, normalizeVersions, parseQuick, uid, versionsOf } from './utils'
import { applyStrategy, createConfig, deleteConfig, listConfigs, listTargets, resolveConfig, updateConfig } from './api'

const targetNamespace = ref(window?.$wujie?.props?.namespace || 'default')
const loading = ref(false)
const saving = ref(false)
const applying = ref(false)
const loadError = ref('')
const configs = ref([])
const current = ref(null)
const activeTab = ref('data')
const versionFilter = ref(null)
const resolvedItems = ref([])
const resolvedLoading = ref(false)
const editing = ref(false)
const editorReturn = ref('list')
const formVersions = ref([])
const inheritValue = ref('')
const inheritedItems = ref([])
const inheritedTitle = ref('')
const inheritLoading = ref(false)
const editorVersionFilter = ref(null)
const importVisible = ref(false)
const quickText = ref('')
const quickVersion = ref('')
const strategyVisible = ref(false)
const strategyIndex = ref(-1)
const targetValue = ref('')
const targets = ref([])
const applyVisible = ref(false)
const applyStrategyId = ref('')
const applyTargetLabel = ref('')
const searchText = ref('')
const updateFilter = ref('all')
const deployFilter = ref('all')

const emptyForm = () => ({ apiVersion: 'cloudconfig.w7.cc/v1alpha1', kind: 'CloudConfig', metadata: { name: '' }, spec: { name: '', versions: [], items: [], strategies: [] } })
const form = reactive(emptyForm())
const strategyForm = reactive({ id: '', type: 'env', target: { namespace: targetNamespace.value, kind: '', name: '', container: '', group: '' }, mountPath: '', autoDeploy: false, lastSelectedVersion: '' })
const applyForm = reactive({ version: '', autoDeploy: false })

const rows = computed(() => configs.value.map((item) => ({ ...item, recent: isRecent(item.status) })))
const filteredRows = computed(() => rows.value.filter((record) => {
  const query = searchText.value.trim().toLowerCase()
  if (query && !`${record.spec.name} ${record.metadata.name}`.toLowerCase().includes(query)) return false
  if (updateFilter.value === 'recent' && !record.recent) return false
  if (deployFilter.value !== 'all' && deploySummary(record).state !== deployFilter.value) return false
  return true
}))
const inheritOptions = computed(() => {
  const options = []
  configs.value.forEach((cfg) => {
    if (cfg.metadata.name === form.metadata.name) return
    options.push({ value: inheritOptionValue({ configName: cfg.metadata.name }), label: `${cfg.spec.name} / 公共配置` })
    versionsOf([cfg]).forEach((version) => options.push({ value: inheritOptionValue({ configName: cfg.metadata.name, version }), label: `${cfg.spec.name} / ${version}` }))
  })
  return options
})
const targetOptions = computed(() => targets.value.flatMap((target) => (target.containers || []).map((container) => {
  const value = JSON.stringify({ namespace: target.namespace, kind: target.kind, name: target.name, container, group: target.group || '' })
  return { value, label: `${target.group || target.namespace} / ${target.kind} / ${target.name} / ${container}` }
})))
const quickPreview = computed(() => parseQuick(quickText.value, quickVersion.value))
const detailPendingCount = computed(() => (current.value?.spec?.strategies || []).filter((strategy) => isStrategyStale(current.value, strategy.id, strategy)).length)
const editableItems = computed(() => editorVersionFilter.value === null
  ? form.spec.items
  : form.spec.items.filter((item) => (item.version || '') === editorVersionFilter.value))

async function refresh() {
  loading.value = true
  loadError.value = ''
  try {
    configs.value = await listConfigs()
    if (current.value) {
      current.value = configs.value.find((item) => item.metadata.name === current.value.metadata.name) || current.value
      await loadResolved()
    }
  } catch (error) {
    loadError.value = error.response?.data?.message || error.message || '配置列表加载失败'
  } finally {
    loading.value = false
  }
}

function inheritLabel(record) {
  const inherit = record?.spec?.inherit
  if (!inherit?.configName) return '未继承'
  const parent = configs.value.find((item) => item.metadata.name === inherit.configName)
  return `${parent?.spec?.name || inherit.configName} / ${inherit.version || '公共配置'}`
}

function deploySummary(record) {
  const strategies = record.spec?.strategies || []
  if (!strategies.length) return { state: 'none', tone: 'neutral', label: '未配置部署' }
  const failed = strategies.filter((strategy) => {
    const status = (record.status?.lastApplied || []).find((item) => item.strategyId === strategy.id)
    return status && !status.success && isStrategyStale(record, strategy.id, strategy)
  }).length
  if (failed) return { state: 'failed', tone: 'failed', label: `${failed} 个应用失败` }
  const pending = strategies.filter((strategy) => isStrategyStale(record, strategy.id, strategy)).length
  if (pending) return { state: 'pending', tone: 'pending', label: `${pending} 个待应用` }
  return { state: 'ready', tone: 'ready', label: '全部已应用' }
}

function deployTagColor(tone) {
  return { ready: 'green', pending: 'orange', failed: 'red', neutral: 'gray' }[tone] || 'gray'
}

function assignForm(data) {
  Object.assign(form, emptyForm(), JSON.parse(JSON.stringify(data || emptyForm())))
  delete form.metadata.namespace
  form.spec.items = (form.spec.items || []).map((item) => ({ ...item, _editKey: uid('item') }))
  form.spec.strategies = form.spec.strategies || []
  formVersions.value = versionsOf([form])
  form.spec.versions = [...formVersions.value]
  inheritValue.value = form.spec.inherit?.configName ? inheritOptionValue(form.spec.inherit) : ''
  inheritedItems.value = []
  editorVersionFilter.value = null
}

function openCreate() {
  editorReturn.value = current.value ? 'detail' : 'list'
  assignForm(emptyForm())
  editing.value = true
}

function openEdit(record) {
  editorReturn.value = current.value ? 'detail' : 'list'
  assignForm(record)
  editing.value = true
  loadInheritedPreview()
}

function closeEditor() {
  editing.value = false
  inheritedItems.value = []
}

async function loadInheritedPreview() {
  inheritedItems.value = []
  if (!inheritValue.value) return
  const inherit = JSON.parse(inheritValue.value)
  const parent = configs.value.find((item) => item.metadata.name === inherit.configName)
  inheritedTitle.value = `${parent?.spec?.name || inherit.configName} / ${inherit.version || '公共配置'}`
  inheritLoading.value = true
  try {
    const result = await resolveConfig(inherit.configName, inherit.version || '')
    inheritedItems.value = (result.items || []).map((item, index) => ({ ...item, _rowKey: `inherit:${item.name}:${index}` }))
  } catch {
    inheritedItems.value = []
  } finally {
    inheritLoading.value = false
  }
}

function guardVersionPool(values) {
  const normalized = normalizeVersions(values)
  const used = new Set((form.spec.items || []).map((item) => item.version).filter(Boolean))
  ;(form.spec.strategies || []).forEach((strategy) => strategy.lastSelectedVersion && used.add(strategy.lastSelectedVersion))
  const removedUsed = [...used].filter((version) => !normalized.includes(version))
  if (removedUsed.length) {
    formVersions.value = normalizeVersions([...normalized, ...removedUsed])
    Message.warning(`版本 ${removedUsed.join('、')} 正在使用，需先调整配置项或部署策略`)
    return
  }
  formVersions.value = normalized
}

function addVersion(version) {
  const normalized = String(version || '').trim()
  if (normalized && !formVersions.value.includes(normalized)) formVersions.value.push(normalized)
}

function addItem(item = {}) {
  form.spec.items.push({ version: editorVersionFilter.value || '', name: '', value: '', remark: '', ...item, _editKey: uid('item') })
}

function removeItem(record) {
  const index = form.spec.items.findIndex((item) => item._editKey === record._editKey)
  if (index > -1) form.spec.items.splice(index, 1)
}

function overrideInherited(record) {
  const targetVersion = editorVersionFilter.value || ''
  const existing = form.spec.items.find((item) => item.name === record.name && (item.version || '') === targetVersion)
  if (existing) {
    Message.info(`当前配置中已存在该${targetVersion || '公共'}配置项`)
    return
  }
  addItem({ version: targetVersion, name: record.name, value: record.value, remark: record.remark || '' })
  Message.success(`已添加 ${record.name} 的覆盖项`)
}

function openImport() {
  quickText.value = ''
  quickVersion.value = ''
  importVisible.value = true
}

function confirmImport() {
  addVersion(quickVersion.value)
  quickPreview.value.forEach((item) => addItem(item))
  Message.success(`已添加 ${quickPreview.value.length} 条配置项`)
  importVisible.value = false
  quickText.value = ''
}

async function saveConfig() {
  if (!form.spec.name.trim()) return Message.warning('请输入配置名称')
  const emptyIndex = form.spec.items.findIndex((item) => !item.name.trim())
  if (emptyIndex > -1) return Message.warning(`第 ${emptyIndex + 1} 条配置项缺少配置名`)
  saving.value = true
  try {
    form.spec.inherit = inheritValue.value ? JSON.parse(inheritValue.value) : null
    form.spec.versions = normalizeVersions(formVersions.value)
    const payload = JSON.parse(JSON.stringify(form))
    payload.spec.items = payload.spec.items.map(({ _editKey, ...item }) => item)
    const saved = form.metadata.name ? await updateConfig(form.metadata.name, payload) : await createConfig(payload)
    editing.value = false
    Message.success('配置已保存')
    await refresh()
    if (editorReturn.value === 'detail') {
      current.value = configs.value.find((item) => item.metadata.name === saved.metadata.name) || saved
      await loadResolved()
    }
  } finally {
    saving.value = false
  }
}

async function remove(record) {
  await deleteConfig(record.metadata.name)
  Message.success('配置已删除')
  await refresh()
}

async function openDetail(record) {
  current.value = record
  activeTab.value = 'data'
  versionFilter.value = null
  await loadResolved()
}

async function setVersion(version) {
  versionFilter.value = version
  await loadResolved()
}

async function loadResolved() {
  if (!current.value) return
  resolvedLoading.value = true
  try {
    const result = await resolveConfig(current.value.metadata.name, versionFilter.value)
    resolvedItems.value = (result.items || []).map((item, index) => ({ ...item, _rowKey: `${item.source || 'self'}:${item.sourceName || ''}:${item.version || ''}:${item.name}:${index}` }))
  } finally {
    resolvedLoading.value = false
  }
}

async function ensureTargets() { targets.value = await listTargets(targetNamespace.value) }
async function reloadTargets() { targetValue.value = ''; await ensureTargets() }

async function openStrategy(record = null, index = -1) {
  if (record?.target?.namespace) targetNamespace.value = record.target.namespace
  await ensureTargets()
  strategyIndex.value = index
  Object.assign(strategyForm, record ? JSON.parse(JSON.stringify(record)) : { id: uid('strategy'), type: 'env', target: { namespace: targetNamespace.value, kind: '', name: '', container: '', group: '' }, mountPath: '', autoDeploy: false, lastSelectedVersion: '' })
  targetValue.value = strategyForm.target?.name ? JSON.stringify(strategyForm.target) : ''
  strategyVisible.value = true
}

async function saveStrategy() {
  if (!targetValue.value) return Message.warning('请选择部署目标')
  Object.assign(strategyForm.target, JSON.parse(targetValue.value))
  if (strategyForm.type === 'file' && !strategyForm.mountPath.trim()) return Message.warning('请输入挂载路径')
  const strategies = current.value.spec.strategies || []
  const payload = JSON.parse(JSON.stringify(strategyForm))
  if (strategyIndex.value > -1) strategies.splice(strategyIndex.value, 1, payload)
  else strategies.push(payload)
  current.value.spec.strategies = strategies
  await updateConfig(current.value.metadata.name, current.value)
  strategyVisible.value = false
  Message.success('部署策略已保存')
  await refresh()
}

async function removeStrategy(index) {
  current.value.spec.strategies.splice(index, 1)
  await updateConfig(current.value.metadata.name, current.value)
  Message.success('部署策略已删除')
  await refresh()
}

function isStale(strategy) { return isStrategyStale(current.value, strategy.id, strategy) }
function lastApplyStatus(strategy) { return (current.value?.status?.lastApplied || []).find((item) => item.strategyId === strategy.id) || null }
function strategyFailed(strategy) { const status = lastApplyStatus(strategy); return !!status && !status.success && isStale(strategy) }

function openApply(strategy) {
  applyStrategyId.value = strategy.id
  applyForm.version = strategy.lastSelectedVersion || ''
  applyForm.autoDeploy = !!strategy.autoDeploy
  applyTargetLabel.value = `${strategy.target.namespace} / ${strategy.target.kind} / ${strategy.target.name} / ${strategy.target.container}`
  applyVisible.value = true
}

async function doApply() {
  applying.value = true
  try {
    await applyStrategy(current.value.metadata.name, applyStrategyId.value, applyForm)
    Message.success('配置已应用并触发工作负载重启')
    applyVisible.value = false
    await refresh()
  } catch {
    await refresh()
  } finally {
    applying.value = false
  }
}

onMounted(refresh)
</script>
