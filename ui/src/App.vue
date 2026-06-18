<template>
  <div class="page">
    <div class="toolbar">
      <div>
        <div class="title">配置中心</div>
        <div class="muted">统一管理多应用共享配置、环境差异配置和部署策略</div>
      </div>
      <a-space>
        <a-input v-model="namespace" placeholder="namespace" style="width: 180px" @press-enter="refresh" />
        <a-button @click="refresh"><template #icon><icon-refresh /></template></a-button>
        <a-button type="primary" @click="openCreate"><template #icon><icon-plus /></template>新建配置</a-button>
      </a-space>
    </div>

    <div v-if="!current" class="panel">
      <a-table :data="rows" :pagination="false" row-key="metadata.name" :loading="loading">
        <template #columns>
          <a-table-column title="名称">
            <template #cell="{ record }">
              <span class="link" @click="openDetail(record)">{{ record.spec.name }}</span>
              <icon-sync v-if="record.recent" class="danger" style="margin-left: 6px" />
            </template>
          </a-table-column>
          <a-table-column title="版本数" :width="110">
            <template #cell="{ record }">{{ record.versionCount }}</template>
          </a-table-column>
          <a-table-column title="配置项" :width="110">
            <template #cell="{ record }">{{ record.spec.items?.length || 0 }}</template>
          </a-table-column>
          <a-table-column title="继承配置" :width="220">
            <template #cell="{ record }">{{ inheritLabel(record) }}</template>
          </a-table-column>
          <a-table-column title="创建时间" :width="190">
            <template #cell="{ record }">{{ formatDate(record.status.createdAt || record.metadata.creationTimestamp) }}</template>
          </a-table-column>
          <a-table-column title="更新时间" :width="220">
            <template #cell="{ record }">
              <span :class="{ danger: record.recent }">{{ formatDate(record.status.updatedAt || record.metadata.creationTimestamp) }}</span>
              <a-tag v-if="record.recent" color="red" style="margin-left: 8px">更新</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="操作" :width="180">
            <template #cell="{ record }">
              <div class="table-actions">
                <a-button size="mini" @click="openEdit(record)">编辑</a-button>
                <a-popconfirm content="确定删除该配置？" @ok="remove(record)">
                  <a-button size="mini" status="danger">删除</a-button>
                </a-popconfirm>
              </div>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </div>

    <div v-else class="panel">
      <div class="toolbar">
        <div>
          <a-button type="text" @click="current = null"><template #icon><icon-left /></template>返回</a-button>
          <div class="title">{{ current.spec.name }}</div>
          <div class="muted">
            更新时间：
            <span :class="{ danger: isRecent(current.status) }">{{ formatDate(current.status.updatedAt || current.metadata.creationTimestamp) }}</span>
          </div>
        </div>
        <a-button type="primary" @click="openEdit(current)"><template #icon><icon-edit /></template>编辑配置</a-button>
      </div>

      <a-tabs v-model:active-key="activeTab">
        <a-tab-pane key="data" title="配置数据">
          <a-space wrap style="margin-bottom: 12px">
            <a-tag class="link" :color="versionFilter === '' ? 'arcoblue' : 'gray'" @click="setVersion('')">全部</a-tag>
            <a-tag v-for="v in versionsOf([current])" :key="v" class="link" :color="versionFilter === v ? 'arcoblue' : 'gray'" @click="setVersion(v)">{{ v }}</a-tag>
          </a-space>
          <a-table :data="resolvedItems" :pagination="false" row-key="name">
            <template #columns>
              <a-table-column title="version" :width="120">
                <template #cell="{ record }">{{ record.version || '公共' }}</template>
              </a-table-column>
              <a-table-column title="name" data-index="name" />
              <a-table-column title="value" data-index="value" />
              <a-table-column title="remark" data-index="remark" />
              <a-table-column title="来源" :width="160">
                <template #cell="{ record }">
                  <a-tag :color="record.source === 'inherit' ? 'arcoblue' : 'green'">{{ record.source === 'inherit' ? `继承 ${record.sourceTitle}` : '当前配置' }}</a-tag>
                </template>
              </a-table-column>
            </template>
          </a-table>
        </a-tab-pane>
        <a-tab-pane key="deploy" title="配置部署">
          <div class="row-tools">
            <a-button type="primary" @click="openStrategy()"><template #icon><icon-plus /></template>新增部署策略</a-button>
          </div>
          <a-table :data="current.spec.strategies || []" :pagination="false" row-key="id">
            <template #columns>
              <a-table-column title="部署应用">
                <template #cell="{ record }">{{ record.target.namespace }} / {{ record.target.kind }} / {{ record.target.name }} / {{ record.target.container }}</template>
              </a-table-column>
              <a-table-column title="类型" :width="120">
                <template #cell="{ record }">{{ record.type === 'file' ? '配置文件' : '环境变量' }}</template>
              </a-table-column>
              <a-table-column title="挂载路径" :width="180">
                <template #cell="{ record }">{{ record.type === 'file' ? record.mountPath : '-' }}</template>
              </a-table-column>
              <a-table-column title="自动部署" :width="110">
                <template #cell="{ record }">{{ record.autoDeploy ? '开启' : '关闭' }}</template>
              </a-table-column>
              <a-table-column title="状态" :width="100">
                <template #cell="{ record }">
                  <a-tag :color="isStale(record) ? 'red' : 'green'">{{ isStale(record) ? '待应用' : '已应用' }}</a-tag>
                </template>
              </a-table-column>
              <a-table-column title="操作" :width="230">
                <template #cell="{ record, rowIndex }">
                  <div class="table-actions">
                    <a-button size="mini" @click="openStrategy(record, rowIndex)">编辑</a-button>
                    <a-button size="mini" status="danger" @click="removeStrategy(rowIndex)">删除</a-button>
                    <a-button size="mini" type="primary" :disabled="!isStale(record)" @click="openApply(record)">应用</a-button>
                  </div>
                </template>
              </a-table-column>
            </template>
          </a-table>
        </a-tab-pane>
      </a-tabs>
    </div>

    <a-drawer :visible="formVisible" :width="1100" unmount-on-close @cancel="formVisible = false" @ok="saveConfig">
      <template #title>{{ form.metadata.name ? '编辑配置' : '新建配置' }}</template>
      <a-form :model="form" auto-label-width>
        <a-form-item label="名称" required><a-input v-model="form.spec.name" style="width: 420px" /></a-form-item>
        <a-form-item label="版本池"><a-input-tag v-model="formVersions" style="width: 520px" placeholder="输入版本后回车" /></a-form-item>
        <a-form-item label="继承配置">
          <a-select v-model="inheritValue" allow-clear allow-search style="width: 520px" placeholder="搜索并选择已有配置 + version">
            <a-option v-for="option in inheritOptions" :key="option.value" :value="option.value" :label="option.label" />
          </a-select>
        </a-form-item>
        <a-form-item label="文本快速配置">
          <div class="full">
            <a-space>
              <a-select v-model="quickVersion" allow-clear allow-search style="width: 180px" placeholder="导入版本">
                <a-option v-for="v in formVersions" :key="v" :value="v" />
              </a-select>
              <a-button type="primary" @click="importQuick">导入到表格</a-button>
            </a-space>
            <a-textarea v-model="quickText" style="margin-top: 8px" :auto-size="{ minRows: 4, maxRows: 8 }" placeholder="多行 key=value" />
          </div>
        </a-form-item>
        <a-form-item label="配置项" required>
          <div class="full">
            <div class="row-tools"><a-button @click="addItem"><template #icon><icon-plus /></template>添加配置项</a-button></div>
            <a-table :data="form.spec.items" :pagination="false" row-key="name">
              <template #columns>
                <a-table-column title="version" :width="160">
                  <template #cell="{ record }"><a-select v-model="record.version" allow-clear allow-create allow-search placeholder="公共"><a-option v-for="v in formVersions" :key="v" :value="v" /></a-select></template>
                </a-table-column>
                <a-table-column title="name" :width="220">
                  <template #cell="{ record }"><a-input v-model="record.name" /></template>
                </a-table-column>
                <a-table-column title="value">
                  <template #cell="{ record }"><a-textarea v-model="record.value" :auto-size="{ minRows: 1, maxRows: 4 }" /></template>
                </a-table-column>
                <a-table-column title="remark" :width="220">
                  <template #cell="{ record }"><a-input v-model="record.remark" /></template>
                </a-table-column>
                <a-table-column title="操作" :width="90">
                  <template #cell="{ rowIndex }"><a-button size="mini" status="danger" @click="form.spec.items.splice(rowIndex, 1)">删除</a-button></template>
                </a-table-column>
              </template>
            </a-table>
          </div>
        </a-form-item>
      </a-form>
    </a-drawer>

    <a-modal v-model:visible="strategyVisible" :title="strategyIndex > -1 ? '编辑部署策略' : '新增部署策略'" width="820px" @ok="saveStrategy">
      <a-form :model="strategyForm" auto-label-width>
        <a-form-item label="策略类型"><a-radio-group v-model="strategyForm.type"><a-radio value="env">环境变量类型</a-radio><a-radio value="file">配置文件类型</a-radio></a-radio-group></a-form-item>
        <a-form-item label="部署目标">
          <a-select v-model="targetValue" allow-search placeholder="选择应用容器" style="width: 620px">
            <a-option v-for="target in targetOptions" :key="target.value" :value="target.value" :label="target.label" />
          </a-select>
        </a-form-item>
        <a-form-item v-if="strategyForm.type === 'file'" label="挂载路径"><a-input v-model="strategyForm.mountPath" style="width: 420px" placeholder="/app/config" /></a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:visible="applyVisible" title="应用部署策略" width="680px" @ok="doApply">
      <a-form :model="applyForm" auto-label-width>
        <a-form-item label="部署应用">{{ applyTargetLabel }}</a-form-item>
        <a-form-item label="配置版本"><a-select v-model="applyForm.version" allow-clear style="width: 320px" placeholder="留空表示公共配置"><a-option v-for="v in versionsOf([current])" :key="v" :value="v" /></a-select></a-form-item>
        <a-form-item label="自动部署"><a-switch v-model="applyForm.autoDeploy" /></a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { appliedRevision, formatDate, isRecent, parseQuick, uid, versionsOf } from './utils'
