<template>
  <div class="ftree">
    <div v-for="n in nodes" :key="n.path" class="ft-node">
      <div class="ft-row"
           :style="{ paddingLeft: (depth * 14 + 6) + 'px' }"
           :class="{ active: !n.isDir && n.path === activePath }"
           :title="n.isDir ? n.path : n.path + (n.size > 0 ? '\n' + fmtSize(n.size) : '')"
           @click="onNode(n)">
        <span class="ft-ch">{{ n.isDir ? (n.open ? '▾' : '▸') : '' }}</span>
        <span class="ft-ic">{{ iconOf(n) }}</span>
        <span class="ft-nm">{{ n.name }}</span>
        <span v-if="n.loading" class="ft-load">…</span>
      </div>

      <template v-if="n.isDir && n.open">
        <div v-if="n.loaded && n.children && n.children.length" class="ft-children">
          <FileTree :nodes="n.children" :depth="depth + 1" :active-path="activePath"
                    @open="p => $emit('open', p)" @toast="m => $emit('toast', m)" />
        </div>
        <div v-else-if="n.loaded && !n.loading" class="ft-empty"
             :style="{ paddingLeft: ((depth + 1) * 14 + 24) + 'px' }">（空）</div>
      </template>
    </div>
  </div>
</template>

<script>
import { ListLocalDir } from '../../wailsjs/go/main/App'

const EXT_ICON = {
  md: '📝', markdown: '📝',
  png: '🖼', jpg: '🖼', jpeg: '🖼', gif: '🖼', svg: '🖼', webp: '🖼', ico: '🖼',
  js: '⚡', ts: '⚡', mjs: '⚡', cjs: '⚡',
  py: '🐍', go: '🐹', java: '☕', c: '🅒', cpp: '🅒', h: '🅗',
  json: '🧾', yml: '🧾', yaml: '🧾', toml: '🧾', xml: '🧾',
  sh: '🖥', bash: '🖥', zsh: '🖥',
  zip: '🗜', gz: '🗜', tar: '🗜', rar: '🗜', '7z': '🗜'
}

export default {
  name: 'FileTree',
  components: {}, // 通过 name 自身递归
  props: {
    nodes: { type: Array, required: true },
    depth: { type: Number, default: 0 },
    activePath: { type: String, default: '' }
  },
  emits: ['open', 'toast'],
  methods: {
    fmtSize(n) {
      if (n < 1024) return n + ' B'
      if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
      return (n / 1024 / 1024).toFixed(1) + ' MB'
    },
    iconOf(n) {
      if (n.isDir) return n.open ? '📂' : '📁'
      if (n.name.startsWith('.')) return '⚙️'
      const ext = n.name.includes('.') ? n.name.split('.').pop().toLowerCase() : ''
      return EXT_ICON[ext] || '📄'
    },
    async onNode(n) {
      if (!n.isDir) {
        this.$emit('open', n)
        return
      }
      // 目录：首次点击懒加载子项，之后切换展开/收起
      if (!n.loaded) {
        n.loading = true
        try {
          const list = await ListLocalDir(n.path)
          n.children = (list || []).map(e => ({
            name: e.name, path: e.path, isDir: e.isDir,
            size: e.size, modTime: e.modTime,
            open: false, loaded: false, loading: false, children: []
          }))
          n.loaded = true
          n.open = true
        } catch (e) {
          this.$emit('toast', '读取目录失败: ' + e)
        }
        n.loading = false
      } else {
        n.open = !n.open
      }
    }
  }
}
</script>
