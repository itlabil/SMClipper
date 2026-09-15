<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '@/services/api'

const router = useRouter()

const youtubeUrl = ref('')
const projects = ref([])
const loading = ref(false)
const errorMsg = ref('')

async function loadProjects() {
  try {
    const res = await api.listProjects()
    projects.value = res.data
  } catch (err) {
    errorMsg.value = 'Gagal memuat daftar project'
  }
}

async function handleCreateProject() {
  if (!youtubeUrl.value.trim()) {
    errorMsg.value = 'URL YouTube tidak boleh kosong'
    return
  }
  errorMsg.value = ''
  loading.value = true
  try {
    const res = await api.createProject(youtubeUrl.value.trim())
    youtubeUrl.value = ''
    router.push(`/projects/${res.data.id}`)
  } catch (err) {
    errorMsg.value = err.response?.data?.error || 'Gagal membuat project'
  } finally {
    loading.value = false
  }
}

function statusBadgeClass(status) {
  const map = {
    pending: 'bg-gray-200 text-gray-700',
    downloading: 'bg-blue-100 text-blue-700',
    downloaded: 'bg-blue-200 text-blue-800',
    transcribing: 'bg-purple-100 text-purple-700',
    transcribed: 'bg-purple-200 text-purple-800',
    ready: 'bg-green-100 text-green-700',
    failed: 'bg-red-100 text-red-700',
  }
  return map[status] || 'bg-gray-100 text-gray-600'
}

onMounted(loadProjects)
</script>

<template>
  <div class="max-w-3xl mx-auto px-4 py-10">
    <h1 class="text-2xl font-bold text-gray-900 mb-1">SMClipper</h1>
    <p class="text-gray-500 mb-8">Buat clip pendek dari video YouTube panjang</p>

    <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6 mb-10">
      <label class="block text-sm font-medium text-gray-700 mb-2">URL YouTube</label>
      <div class="flex gap-2">
        <input
          v-model="youtubeUrl"
          type="text"
          placeholder="https://www.youtube.com/watch?v=..."
          class="flex-1 border border-gray-300 rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @keyup.enter="handleCreateProject"
        />
        <button
          @click="handleCreateProject"
          :disabled="loading"
          class="bg-blue-600 hover:bg-blue-700 disabled:bg-blue-300 text-white font-medium px-5 py-2 rounded-lg transition"
        >
          {{ loading ? 'Membuat...' : 'Buat Project' }}
        </button>
      </div>
      <p v-if="errorMsg" class="text-red-600 text-sm mt-2">{{ errorMsg }}</p>
    </div>

    <h2 class="text-lg font-semibold text-gray-800 mb-3">Project Sebelumnya</h2>
    <div v-if="projects.length === 0" class="text-gray-400 text-sm">Belum ada project.</div>
    <div class="space-y-2">
      <router-link
        v-for="p in projects"
        :key="p.id"
        :to="`/projects/${p.id}`"
        class="block bg-white border border-gray-200 rounded-lg px-4 py-3 hover:border-blue-400 transition"
      >
        <div class="flex items-center justify-between">
          <div class="truncate">
            <p class="text-sm text-gray-800 truncate">{{ p.youtube_url }}</p>
            <p class="text-xs text-gray-400 mt-0.5">{{ new Date(p.created_at).toLocaleString() }}</p>
          </div>
          <span :class="['text-xs font-medium px-2 py-1 rounded-full whitespace-nowrap ml-3', statusBadgeClass(p.status)]">
            {{ p.status }}
          </span>
        </div>
      </router-link>
    </div>
  </div>
</template>