import { applyStrategy, createConfig, deleteConfig, listConfigs, listTargets, resolveConfig, updateConfig } from './api'

const namespace = ref(window?.$wujie?.props?.namespace || 'default')
const loading = ref(false)
const configs = ref([])
const current = ref(null)
const activeTab = ref('data')
const versionFilter = ref('')
const resolvedItems = ref([])
const formVisible = ref(false)
const formVersions = ref([])
const inheritValue = ref('')
const quickText = ref('')
const quickVersion = ref('')
const strategyVisible = ref(false)
const strategyIndex = ref(-1)
const targetValue = ref('')
const targets = ref([])
const applyVisible = ref(false)
const applyStrategyId = ref('')
const applyTargetLabel = ref('')
const applyForm = reactive({ version: '', autoDeploy: false })

const emptyForm = () => ({ apiVersion: 'cloudconfig.w7.cc/v1alpha1', kind: 'CloudConfig', metadata: { namespace: namespace.value, name: '' }, spec: { name: '', items: [], strategies: [] } })
const form = reactive(emptyForm())
const strategyForm = reactive({ id: '', type: 'env', target: { namespace: namespace.value, kind: '', name: '', container: '', group: '' }, mountPath: '', autoDeploy: false, lastSelectedVersion: '' })

const rows = computed(() => configs.value.map((item) => ({ ...item, recent: isRecent(item.status), versionCount: versionsOf([item]).length })))
const inheritOptions = computed(() => {
  const options = []
  configs.value.forEach((cfg) => {
    if (cfg.metadata.name === form.metadata.name) return
    options.push({ value: JSON.stringify({ namespace: cfg.metadata.namespace, configName: cfg.metadata.name, version: '' }), label: `${cfg.spec.name} / 公共配置` })
    versionsOf([cfg]).forEach((version) => options.push({ value: JSON.stringify({ namespace: cfg.metadata.namespace, configName: cfg.metadata.name, version }), label: `${cfg.spec.name} / ${version}` }))
  })
  return options
})
const targetOptions = computed(() => {
  const options = []
  targets.value.forEach((target) => {
    ;(target.containers || []).forEach((container) => {
      const value = JSON.stringify({ namespace: target.namespace, kind: target.kind, name: target.name, container, group: target.group || '' })
      options.push({ value, label: `${target.namespace} / ${target.kind} / ${target.name} / ${container}` })
    })
  })
  return options
})

