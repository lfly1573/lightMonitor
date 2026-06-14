<script setup lang="ts">
defineProps<{
  labels: {
    title: string
    username: string
    password: string
    submit: string
  }
}>()

const emit = defineEmits<{
  installed: []
}>()
</script>

<template>
  <section class="section">
    <div class="form-panel">
      <h2>{{ labels.title }}</h2>
      <label>
        <span>{{ labels.username }}</span>
        <input type="text" autocomplete="username" />
      </label>
      <label>
        <span>{{ labels.password }}</span>
        <input type="password" autocomplete="new-password" />
      </label>
      <button type="button" @click="emit('installed')">{{ labels.submit }}</button>
    </div>
  </section>
</template>
