<script setup lang="ts">
defineProps<{
  labels: {
    title: string
    empty: string
  }
}>()
</script>

<template>
  <section class="section">
    <h2>{{ labels.title }}</h2>
    <p class="empty">{{ labels.empty }}</p>
  </section>
</template>
