<template>
  <div class="editor-wrap" :class="{ 'with-preview': preview }" :ref="el => (wrapEl = el)">
    <div ref="host" class="cm-host" :style="preview ? { flex: '0 0 ' + pvPct + '%' } : {}"></div>
    <!-- 编辑区 / 预览区 分栏拖拽条 -->
    <div v-show="preview" class="pv-split" @mousedown="startPvResize"></div>
    <div ref="pv" v-show="preview" class="md-preview" @scroll="syncPvScroll"></div>
  </div>
</template>

<script>
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { searchKeymap, highlightSelectionMatches } from '@codemirror/search'
import { syntaxHighlighting, defaultHighlightStyle, bracketMatching, indentOnInput } from '@codemirror/language'
import { markdown } from '@codemirror/lang-markdown'
import { javascript } from '@codemirror/lang-javascript'
import { python } from '@codemirror/lang-python'
import { json } from '@codemirror/lang-json'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

function langFor(path) {
  const p = (path || '').toLowerCase()
  if (p.endsWith('.md') || p.endsWith('.markdown')) return markdown()
  if (p.endsWith('.js') || p.endsWith('.ts') || p.endsWith('.jsx') || p.endsWith('.mjs')) return javascript()
  if (p.endsWith('.json')) return json()
  if (p.endsWith('.py')) return python()
  return []
}

export default {
  name: 'EditorPane',
  emits: ['status', 'change'],
  props: {
    fontSize: { type: Number, default: 14 },
    wordWrap: { type: Boolean, default: true },
    preview: { type: Boolean, default: false }
  },
  data() {
    return { view: null, pvTimer: null, wrapEl: null, pvPct: parseFloat(localStorage.getItem('se.pvPct')) || 50 }
  },
  watch: {
    fontSize() { this.applyTheme() },
    wordWrap() { this.applyWrap() },
    preview() {
      if (this.preview) {
        this.$nextTick(() => { this.renderPreview(); this.syncEditorScroll() })
      }
    }
  },
  methods: {
    initDoc(content, path) {
      if (!this.view) return
      this.view.dispatch({
        changes: { from: 0, to: this.view.state.doc.length, insert: content || '' },
        effects: this.langComp.reconfigure(langFor(path))
      })
      if (this.preview) this.$nextTick(() => this.renderPreview())
    },
    getContent() {
      return this.view ? this.view.state.doc.toString() : ''
    },
    focus() { if (this.view) this.view.focus() },
    applyTheme() {
      if (!this.view) return
      this.view.dispatch({ effects: this.themeComp.reconfigure(this.mkTheme()) })
    },
    applyWrap() {
      if (!this.view) return
      this.view.dispatch({ effects: this.wrapComp.reconfigure(this.wordWrap ? EditorView.lineWrapping : []) })
    },
    mkTheme() {
      const px = this.fontSize
      return EditorView.theme({
        '&': { fontSize: px + 'px', height: '100%' },
        '.cm-content': { paddingBottom: '40px' },
        '.cm-gutters': { background: '#fafbfc', border: 'none', color: '#9aa1ac' }
      })
    },
    emitStatus() {
      if (!this.view) return
      const st = this.view.state
      const pos = st.selection.main.head
      const line = st.doc.lineAt(pos)
      const selLen = st.selection.main.length
      const chars = st.doc.length
      const totalLines = st.doc.lines
      this.$emit('status', {
        line: line.number,
        col: pos - line.from + 1,
        chars,
        lines: totalLines,
        selected: selLen
      })
    },
    // ===== Markdown 预览 =====
    // 拖动调整 编辑区/预览区 分栏比例（持久化）
    startPvResize(e) {
      const wrap = this.wrapEl
      if (!wrap) return
      e.preventDefault()
      const rect = wrap.getBoundingClientRect()
      const move = ev => {
        this.pvPct = Math.max(15, Math.min(85, (ev.clientX - rect.left) / rect.width * 100))
      }
      const up = () => {
        window.removeEventListener('mousemove', move)
        window.removeEventListener('mouseup', up)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
        localStorage.setItem('se.pvPct', String(this.pvPct))
        if (this.view) this.view.requestMeasure()
      }
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
      window.addEventListener('mousemove', move)
      window.addEventListener('mouseup', up)
    },
    renderPreview() {
      const el = this.$refs.pv
      if (!el) return
      const src = this.getContent() || ''
      let html = ''
      try { html = marked.parse(src) } catch (e) { html = '<p class="pv-err">渲染失败: ' + e + '</p>' }
      el.innerHTML = DOMPurify.sanitize(html)
    },
    // 编辑区滚动 → 预览区按比例跟随
    syncEditorScroll() {
      const el = this.$refs.pv
      if (!el || !this.view) return
      const sc = this.view.scrollDOM
      const ratio = sc.scrollTop / (sc.scrollHeight - sc.clientHeight || 1)
      el.scrollTop = ratio * (el.scrollHeight - el.clientHeight)
    },
    syncPvScroll() {
      const el = this.$refs.pv
      if (!el || !this.view) return
      const sc = this.view.scrollDOM
      const ratio = el.scrollTop / (el.scrollHeight - el.clientHeight || 1)
      sc.scrollTop = ratio * (sc.scrollHeight - sc.clientHeight)
    },
    onEditorScroll() {
      if (this.preview) this.syncEditorScroll()
    }
  },
  mounted() {
    this.themeComp = new Compartment()
    this.wrapComp = new Compartment()
    this.langComp = new Compartment()
    const state = EditorState.create({
      doc: '',
      extensions: [
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        history(),
        bracketMatching(),
        indentOnInput(),
        highlightSelectionMatches(),
        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
        keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap, indentWithTab]),
        this.themeComp.of(this.mkTheme()),
        this.wrapComp.of(this.wordWrap ? EditorView.lineWrapping : []),
        this.langComp.of([]),
        EditorView.updateListener.of(u => {
          if (u.docChanged) {
            this.$emit('change')
            if (this.preview) {
              clearTimeout(this.pvTimer)
              this.pvTimer = setTimeout(() => this.renderPreview(), 120)
            }
          }
          if (u.docChanged || u.selectionSet) this.emitStatus()
        })
      ]
    })
    this.view = new EditorView({ state, parent: this.$refs.host })
    this.view.scrollDOM.addEventListener('scroll', this.onEditorScroll)
    this.emitStatus()
    if (this.preview) this.$nextTick(() => this.renderPreview())
  },
  beforeUnmount() {
    if (this.pvTimer) clearTimeout(this.pvTimer)
    if (this.view) {
      this.view.scrollDOM.removeEventListener('scroll', this.onEditorScroll)
      this.view.destroy()
    }
  }
}
</script>

