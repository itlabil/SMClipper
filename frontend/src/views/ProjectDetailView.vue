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

const promptCopied = ref(false)

const aiPrompt = `Kamu akan menerima transcript sebuah video dalam format berikut:
[HH:MM:SS,mmm --> HH:MM:SS,mmm] teks ucapan

Tugasmu: identifikasi momen-momen paling menarik/penting dalam video ini yang berpotensi jadi video pendek (short/reels) berdurasi 30-90 detik. Pilih momen yang punya salah satu dari ciri berikut:
- Punya hook kuat di awal (menarik perhatian dalam 3 detik pertama)
- Mengandung insight, poin penting, atau informasi yang berdiri sendiri (tidak butuh konteks sebelumnya untuk dipahami)
- Momen emosional, lucu, kontroversial, atau mengejutkan
- Punya kesimpulan/punchline yang jelas di akhir

Aturan output:
1. Balas HANYA dengan JSON valid, tanpa teks lain, tanpa markdown code block, tanpa penjelasan tambahan.
2. Format timestamp WAJIB "HH:MM:SS" (tanpa milidetik, tanpa koma). Contoh: "00:02:15".
3. Setiap clip idealnya berdurasi antara 30 sampai 90 detik.
4. Urutkan clips berdasarkan waktu kemunculan di video (start_ms terkecil duluan).
5. "hook_score" adalah angka 1-10 yang menunjukkan seberapa kuat potensi momen ini untuk menarik perhatian di awal video pendek.
6. Gunakan bahasa yang sama dengan transcript untuk field "title" dan "reason".

Format JSON yang harus kamu hasilkan:
{
  "clips": [
    {
      "title": "Judul singkat dan menarik untuk clip ini",
      "start": "HH:MM:SS",
      "end": "HH:MM:SS",
      "reason": "Alasan singkat kenapa momen ini menarik dijadikan clip",
      "hook_score": 8
    }
  ]
}

Simpan / buatkan dalam bentuk file Transcribe.json

Berikut transcript-nya (atau lihat file terlampir):
[TEMPEL/LAMPIRKAN TRANSCRIPT DI SINI]`

function copyPrompt() {
  navigator.clipboard.writeText(aiPrompt).then(() => {
    promptCopied.value = true
    setTimeout(() => {
      promptCopied.value = false
    }, 2000)
  })
}

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
      <h1 class="text-xl font-bold text-gray-900 mb-1">{{ project.title || 'Project Detail' }}</h1>
      <p class="text-sm text-gray-500 mb-1" v-if="project.channel_name">{{ project.channel_name }}</p>
      <p class="text-xs text-gray-400 mb-6 break-all">{{ project.youtube_url }}</p>

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
        <div class="mt-4 pt-4 border-t border-gray-100">
          <div class="flex items-center justify-between mb-2">
            <p class="text-sm font-medium text-gray-700">Prompt untuk AI</p>
            <button
              @click="copyPrompt"
              class="flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-lg border transition"
              :class="promptCopied
                ? 'bg-green-50 border-green-300 text-green-700'
                : 'border-gray-300 text-gray-600 hover:bg-gray-50'"
            >
              <svg v-if="!promptCopied" xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="9" y="9" width="13" height="13" rx="2" />
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M20 6 9 17l-5-5" />
              </svg>
              {{ promptCopied ? 'Tersalin!' : 'Copy Prompt' }}
            </button>
          </div>
          <textarea
            readonly
            rows="6"
            class="w-full text-xs font-mono text-gray-600 bg-gray-50 border border-gray-200 rounded-lg p-3 resize-none focus:outline-none"
            :value="aiPrompt"
          ></textarea>
          <p class="text-xs text-gray-400 mt-2">
            Copy prompt ini, tempel ke ChatGPT/Claude, lalu lampirkan atau paste isi file .txt transcript yang sudah kamu download.
          </p>
        </div>
      </div>

      <!-- Placeholder untuk import clips (step berikutnya) -->
      <div v-if="transcript" class="bg-white border border-gray-200 rounded-xl p-6 text-center">
        <router-link
          :to="`/projects/${projectId}/review`"
          class="inline-block bg-green-600 hover:bg-green-700 text-white text-sm font-medium px-5 py-2.5 rounded-lg transition"
        >
          Lanjut ke Review & Edit Clip &rarr;
        </router-link>
      </div>
    </div>
  </div>
</template>