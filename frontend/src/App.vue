<template>
  <div class="app-shell">
    <!-- 工具栏 -->
    <div class="toolbar">
      <span class="tb-title">SimpleEdit</span>
      <template v-if="mode === 'editor'">
        <button class="tb-btn" @click="newDoc" title="新建 ⌘N">新建</button>
        <button class="tb-btn" @click="openFile" title="打开 ⌘O">打开</button>
        <button class="tb-btn" @click="save" title="保存 ⌘S">保存</button>
        <button class="tb-btn" @click="saveAs" title="另存为 ⇧⌘S">另存为</button>
        <span class="tb-sep"></span>
        <button class="tb-btn" @click="fontSize = Math.max(10, fontSize - 1)" title="缩小 ⌘-">A⁻</button>
        <button class="tb-btn" @click="fontSize = 14" title="复位 ⌘0">{{ fontSize }}pt</button>
        <button class="tb-btn" @click="fontSize = Math.min(32, fontSize + 1)" title="放大 ⌘+">A⁺</button>
        <span class="tb-sep"></span>
        <button class="tb-btn" :class="{ active: wordWrap }" @click="wordWrap = !wordWrap">自动换行</button>
        <button class="tb-btn" :class="{ active: mdPreview }" @click="mdPreview = !mdPreview" title="Markdown 预览">MD 预览</button>
      </template>
      <template v-else>
        <button class="tb-btn" :class="{ active: sessionView === 'term' }" @click="sessionView = 'term'">终端</button>
        <button class="tb-btn" :class="{ active: sessionView === 'sftp' }" @click="sessionView = 'sftp'">SFTP 文件</button>
        <span class="tb-sep"></span>
        <button class="tb-btn" @click="splitPane('row')" title="竖分屏 ⌘D" :disabled="!activeTab">⬒ 分屏</button>
        <button class="tb-btn" @click="splitPane('col')" title="横分屏 ⇧⌘D" :disabled="!activeTab">⬓ 分屏</button>
      </template>
      <span class="spacer"></span>
      <button class="tb-btn" :class="{ active: mode === 'editor' }" @click="mode = 'editor'">📝 编辑器</button>
      <button class="tb-btn" :class="{ active: mode === 'ssh' }" @click="enterSsh">🖥 SSH</button>
    </div>

    <!-- 主体 -->
    <div class="app-main">
      <!-- 编辑器模式 -->
      <EditorPane
        v-show="mode === 'editor' && docReady"
        ref="editor"
        :font-size="fontSize"
        :word-wrap="wordWrap"
        :preview="mdPreview"
        @status="s => status = s"
        @change="doc.dirty = true"
      />
      <div v-if="mode === 'editor' && !docReady" class="empty-hint">
        <div style="font-size: 34px">📝</div>
        <div>按 <kbd>⌘O</kbd> 打开文件，<kbd>⌘N</kbd> 新建，或点击工具栏按钮</div>
      </div>

      <!-- SSH 模式 -->
      <div v-if="mode === 'ssh'" class="ssh-root" :ref="el => (sshRootEl = el)">
        <div class="ssh-side" v-show="!sshSideCollapsed" :style="{ width: sshSideW + 'px' }">
          <div class="ssh-side-head">
            <span>连接（{{ connections.length }}）</span>
            <button @click="toggleSide" title="收起侧栏">◀</button>
            <button @click="addConn" title="新建连接">＋</button>
          </div>
          <div class="conn-item" v-for="(c, i) in connections" :key="c.id"
               :class="{ active: activeConnId === c.id, dragging: connDragFrom === i }"
               @click="activeConnId = c.id"
               draggable="true"
               @dragstart="connDragFrom = i"
               @dragend="connDragFrom = null"
               @dragover.prevent
               @drop.prevent="moveConn(i)">
            <span class="drag-grip" title="拖动排序">⠿</span>
            <span>{{ c.authType === 'key' ? '🔑' : '🖥' }}</span>
            <div style="flex:1; min-width:0">
              <div class="c-name">{{ c.name }}</div>
              <div class="c-host" :title="c.user + '@' + c.host + (c.port && c.port !== 22 ? ':' + c.port : '')">{{ c.user }}@{{ c.host }}{{ c.port && c.port !== 22 ? ':' + c.port : '' }}</div>
            </div>
            <div class="c-act">
              <button @click.stop="connect(c)" title="连接">▶</button>
              <button @click.stop="duplicateConn(c)" title="复制配置">⧉</button>
              <button @click.stop="editConn(c)" title="编辑">✎</button>
              <button @click.stop="delConn(c)" title="删除">✕</button>
            </div>
          </div>
          <div v-if="connections.length === 0" class="sftp-empty" style="padding: 24px 10px">
            还没有连接<br />点右上角 ＋ 新建
          </div>
          <div class="ssh-side-foot">
            <div class="kbd-hint"><span>⌘1-9</span> 切换标签</div>
            <div class="kbd-hint"><span>⌘←→</span> 前后标签</div>
            <div class="kbd-hint"><span>⌘D / ⇧⌘D</span> 分屏</div>
            <div class="kbd-hint"><span>⌘[ ⌘]</span> 切换分屏</div>
            <div class="kbd-hint"><span>⌘W</span> 关闭标签</div>
          </div>
        </div>
        <!-- 侧栏宽度拖拽条 -->
        <div class="side-split" v-show="!sshSideCollapsed" @mousedown="startSideResize"></div>
        <!-- 收起后贴边的展开条（三角） -->
        <button class="side-rail" v-if="sshSideCollapsed" @click="toggleSide"
                title="展开连接列表">▶</button>
        <div class="ssh-main">
          <!-- 标签栏（可拖拽排序） -->
          <div class="session-tabs" v-if="tabs.length > 0">
            <div class="session-tab" v-for="(tab, i) in tabs" :key="tab.id"
                 :class="{ active: tab.id === activeTabId, dragging: dragFrom === i }"
                 @click="activeTabId = tab.id"
                 draggable="true"
                 @dragstart="dragFrom = i"
                 @dragend="dragFrom = null"
                 @dragover.prevent
                 @drop.prevent="moveTab(i)">
              <span class="dot" :class="tab.layout"></span>
              <span class="tname">{{ tab.name }}<em v-if="tab.panes.length > 1">×{{ tab.panes.length }}</em></span>
              <span class="x" @click.stop="duplicateTab(tab)" title="复制终端（同连接新开标签）">⧉</span>
              <span class="x" @click.stop="closeTab(tab)" title="关闭">✕</span>
            </div>
          </div>

          <!-- 所有标签的 pane 全部常驻渲染（v-show），保留每个终端的滚动历史 -->
          <div v-for="tab in tabs" :key="tab.id" class="tab-body"
               :style="{ display: (tab.id === activeTabId && sessionView === 'term') ? 'flex' : 'none' }">
            <div class="panes" :class="tab.layout === 'col' ? 'pane-col' : 'pane-row'"
                 :ref="el => setPanesEl(tab.id, el)">
              <template v-for="(p, pi) in tab.panes" :key="p">
                <div class="pane" :style="paneStyle(tab, pi)"
                     :class="{ focused: p === tab.activePane }"
                     @click="focusPane(tab, p)">
                  <div class="pane-tag">
                    <span>{{ tab.name }} · {{ pi + 1 }}</span>
                  </div>
                  <TerminalPane :ref="el => setPaneRef(p, el)" :session-id="p" :title="tab.name" @exit="onPaneExit" />
                </div>
                <!-- 可拖拽分割条：调整左右/上下分屏比例 -->
                <div v-if="pi < tab.panes.length - 1" class="pane-split"
                     :class="tab.layout === 'col' ? 'h' : 'v'"
                     @mousedown="startResize(tab, pi, $event)"></div>
              </template>
            </div>
          </div>

          <!-- SFTP 面板（挂在当前标签的聚焦分屏上） -->
          <div v-for="tab in tabs" :key="'s-' + tab.id" class="tab-body"
               :style="{ display: (tab.id === activeTabId && sessionView === 'sftp') ? 'flex' : 'none' }">
            <SftpPanel v-if="tab.id === activeTabId" :key="'f-' + tab.activePane"
                       :session-id="activePaneSessionOf(tab)"
                       @open-file="openRemoteFile"
                       @toast="toast" />
          </div>

          <div v-if="tabs.length === 0" class="ssh-placeholder">
            <div style="font-size: 34px">🖥</div>
            <div>左侧选择连接，点 ▶ 开始会话</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 状态栏 -->
    <div class="statusbar">
      <template v-if="mode === 'editor' && docReady">
        <span>行 {{ status.line }}，列 {{ status.col }}</span>
        <span>共 {{ status.lines }} 行</span>
        <span>{{ status.chars }} 字符</span>
        <span v-if="status.selected > 0">已选 {{ status.selected }}</span>
        <span class="spacer"></span>
        <span>{{ doc.encoding }}{{ doc.remote ? ' · 远程' : '' }}</span>
        <span>{{ fontSize }}pt</span>
      </template>
      <template v-else-if="mode === 'ssh' && activeTab">
        <span>{{ activeTab.name }}</span>
        <span v-if="activeTab.panes.length > 1">{{ panesOf(activeTab).findIndex(p => p === activeTab.activePane) + 1 }}/{{ activeTab.panes.length }} 分屏</span>
        <span>{{ sessionView === 'term' ? '终端' : 'SFTP' }}</span>
        <span class="spacer"></span>
        <span>{{ connections.length }} 个连接</span>
      </template>
      <template v-else>
        <span>就绪</span>
        <span class="spacer"></span>
      </template>
    </div>

    <!-- 弹窗 -->
    <ConnectionModal v-if="connModal" :conn="connModal" @close="connModal = null" @saved="onConnSaved" />

    <!-- Toast -->
    <div v-if="toastMsg" style="position: fixed; bottom: 40px; left: 50%; transform: translateX(-50%);
         background: #24292f; color: #fff; padding: 8px 18px; border-radius: 6px;
         font-size: 12.5px; z-index: 200; box-shadow: 0 4px 16px rgba(0,0,0,.3); max-width: 80vw;">
      {{ toastMsg }}
    </div>
  </div>
