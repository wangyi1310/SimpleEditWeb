<template>
  <div class="modal-mask" @mousedown.self="$emit('close')">
    <div class="modal">
      <h3>{{ conn.id ? '编辑连接' : '新建连接' }}</h3>
      <div class="form-row row2">
        <div>
          <label>名称</label>
          <input v-model="conn.name" placeholder="我的服务器" />
        </div>
        <div>
          <label>端口</label>
          <input v-model.number="conn.port" type="number" min="1" max="65535" />
        </div>
      </div>
      <div class="form-row">
        <label>主机地址</label>
        <input v-model="conn.host" placeholder="192.168.1.10 或 example.com" spellcheck="false" />
      </div>
      <div class="form-row">
        <label>用户名</label>
        <input v-model="conn.user" placeholder="root" spellcheck="false" />
      </div>
      <div class="form-row">
        <label>认证方式</label>
        <select v-model="conn.authType">
          <option value="password">密码</option>
          <option value="key">私钥文件</option>
        </select>
      </div>
      <div class="form-row" v-if="conn.authType === 'password'">
        <label>密码</label>
        <input v-model="conn.password" type="password" placeholder="留空表示不修改" v-if="conn.id" />
        <input v-model="conn.password" type="password" placeholder="登录密码" v-else />
      </div>
      <template v-else>
        <div class="form-row">
          <label>私钥路径（本机）</label>
          <input v-model="conn.keyPath" placeholder="~/.ssh/id_rsa" spellcheck="false" />
        </div>
        <div class="form-row">
          <label>私钥口令（可选）</label>
          <input v-model="conn.keyPassphrase" type="password" />
        </div>
      </template>
      <div class="form-row">
        <label>连接后自动执行</label>
        <input v-model="conn.autoCmd" placeholder="如: cd /var/www && ls" spellcheck="false" />
        <div class="field-hint">连接成功后将自动执行该命令（可留空）</div>
      </div>
      <div class="err">{{ error }}</div>
      <div class="form-actions">
        <button class="btn" @click="$emit('close')">取消</button>
        <button class="btn primary" @click="save">保存</button>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'ConnectionModal',
  props: { conn: { type: Object, required: true } },
  emits: ['close', 'saved'],
  data() { return { error: '' } },
  methods: {
    save() {
      this.error = ''
      if (!this.conn.host.trim()) { this.error = '主机地址不能为空'; return }
      if (!this.conn.user.trim()) { this.error = '用户名不能为空'; return }
      if (!this.conn.port) this.conn.port = 22
      if (!this.conn.name.trim()) this.conn.name = this.conn.host
      this.$emit('saved', { ...this.conn })
    }
  }
}
</script>