async function refresh() {
  loading.value = true
  try {
    configs.value = await listConfigs(namespace.value)
    if (current.value) {
      current.value = configs.value.find((item) => item.metadata.name === current.value.metadata.name) || current.value
      await loadResolved()
    }
  } finally {
    loading.value = false
  }
}

function inheritLabel(record) {
  const inherit = record.spec?.inherit
  if (!inherit?.configName) return '-'
  const parent = configs.value.find((item) => item.metadata.name === inherit.configName)
  return `${parent?.spec?.name || inherit.configName} / ${inherit.version || '公共配置'}`
}

function assignForm(data) {
  Object.assign(form, emptyForm(), JSON.parse(JSON.stringify(data || emptyForm())))
  form.metadata.namespace = form.metadata.namespace || namespace.value
  form.spec.items = form.spec.items || []
  form.spec.strategies = form.spec.strategies || []
  formVersions.value = versionsOf([form])
  inheritValue.value = form.spec.inherit?.configName ? JSON.stringify(form.spec.inherit) : ''
}

function openCreate() {
  assignForm(emptyForm())
  formVisible.value = true
}

function openEdit(record) {
  assignForm(record)
  formVisible.value = true
}

async function openDetail(record) {
  current.value = record
  activeTab.value = 'data'
  versionFilter.value = ''
  await loadResolved()
}

