<template>
  <div class="sftp-root">
    <div class="sftp-bar">
      <button class="sftp-btn" @click="goUp" title="上一级">↑ 上级</button>
      <input class="sftp-path" v-model="inputPath" @keyup.enter="load(inputPath)" spellcheck="false" />
      <button class="sftp-btn" @click="load(inputPath)" title="刷新">刷新</button>
      <button class="sftp-btn" @click="mkdir" title="新建文件夹">+ 目录</button>
      <button class="sftp-btn primary" @click="upload" title="上传文件">⬆ 上传</button>
    </div>
    <div class="sftp-list">
      <div class="sftp-row header">
        <span class="f-icon"></span>
        <span class="f-name">名称</span>
        <span class="f-size">大小</span>
        <span class="f-time">修改时间</span>
        <span class="f-act"></span>
      </div>
      <div v-if="loading" class="sftp-empty">读取中…</div>
      <div v-else-if="entries.length === 0" class="sftp-empty">空目录</div>
      <div v-for="e in entries" :key="e.name" class="sftp-row" @dblclick="enter(e)">
        <span class="f-icon">{{ e.isDir ? '📁' : '📄' }}</span>
        <span class="f-name" :title="e.name">{{ e.name }}</span>
        <span class="f-size">{{ e.isDir ? '-' : fmtSize(e.size) }}</span>
        <span class="f-time">{{ e.modTime }}</span>
        <span class="f-act">
          <button v-if="!e.isDir" @click.stop="openFile(e)" title="在编辑器中打开">编辑</button>
          <button v-if="!e.isDir" @click.stop="download(e)" title="下载到本地">下载</button>
          <button class="del" @click.stop="remove(e)" title="删除">删除</button>
        </span>
      </div>
    </div>
  </div>
</template>

<script>
import { SftpList, SftpMkdir, SftpRemove, SftpDownload, SftpUpload, SftpReadFile, SftpWriteFile } from '../../wailsjs/go/main/SSHManager'

export default {
  name: 'SftpPanel',
  props: { sessionId: { type: String, required: true } },
  emits: ['open-file', 'save-file', 'toast'],
  data() {
    return { path: '', inputPath: '', entries: [], loading: false }
  },
  methods: {
    fmtSize(n) {
      if (n < 1024) return n + ' B'
      if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
      return (n / 1024 / 1024).toFixed(1) + ' MB'
    },
    join(dir, name) {
      if (dir === '/') return '/' + name
      return dir.replace(/\/$/, '') + '/' + name
    },
    async load(p) {
      if (!p || !p.startsWith('/')) p = '/'
      this.loading = true
      try {
        const list = await SftpList(this.sessionId, p)
        list.sort((a, b) => (b.isDir - a.isDir) || a.name.localeCompare(b.name))
        this.entries = list
        this.path = this.inputPath = p.replace(/\/+$/, '') || '/'
      } catch (e) {
        this.$emit('toast', '读取目录失败: ' + e)
      }
      this.loading = false
    },
    goUp() {
      if (!this.path || this.path === '/') return
      const parts = this.path.replace(/\/$/, '').split('/')
      parts.pop()
      this.load(parts.length <= 1 ? '/' : parts.join('/'))
    },
    enter(e) {
      if (!e.isDir) return
      this.load(this.join(this.path, e.name))
    },
    async mkdir() {
      const name = prompt('新目录名称：')
      if (!name) return
      try {
        await SftpMkdir(this.sessionId, this.join(this.path, name))
        this.load(this.path)
      } catch (e) { this.$emit('toast', '创建失败: ' + e) }
    },
    async remove(e) {
      if (!confirm(`确认删除 ${e.isDir ? '目录' : '文件'} "${e.name}"？`)) return
      try {
        await SftpRemove(this.sessionId, this.join(this.path, e.name), e.isDir)
        this.load(this.path)
      } catch (err) { this.$emit('toast', '删除失败: ' + err) }
    },
    async download(e) {
      try {
        const local = await SftpDownload(this.sessionId, this.join(this.path, e.name))
        if (local) this.$emit('toast', '已下载到: ' + local)
      } catch (err) { this.$emit('toast', '下载失败: ' + err) }
    },
    async upload() {
      try {
        const n = await SftpUpload(this.sessionId, this.path)
        if (n > 0) {
          this.$emit('toast', `已上传 ${n} 个文件`)
          this.load(this.path)
        }
      } catch (e) { this.$emit('toast', '上传失败: ' + e) }
    },
    async openFile(e) {
      try {
        const content = await SftpReadFile(this.sessionId, this.join(this.path, e.name))
        this.$emit('open-file', {
          path: this.join(this.path, e.name),
          name: e.name,
          content,
          sessionId: this.sessionId
        })
      } catch (err) { this.$emit('toast', '读取失败: ' + err) }
    },
    async saveRemote(path, content) {
      await SftpWriteFile(this.sessionId, path, content)
    }
  },
  async mounted() {
    // 默认从根目录开始，用户可自行输入路径
    await this.load('/')
  }
}
</script>
