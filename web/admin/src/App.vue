<script setup lang="ts">
import { computed, ref } from 'vue'
import InstallView from './views/InstallView.vue'
import OverviewView from './views/OverviewView.vue'
import { messages, type Locale } from './i18n/messages'

const locale = ref<Locale>('zh-CN')
const installed = ref(false)

const t = computed(() => messages[locale.value])
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark">LM</span>
        <span>lightMonitor</span>
      </div>
      <nav class="nav">
        <a class="nav-link active" href="#">{{ t.nav.overview }}</a>
        <a class="nav-link" href="#">{{ t.nav.groups }}</a>
        <a class="nav-link" href="#">{{ t.nav.alerts }}</a>
        <a class="nav-link" href="#">{{ t.nav.settings }}</a>
      </nav>
    </aside>

    <main class="content">
      <header class="topbar">
        <div>
          <h1>{{ t.title }}</h1>
          <p>{{ t.subtitle }}</p>
        </div>
        <select v-model="locale" class="locale-switch" aria-label="Language">
          <option value="zh-CN">中文</option>
          <option value="en-US">English</option>
        </select>
      </header>

      <InstallView v-if="!installed" :labels="t.install" @installed="installed = true" />
      <OverviewView v-else :labels="t.overview" />
    </main>
  </div>
</template>