async function setVersion(version) {
  versionFilter.value = version
  await loadResolved()
}

async function loadResolved() {
  if (!current.value) return
  const result = await resolveConfig(current.value.metadata.namespace, current.value.metadata.name, versionFilter.value)
  resolvedItems.value = result.items || []
}

function addItem() {
  form.spec.items.push({ version: '', name: '', value: '', remark: '' })
}

function importQuick() {
  form.spec.items.push(...parseQuick(quickText.value, quickVersion.value))
  quickText.value = ''
}

async function saveConfig() {
  if (!form.spec.name) {
    Message.warning('请输入名称')
    return
  }
  form.spec.inherit = inheritValue.value ? JSON.parse(inheritValue.value) : null
  if (form.metadata.name) {
    await updateConfig(form.metadata.namespace, form.metadata.name, form)
  } else {
    await createConfig(form, namespace.value)
  }
  formVisible.value = false
  Message.success('保存成功')
  await refresh()
}

async function remove(record) {
  await deleteConfig(record.metadata.namespace, record.metadata.name)
  Message.success('删除成功')
  await refresh()
}

async function ensureTargets() {
  targets.value = await listTargets(namespace.value)
}

async function openStrategy(record = null, index = -1) {
  await ensureTargets()
  strategyIndex.value = index
  Object.assign(strategyForm, record ? JSON.parse(JSON.stringify(record)) : { id: uid('strategy'), type: 'env', target: { namespace: namespace.value, kind: '', name: '', container: '', group: '' }, mountPath: '', autoDeploy: false, lastSelectedVersion: '' })
  targetValue.value = strategyForm.target?.name ? JSON.stringify(strategyForm.target) : ''
  strategyVisible.value = true
}

async function saveStrategy() {
  if (!targetValue.value) {
    Message.warning('请选择部署目标')
    return
  }
  Object.assign(strategyForm.target, JSON.parse(targetValue.value))
  if (strategyForm.type === 'file' && !strategyForm.mountPath) {
    Message.warning('请输入挂载路径')
    return
  }
  const strategies = current.value.spec.strategies || []
  const payload = JSON.parse(JSON.stringify(strategyForm))
  if (strategyIndex.value > -1) strategies.splice(strategyIndex.value, 1, payload)
  else strategies.push(payload)
  current.value.spec.strategies = strategies
  await updateConfig(current.value.metadata.namespace, current.value.metadata.name, current.value)
  strategyVisible.value = false
  Message.success('策略已保存')
  await refresh()
}

async function removeStrategy(index) {
  current.value.spec.strategies.splice(index, 1)
  await updateConfig(current.value.metadata.namespace, current.value.metadata.name, current.value)
  Message.success('策略已删除')
  await refresh()
}

function isStale(strategy) {
  return appliedRevision(current.value, strategy.id) !== current.value.status?.revision
}

function openApply(strategy) {
  applyStrategyId.value = strategy.id
  applyForm.version = strategy.lastSelectedVersion || ''
  applyForm.autoDeploy = !!strategy.autoDeploy
  applyTargetLabel.value = `${strategy.target.namespace} / ${strategy.target.kind} / ${strategy.target.name} / ${strategy.target.container}`
  applyVisible.value = true
}

async function doApply() {
  await applyStrategy(current.value.metadata.namespace, current.value.metadata.name, applyStrategyId.value, applyForm)
  Message.success('应用成功')
  applyVisible.value = false
  await refresh()
}

onMounted(refresh)
</script>