<style scoped>
.editor-wrap { flex: 1; min-width: 0; display: flex; flex-direction: row; height: 100%; }
.cm-host { flex: 1; min-width: 0; height: 100%; overflow: hidden; }
/* 编辑区/预览区分栏拖拽条 */
.pv-split { flex: none; width: 5px; cursor: col-resize; background: #e3e6ea; }
.pv-split:hover { background: var(--accent, #2f6fed); }

.md-preview {
  flex: 1; min-width: 0; height: 100%; overflow: auto;
  padding: 18px 26px 60px; font-size: 14px; line-height: 1.7;
  color: #24292f; background: #fff;
  scrollbar-width: thin;
}
.md-preview :deep(h1) { font-size: 1.7em; border-bottom: 1px solid #eaecef; padding-bottom: .3em; margin: .8em 0 .5em; }
.md-preview :deep(h2) { font-size: 1.4em; border-bottom: 1px solid #eaecef; padding-bottom: .3em; margin: .9em 0 .5em; }
.md-preview :deep(h3) { font-size: 1.2em; margin: .9em 0 .4em; }
.md-preview :deep(h4) { font-size: 1.05em; margin: .8em 0 .4em; }
.md-preview :deep(p) { margin: .5em 0; }
.md-preview :deep(a) { color: #0969da; text-decoration: none; }
.md-preview :deep(a:hover) { text-decoration: underline; }
.md-preview :deep(code) {
  background: rgba(175, 184, 193, .2); border-radius: 4px;
  padding: .15em .35em; font-size: .9em; font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.md-preview :deep(pre) {
  background: #f6f8fa; border-radius: 6px; padding: 12px 14px; overflow: auto;
  border: 1px solid #eaecef;
}
.md-preview :deep(pre code) { background: none; padding: 0; font-size: 13px; }
.md-preview :deep(blockquote) {
  margin: .6em 0; padding: .2em 1em; color: #57606a;
  border-left: 4px solid #d0d7de; background: #f6f8fa; border-radius: 0 4px 4px 0;
}
.md-preview :deep(ul), .md-preview :deep(ol) { padding-left: 1.8em; margin: .4em 0; }
.md-preview :deep(li) { margin: .15em 0; }
.md-preview :deep(table) { border-collapse: collapse; margin: .8em 0; display: block; overflow: auto; max-width: 100%; }
.md-preview :deep(th), .md-preview :deep(td) { border: 1px solid #d0d7de; padding: 6px 12px; }
.md-preview :deep(th) { background: #f6f8fa; font-weight: 600; }
.md-preview :deep(tr:nth-child(2n)) { background: #fafbfc; }
.md-preview :deep(hr) { border: none; border-top: 2px solid #eaecef; margin: 1.2em 0; }
.md-preview :deep(img) { max-width: 100%; }
.md-preview :deep(.pv-err) { color: #cf222e; }
.md-preview :deep(::selection) { background: #b6d7ff; }
</style>
