<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '@/services/api'
import { useJobPolling } from '@/composables/useJobPolling'

const route = useRoute()
const projectId = route.params.id

const project = ref(null)
const transcript = ref(null)
const errorMsg = ref('')
const actionError = ref('')

const downloadJob = useJobPolling()
const transcribeJob = useJobPolling()

async function loadProject() {
  try {
    const res = await api.getProject(projectId)
    project.value = res.data
  } catch (err) {
    errorMsg.value = 'Project tidak ditemukan'
  }
}

async function loadTranscript() {
  try {
    const res = await api.getTranscript(projectId)
    transcript.value = res.data
  } catch (err) {
    transcript.value = null
  }
}

async function handleDownload() {
  actionError.value = ''
  try {
    const res = await api.triggerDownload(projectId)
    downloadJob.startPolling(res.data.job_id, () => {
      loadProject()
    })
  } catch (err) {
    actionError.value = err.response?.data?.error || 'Gagal memulai download'
  }
}

async function handleTranscribe() {
  actionError.value = ''
  try {
    const res = await api.triggerTranscribe(projectId)
    transcribeJob.startPolling(res.data.job_id, () => {
      loadProject()
      loadTranscript()
    })
  } catch (err) {
    actionError.value = err.response?.data?.error || 'Gagal memulai transcribe'
  }
}

function downloadTranscriptFile(format) {
  window.open(api.downloadTranscriptUrl(projectId, format), '_blank')
}

onMounted(() => {
  loadProject()
  loadTranscript()
})

onUnmounted(() => {
  downloadJob.stopPolling()
  transcribeJob.stopPolling()
})
</script>

<template>
  <div class="max-w-3xl mx-auto px-4 py-10">
    <router-link to="/" class="text-sm text-blue-600 hover:underline mb-4 inline-block">&larr; Kembali</router-link>

    <div v-if="errorMsg" class="text-red-600">{{ errorMsg }}</div>

    <div v-else-if="project">
      <h1 class="text-xl font-bold text-gray-900 mb-1">Project Detail</h1>
      <p class="text-sm text-gray-500 mb-6 break-all">{{ project.youtube_url }}</p>

      <div class="bg-white border border-gray-200 rounded-xl p-6 mb-4">
        <p class="text-sm text-gray-700 mb-4">
          Status project: <span class="font-semibold">{{ project.status }}</span>
        </p>

        <!-- Step 1: Download -->
        <div class="mb-4 pb-4 border-b border-gray-100">
          <div class="flex items-center justify-between mb-2">
            <span class="text-sm font-medium text-gray-700">1. Download Video</span>
            <button
              v-if="project.status === 'pending' || project.status === 'failed'"
              @click="handleDownload"
              :disabled="downloadJob.isPolling.value"
              class="bg-blue-600 hover:bg-blue-700 disabled:bg-blue-300 text-white text-sm font-medium px-4 py-1.5 rounded-lg transition"
            >
              Mulai Download
            </button>
            <span v-else class="text-xs text-green-600 font-medium">✓ Selesai</span>
          </div>
          <div v-if="downloadJob.isPolling.value" class="w-full bg-gray-100 rounded-full h-2 mt-2">
            <div
              class="bg-blue-500 h-2 rounded-full transition-all"
              :style="{ width: downloadJob.jobProgress.value + '%' }"
            ></div>
          </div>
          <p v-if="downloadJob.jobError.value" class="text-red-600 text-xs mt-2">{{ downloadJob.jobError.value }}</p>
        </div>

        <!-- Step 2: Transcribe -->
        <div class="mb-2">
          <div class="flex items-center justify-between mb-2">
            <span class="text-sm font-medium text-gray-700">2. Transcribe</span>
            <button
              v-if="['downloaded', 'failed'].includes(project.status) && !transcript"
              @click="handleTranscribe"
              :disabled="transcribeJob.isPolling.value"
              class="bg-purple-600 hover:bg-purple-700 disabled:bg-purple-300 text-white text-sm font-medium px-4 py-1.5 rounded-lg transition"
            >
              Mulai Transcribe
            </button>
            <span v-else-if="transcript" class="text-xs text-green-600 font-medium">✓ Selesai</span>
            <span v-else class="text-xs text-gray-400">Menunggu download selesai</span>
          </div>
          <div v-if="transcribeJob.isPolling.value" class="w-full bg-gray-100 rounded-full h-2 mt-2">
            <div
              class="bg-purple-500 h-2 rounded-full transition-all"
              :style="{ width: transcribeJob.jobProgress.value + '%' }"
            ></div>
          </div>
          <p v-if="transcribeJob.jobError.value" class="text-red-600 text-xs mt-2">{{ transcribeJob.jobError.value }}</p>
        </div>

        <p v-if="actionError" class="text-red-600 text-sm mt-2">{{ actionError }}</p>
      </div>

      <!-- Transcript export -->
      <div v-if="transcript" class="bg-white border border-gray-200 rounded-xl p-6 mb-4">
        <p class="text-sm font-medium text-gray-700 mb-3">Transcript siap ({{ transcript.language }})</p>
        <div class="flex gap-2">
          <button
            @click="downloadTranscriptFile('txt')"
            class="border border-gray-300 hover:bg-gray-50 text-gray-700 text-sm font-medium px-4 py-1.5 rounded-lg transition"
          >
            Download .txt
          </button>
          <button
            @click="downloadTranscriptFile('json')"
            class="border border-gray-300 hover:bg-gray-50 text-gray-700 text-sm font-medium px-4 py-1.5 rounded-lg transition"
          >
            Download .json
          </button>
        </div>
        <p class="text-xs text-gray-400 mt-3">
          Bawa file .txt ini ke AI (ChatGPT/Claude), minta list momen penting dalam format JSON, lalu upload hasilnya di bawah ini.
        </p>
      </div>

      <!-- Placeholder untuk import clips (step berikutnya) -->
      <div v-if="transcript" class="bg-white border border-gray-200 rounded-xl p-6">
        <p class="text-sm text-gray-400">Form upload JSON clip candidates akan ditambahkan di step berikutnya.</p>
      </div>
    </div>
  </div>
</template>