</template>

<script>
import EditorPane from './components/EditorPane.vue'
import TerminalPane from './components/TerminalPane.vue'
import SftpPanel from './components/SftpPanel.vue'
import ConnectionModal from './components/ConnectionModal.vue'
import {
  OpenTextFile, SaveTextFile, SaveAsTextFile,
  ListConnections, SaveConnection, DeleteConnection, ReorderConnections
} from '../wailsjs/go/main/App'
import { Connect, CloseSession, SftpWriteFile } from '../wailsjs/go/main/SSHManager'

function uid(prefix) {
  return prefix + Date.now().toString(36) + Math.floor(Math.random() * 1e6).toString(36)
}

export default {
  name: 'App',
  components: { EditorPane, TerminalPane, SftpPanel, ConnectionModal },
  data() {
    return {
      mode: 'editor',
      // 文档
      doc: { path: '', name: '', encoding: 'UTF-8', dirty: false, remote: null },
      docReady: false,
      status: { line: 1, col: 1, chars: 0, lines: 1, selected: 0 },
      mdPreview: false,
      // 设置（localStorage 持久化）
      fontSize: parseInt(localStorage.getItem('se.fontSize') || '14'),
      wordWrap: localStorage.getItem('se.wordWrap') !== '0',
      // SSH：tabs = 标签（连接），每个标签含多个 pane（分屏会话）
      connections: [],
      activeConnId: '',
      tabs: [],
      activeTabId: '',
      sessionView: 'term',
      dragFrom: null,
      connDragFrom: null,
      sshSideW: parseInt(localStorage.getItem('se.sshSideW2') || '280'),
      sshSideCollapsed: localStorage.getItem('se.sshSideCollapsed') === '1',
      sshRootEl: null,
      // 弹窗 / toast
      connModal: null,
      toastMsg: '',
      toastTimer: null
    }
  },
  computed: {
    activeTab() { return this.tabs.find(t => t.id === this.activeTabId) || null },
    activeSession() {
      const t = this.activeTab
      if (!t) return null
      return { id: this.activePaneSessionOf(t), name: t.name }
    }
  },
  watch: {
    fontSize(v) { localStorage.setItem('se.fontSize', String(v)) },
    wordWrap(v) { localStorage.setItem('se.wordWrap', v ? '1' : '0') },
    activeTabId() { if (this.sessionView === 'term') this.focusActivePane() },
    sessionView(v) { if (v === 'term') this.focusActivePane() }
  },
  methods: {
    toast(msg) {
      this.toastMsg = String(msg)
      clearTimeout(this.toastTimer)
      this.toastTimer = setTimeout(() => { this.toastMsg = '' }, 3200)
    },
    panesOf(tab) { return tab.panes },
    setPaneRef(sid, el) {
      if (!this._paneRefs) this._paneRefs = {}
      if (el) this._paneRefs[sid] = el
      else delete this._paneRefs[sid]
    },
    setPanesEl(tabId, el) {
      if (!this._panesEls) this._panesEls = {}
      if (el) this._panesEls[tabId] = el
      else delete this._panesEls[tabId]
    },
    // 分屏比例：paneSizes[i] 为第 i 个 pane 的百分比，最后一个 pane 自动占剩余
    paneStyle(tab, pi) {
      if (pi >= tab.panes.length - 1) return {} // 最后一个 pane 弹性占满剩余
      const sz = tab.paneSizes && tab.paneSizes[pi]
      if (!sz) return {}
      return { flex: '0 0 ' + sz + '%' }
    },
    startResize(tab, pi, e) {
      const panesEl = this._panesEls && this._panesEls[tab.id]
      if (!panesEl) return
      e.preventDefault()
      const rect = panesEl.getBoundingClientRect()
      const horizontal = tab.layout !== 'col' // row 布局：左右分，拖 X；col 布局：上下分，拖 Y
      const total = horizontal ? rect.width : rect.height
      const n = tab.panes.length
      const MIN = 10 // 每 pane 最小 10%
      if (!tab.paneSizes || tab.paneSizes.length !== n - 1) {
        tab.paneSizes = new Array(n - 1).fill(0)
      }
      const move = ev => {
        const pos = horizontal ? ev.clientX - rect.left : ev.clientY - rect.top
        let before = 0 // 前面 pane 已占的百分比
        for (let i = 0; i < pi; i++) before += tab.paneSizes[i] || 100 / n
        let size = (pos / total) * 100 - before
        // 限制当前 pane 和后续 pane 都不小于 MIN
        let max = 100 - before - MIN * (n - 1 - pi)
        size = Math.max(MIN, Math.min(max, size))
        tab.paneSizes.splice(pi, 1, size)
      }
      const up = () => {
        window.removeEventListener('mousemove', move)
        window.removeEventListener('mouseup', up)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
        // 拖完后让所有终端按新尺寸 refit
        this.$nextTick(() => {
          for (const sid of tab.panes) {
            const el = this._paneRefs && this._paneRefs[sid]
            if (el && el.fitTerm) el.fitTerm()
          }
        })
      }
      document.body.style.cursor = horizontal ? 'col-resize' : 'row-resize'
      document.body.style.userSelect = 'none'
      window.addEventListener('mousemove', move)
      window.addEventListener('mouseup', up)
    },
    focusActivePane() {
      const tab = this.activeTab
      if (!tab) return
      const sid = this.activePaneSessionOf(tab)
      const el = this._paneRefs && this._paneRefs[sid]
      if (el && el.focus) this.$nextTick(() => el.focus())
    },
    activePaneSessionOf(tab) {
      if (!tab || !tab.panes.length) return ''
      return tab.panes.includes(tab.activePane) ? tab.activePane : tab.panes[0]
    },

    // ===== 文件 =====
    async confirmDiscard() {
      if (this.doc.dirty && !confirm(`"${this.docTitle()}" 有未保存的修改，继续将丢失。确定？`)) return false
      return true
    },
    docTitle() {
      return this.doc.remote ? this.doc.name : (this.doc.name || '未命名')
    },
    newDoc() {
      if (!this.confirmDiscard()) return
      this.doc = { path: '', name: '未命名.txt', encoding: 'UTF-8', dirty: false, remote: null }
      this.mdPreview = false
      this.docReady = true
      this.$nextTick(() => {
        this.$refs.editor.initDoc('', '')
        this.$refs.editor.focus()
      })
    },
    async openFile() {
      if (!await this.confirmDiscard()) return
      try {
        const r = await OpenTextFile()
        if (!r) return
        this.doc = { path: r.path, name: r.path.split('/').pop(), encoding: r.encoding, dirty: false, remote: null }
        this.mdPreview = /\.md$|\.markdown$/i.test(r.path)
        this.docReady = true
        this.$nextTick(() => {
          this.$refs.editor.initDoc(r.content, r.path)
          this.$refs.editor.focus()
        })
      } catch (e) { this.toast('打开失败: ' + e) }
    },
    async save() {
      const content = this.$refs.editor ? this.$refs.editor.getContent() : ''
      try {
        if (this.doc.remote) {
          await SftpWriteFile(this.doc.remote.sessionId, this.doc.remote.path, content)
          this.doc.dirty = false
          this.toast('已保存到远程 ' + this.doc.remote.path)
        } else if (this.doc.path) {
          await SaveTextFile(this.doc.path, content, this.doc.encoding)
          this.doc.dirty = false
          this.toast('已保存')
        } else {
          await this.saveAs()
        }
      } catch (e) { this.toast('保存失败: ' + e) }
    },
    async saveAs() {
      const content = this.$refs.editor ? this.$refs.editor.getContent() : ''
      try {
        const p = await SaveAsTextFile(content, this.doc.encoding)
        if (p) {
          this.doc.path = p
          this.doc.name = p.split('/').pop()
          this.doc.dirty = false
          this.toast('已保存到 ' + p)
        }
      } catch (e) { this.toast('保存失败: ' + e) }
    },
    openRemoteFile(f) {
      if (!this.confirmDiscard()) return
      this.doc = {
        path: '', name: f.name + '（远程）', encoding: 'UTF-8', dirty: false,
        remote: { sessionId: f.sessionId, path: f.path }
      }
      this.mdPreview = /\.md$|\.markdown$/i.test(f.name)
      this.docReady = true
      this.mode = 'editor'
      this.$nextTick(() => {
        this.$refs.editor.initDoc(f.content, f.path)
        this.$refs.editor.focus()
      })
    },

    // ===== SSH：侧栏折叠 / 宽度 =====
    toggleSide() {
      this.sshSideCollapsed = !this.sshSideCollapsed
      localStorage.setItem('se.sshSideCollapsed', this.sshSideCollapsed ? '1' : '0')
      this.refitAllPanes()
      if (this.sessionView === 'term') this.focusActivePane()
    },
    // 布局变化（窗口/侧栏/分屏）后让所有终端按新尺寸重新适配
    refitAllPanes() {
      const doFit = () => {
        for (const tab of this.tabs) {
          for (const sid of tab.panes) {
            const el = this._paneRefs && this._paneRefs[sid]
            if (el && el.fitTerm) el.fitTerm()
          }
        }
      }
      // nextTick → 下一帧 → 120ms 兜底，确保布局稳定后再算行列数
      this.$nextTick(() => {
        doFit()
        requestAnimationFrame(() => {
          doFit()
          setTimeout(doFit, 120)
        })
      })
    },
    startSideResize(e) {
      const root = this.sshRootEl
      if (!root) return
      e.preventDefault()
      const rect = root.getBoundingClientRect()
      const move = ev => {
        this.sshSideW = Math.max(160, Math.min(520, ev.clientX - rect.left))
      }
      const up = () => {
        window.removeEventListener('mousemove', move)
        window.removeEventListener('mouseup', up)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
        localStorage.setItem('se.sshSideW2', String(this.sshSideW))
      }
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
      window.addEventListener('mousemove', move)
      window.addEventListener('mouseup', up)
    },
    enterSsh() {
      this.mode = 'ssh'
      this.refreshConns()
      if (this.sessionView === 'term') this.focusActivePane()
    },
    async refreshConns() {
      try {
        this.connections = (await ListConnections()) || []
      } catch (e) {
        this.connections = []
      }
    },
    addConn() {
      this.connModal = { id: '', name: '', host: '', port: 22, user: 'root', authType: 'password', password: '', keyPath: '', keyPassphrase: '', autoCmd: '' }
    },
    editConn(c) {
      this.connModal = { ...c }
    },
    // 复制连接配置：弹出编辑窗，全部字段带过去，改完保存就是一条新连接
    duplicateConn(c) {
      this.connModal = { ...c, id: '', name: c.name + ' 副本' }
    },
    // 拖拽排序连接列表，并持久化顺序
    async moveConn(to) {
      if (this.connDragFrom === null || this.connDragFrom === to) { this.connDragFrom = null; return }
      const c = this.connections.splice(this.connDragFrom, 1)[0]
      this.connections.splice(to, 0, c)
      this.connDragFrom = null
      try {
        await ReorderConnections(this.connections.map(x => x.id))
      } catch (e) { this.toast('顺序保存失败: ' + e) }
    },
    async onConnSaved(c) {
      try {
        await SaveConnection(c)
        this.connModal = null
        await this.refreshConns()
        this.toast('已保存连接「' + c.name + '」')
      } catch (e) { this.toast('保存失败: ' + e) }
    },
    async delConn(c) {
      if (!confirm(`删除连接「${c.name}」？`)) return
      try {
        await DeleteConnection(c.id)
        await this.refreshConns()
      } catch (e) { this.toast('删除失败: ' + e) }
    },

    // ===== SSH：标签与分屏 =====
    async connect(c) {
      try {
        const sid = await Connect(c)
        const tab = { id: uid('t'), connId: c.id, name: c.name, layout: 'row', panes: [sid], activePane: sid, paneSizes: [] }
        this.tabs.push(tab)
        this.activeTabId = tab.id
        this.sessionView = 'term'
        this.focusActivePane()
      } catch (e) {
        this.toast('连接失败: ' + e)
      }
    },
    async splitPane(dir) {
      const tab = this.activeTab
      if (!tab) return
      const conn = this.connections.find(c => c.id === tab.connId)
      if (!conn) { this.toast('找不到连接配置'); return }
      try {
        const sid = await Connect(conn)
        tab.panes.push(sid)
        tab.layout = dir
        tab.activePane = sid
        tab.paneSizes = [] // 重新等分
        this.focusActivePane()
      } catch (e) { this.toast('分屏失败: ' + e) }
    },
    focusPane(tab, p) {
      tab.activePane = p
      if (this.sessionView === 'term') this.focusActivePane()
    },
    // 复制终端：用同一连接配置新开一个标签（独立会话）
    async duplicateTab(tab) {
      const conn = this.connections.find(c => c.id === tab.connId)
      if (!conn) { this.toast('找不到连接配置'); return }
      await this.connect(conn)
    },
    async closeTab(tab) {
      const idx = this.tabs.indexOf(tab)
      if (idx < 0) return // 已被 onPaneExit 递归移除，直接返回防止 splice(-1) 误删末尾标签
      this.tabs.splice(idx, 1)
      if (this.activeTabId === tab.id) {
        this.activeTabId = this.tabs.length ? this.tabs[Math.min(idx, this.tabs.length - 1)].id : ''
      }
      // 先清空 panes 再逐个关会话：CloseSession 触发的 exit 事件回来时
      // onPaneExit 找不到 pane，就不会递归 closeTab 造成误删
      const panes = tab.panes.splice(0)
      for (const p of panes) { try { await CloseSession(p) } catch (e) { /* 忽略 */ } }
    },
    switchTab(delta) {
      if (!this.tabs.length) return
      const cur = this.tabs.findIndex(t => t.id === this.activeTabId)
      const next = (cur + delta + this.tabs.length) % this.tabs.length
      this.activeTabId = this.tabs[next].id
    },
    switchTabIndex(i) {
      if (i >= 0 && i < this.tabs.length) this.activeTabId = this.tabs[i].id
    },
    cyclePane() {
      const tab = this.activeTab
      if (!tab || tab.panes.length < 2) return
      const idx = tab.panes.indexOf(tab.activePane)
      tab.activePane = tab.panes[(idx + 1) % tab.panes.length]
      if (this.sessionView === 'term') this.focusActivePane()
    },
    moveTab(to) {
      if (this.dragFrom === null || this.dragFrom === to) { this.dragFrom = null; return }
      const t = this.tabs.splice(this.dragFrom, 1)[0]
      this.tabs.splice(to, 0, t)
      this.dragFrom = null
    },
    onPaneExit(sid) {
      for (const tab of this.tabs) {
        const idx = tab.panes.indexOf(sid)
        if (idx >= 0) {
          tab.panes.splice(idx, 1)
          tab.paneSizes = [] // pane 数量变了，重新等分
          if (tab.activePane === sid) tab.activePane = tab.panes[tab.panes.length - 1] || ''
          if (!tab.panes.length) this.closeTab(tab)
          return
        }
      }
    }
  },
  mounted() {
    const onKey = async e => {
      if (!(e.metaKey || e.ctrlKey)) return
      const k = e.key.toLowerCase()
      if (this.mode === 'editor') {
        if (k === 's') { e.preventDefault(); this.save() }
        else if (k === 'o') { e.preventDefault(); this.openFile() }
        else if (k === 'n') { e.preventDefault(); this.newDoc() }
        else if (k === '0') { e.preventDefault(); this.fontSize = 14 }
        return
      }
      // SSH 模式：iTerm2 式快捷键
      if (e.metaKey) {
        if (/^[1-9]$/.test(e.key)) { e.preventDefault(); this.switchTabIndex(parseInt(e.key) - 1) }
        else if (e.key === 'ArrowLeft') { e.preventDefault(); this.switchTab(-1) }
        else if (e.key === 'ArrowRight') { e.preventDefault(); this.switchTab(1) }
        else if (e.key === 'w') { e.preventDefault(); if (this.activeTab) this.closeTab(this.activeTab) }
        else if (e.key === 'd') { e.preventDefault(); this.splitPane(e.shiftKey ? 'col' : 'row') }
        else if (e.key === '[') { e.preventDefault(); this.cyclePane() }
        else if (e.key === ']') { e.preventDefault(); this.cyclePane() }
        else if (k === 'b') { e.preventDefault(); this.toggleSide() }
      }
    }
    window.addEventListener('keydown', onKey)
    // 窗口缩放后让终端按新尺寸重新适配（防抖）
    this._onWinResize = () => {
      clearTimeout(this._resizeTimer)
      this._resizeTimer = setTimeout(() => this.refitAllPanes(), 80)
    }
    window.addEventListener('resize', this._onWinResize)
    // 启动时直接加载已保存的连接
    this.refreshConns()
  },
  beforeUnmount() {
    if (this._onWinResize) window.removeEventListener('resize', this._onWinResize)
    clearTimeout(this._resizeTimer)
  }
}
</script>
