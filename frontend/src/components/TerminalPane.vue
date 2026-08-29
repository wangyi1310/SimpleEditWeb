<template>
  <div ref="host" class="term-host"></div>
</template>

<script>
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { EventsOn } from '../../wailsjs/runtime'
import { Write, Resize } from '../../wailsjs/go/main/SSHManager'

function b64ToBytes(b64) {
  const bin = atob(b64)
  const arr = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i)
  return arr
}

export default {
  name: 'TerminalPane',
  props: { sessionId: { type: String, required: true }, title: { type: String, default: '' } },
  emits: ['exit'],
  data() { return { term: null, fit: null, offData: null, offExit: null, ro: null } },
  methods: {
    focus() {
      if (this.term) {
        this.term.focus()
        this.fitTerm()
      }
    },
    fitTerm() {
      if (!this.fit || !this.$refs.host) return
      try {
        this.fit.fit()
        const t = this.term
        if (t && t.cols && t.rows) Resize(this.sessionId, t.cols, t.rows)
      } catch (e) { /* 容器不可见时忽略 */ }
    }
  },
  mounted() {
    this.term = new Terminal({
      fontSize: 13,
      fontFamily: '"SF Mono", Menlo, Consolas, monospace',
      cursorBlink: true,
      theme: {
        background: '#1e1e2e',
        foreground: '#cdd6f4',
        cursor: '#f5e0dc',
        selectionBackground: '#585b70'
      },
      scrollback: 5000,
      convertEol: false
    })
    this.fit = new FitAddon()
    this.term.loadAddon(this.fit)
    this.term.open(this.$refs.host)
    this.term.onData(d => { Write(this.sessionId, d).catch(() => {}) })

    this.offData = EventsOn('ssh:data:' + this.sessionId, b64 => {
      if (b64 && this.term) this.term.write(b64ToBytes(b64))
    })
    this.offExit = EventsOn('ssh:exit:' + this.sessionId, () => {
      if (this.term) this.term.write('\r\n\x1b[90m── 连接已关闭 ──\x1b[0m\r\n')
      this.$emit('exit', this.sessionId)
    })

    this.term.focus()
    this.$nextTick(() => {
      this.fitTerm()
      // 尺寸自适应
      this.ro = new ResizeObserver(() => this.fitTerm())
      this.ro.observe(this.$refs.host)
    })
  },
  activated() { this.$nextTick(() => this.fitTerm()) },
  beforeUnmount() {
    if (this.offData) this.offData()
    if (this.offExit) this.offExit()
    if (this.ro) this.ro.disconnect()
    if (this.term) this.term.dispose()
  }
}
</script>

<style scoped>
.term-host { padding: 6px 0 0 8px; }
</style>
