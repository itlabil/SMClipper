<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '@/services/api'
import { useJobPolling } from '@/composables/useJobPolling'

const route = useRoute()
const router = useRouter()
const projectId = route.params.id

const project = ref(null)
const clips = ref([])
const selectedClipId = ref(null)
const videoRef = ref(null)

const errorMsg = ref('')
const importError = ref('')
const jsonInput = ref('')
const importing = ref(false)

const editingClipId = ref(null)
const editTitle = ref('')
const editStart = ref('')
const editEnd = ref('')

async function loadProject() {
  try {
    const res = await api.getProject(projectId)
    project.value = res.data
  } catch (err) {
    errorMsg.value = 'Project tidak ditemukan'
  }
}

async function loadClips() {
  try {
    const res = await api.listClips(projectId)
    clips.value = res.data
  } catch (err) {
    clips.value = []
  }
}

function msToTimeLabel(ms) {
  const totalSec = Math.floor(ms / 1000)
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = totalSec % 60
  if (h > 0) {
    return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  }
  return `${m}:${String(s).padStart(2, '0')}`
}

function durationLabel(clip) {
  const sec = Math.round((clip.end_ms - clip.start_ms) / 1000)
  return `${sec}s`
}

function selectClip(clip) {
  selectedClipId.value = clip.id
  if (videoRef.value) {
    videoRef.value.currentTime = clip.start_ms / 1000
    videoRef.value.play()
  }
}

async function handleImportJSON() {
  importError.value = ''
  if (!jsonInput.value.trim()) {
    importError.value = 'JSON tidak boleh kosong'
    return
  }

  let parsed
  try {
    parsed = JSON.parse(jsonInput.value)
  } catch (err) {
    importError.value = 'JSON tidak valid: ' + err.message
    return
  }

  importing.value = true
  try {
    await api.importClips(projectId, parsed)
    jsonInput.value = ''
    await loadClips()
  } catch (err) {
    importError.value = err.response?.data?.error || 'Gagal import clips'
  } finally {
    importing.value = false
  }
}

function handleFileUpload(event) {
  const file = event.target.files[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = (e) => {
    jsonInput.value = e.target.result
  }
  reader.readAsText(file)
}

async function handleDeleteClip(clip) {
  if (!confirm(`Hapus clip "${clip.title}"?`)) return
  try {
    await api.deleteClip(clip.id)
    await loadClips()
  } catch (err) {
    alert('Gagal menghapus clip')
  }
}

function startEdit(clip) {
  editingClipId.value = clip.id
  editTitle.value = clip.title
  editStart.value = msToTimeLabel(clip.start_ms)
  editEnd.value = msToTimeLabel(clip.end_ms)
}

function cancelEdit() {
  editingClipId.value = null
}

function timeLabelToMs(label) {
  const parts = label.split(':').map(Number)
  let h = 0, m = 0, s = 0
  if (parts.length === 3) [h, m, s] = parts
  else if (parts.length === 2) [m, s] = parts
  else if (parts.length === 1) [s] = parts
  return (h * 3600 + m * 60 + s) * 1000
}

async function saveEdit(clip) {
  try {
    await api.updateClip(clip.id, {
      title: editTitle.value,
      start_ms: timeLabelToMs(editStart.value),
      end_ms: timeLabelToMs(editEnd.value),
    })
    editingClipId.value = null
    await loadClips()
  } catch (err) {
    alert('Gagal menyimpan perubahan: ' + (err.response?.data?.error || err.message))
  }
}

const addingManual = ref(false)
const manualTitle = ref('')
const manualStart = ref('')
const manualEnd = ref('')

// --- Layout & Crop Editor ---
const layoutClipId = ref(null)
const layoutType = ref('single_crop')
const cropRegions = ref([])
const subtitleEnabled = ref(true)
const overlayRef = ref(null)
const videoNativeSize = ref({ w: 0, h: 0 })

const drawing = ref(false)
const drawStart = ref({ x: 0, y: 0 })
const drawCurrent = ref({ x: 0, y: 0 })

const savingConfig = ref(false)
const configSaved = ref(false)
const subtitleGenerating = ref(false)
const subtitleGenerated = ref(false)
const renderedClip = ref(null)
const renderJob = useJobPolling()

// --- Segment Editor ---
const useSegments = ref(false)
const segments = ref([])
const activeSegmentId = ref(null)
const detectingSegments = ref(false)

function segmentDuration(seg) {
  return Math.round((seg.end_ms - seg.start_ms) / 1000)
}

async function handleDetectSegments() {
  detectingSegments.value = true
  try {
    const res = await api.detectSegments(layoutClipId.value)
    segments.value = res.data
    useSegments.value = true
    if (segments.value.length > 0) {
      selectSegmentForEditing(segments.value[0])
    }
  } catch (err) {
    alert('Gagal mendeteksi segment: ' + (err.response?.data?.error || err.message))
  } finally {
    detectingSegments.value = false
  }
}

async function loadSegmentsForClip(clipId) {
  try {
    const res = await api.listSegments(clipId)
    segments.value = res.data
    useSegments.value = res.data.length > 0
  } catch (err) {
    segments.value = []
    useSegments.value = false
  }
}

function selectSegmentForEditing(seg) {
  activeSegmentId.value = seg.id
  layoutType.value = seg.layout_type
  cropRegions.value = seg.crop_regions || []

  if (videoRef.value) {
    // seek preview ke awal segmen (relatif terhadap clip)
    videoRef.value.currentTime = (layoutClip.value.start_ms + seg.start_ms) / 1000
  }
}

async function saveActiveSegment() {
  const seg = segments.value.find((s) => s.id === activeSegmentId.value)
  if (!seg) return

  const required = layoutType.value === 'split_top_bottom' ? 2 : 1
  if (cropRegions.value.length < required) {
    alert(`Gambar dulu ${required} area crop untuk segmen ini`)
    return
  }

  try {
    const res = await api.updateSegment(seg.id, {
      layout_type: layoutType.value,
      crop_regions: cropRegions.value,
    })
    const idx = segments.value.findIndex((s) => s.id === seg.id)
    segments.value[idx] = res.data
    configSaved.value = true
    setTimeout(() => { configSaved.value = false }, 2000)
  } catch (err) {
    alert('Gagal menyimpan segmen: ' + (err.response?.data?.error || err.message))
  }
}

function allSegmentsConfigured() {
  return segments.value.every((s) => {
    const required = s.layout_type === 'split_top_bottom' ? 2 : 1
    return (s.crop_regions || []).length >= required
  })
}

const layoutClip = computed(() => clips.value.find((c) => c.id === layoutClipId.value))

function openLayoutEditor(clip) {
  selectClip(clip)
  layoutClipId.value = clip.id
  cropRegions.value = []
  layoutType.value = 'single_crop'
  subtitleEnabled.value = true
  subtitleGenerated.value = false
  renderedClip.value = null
  activeSegmentId.value = null
  segments.value = []
  useSegments.value = false

  api.getRenderConfig(clip.id)
    .then((res) => {
      layoutType.value = res.data.layout_type
      cropRegions.value = res.data.crop_regions || []
      subtitleEnabled.value = res.data.subtitle_enabled
    })
    .catch(() => {})

  api.getRenderedClip(clip.id)
    .then((res) => { renderedClip.value = res.data })
    .catch(() => {})

  loadSegmentsForClip(clip.id)
}

function closeLayoutEditor() {
  layoutClipId.value = null
}

function onVideoLoadedMeta(e) {
  videoNativeSize.value = { w: e.target.videoWidth, h: e.target.videoHeight }
}

function startDrawing(e) {
  if (!layoutClipId.value || !overlayRef.value) return
  const rect = overlayRef.value.getBoundingClientRect()
  drawing.value = true
  drawStart.value = { x: e.clientX - rect.left, y: e.clientY - rect.top }
  drawCurrent.value = { ...drawStart.value }
}

function moveDrawing(e) {
  if (!drawing.value || !overlayRef.value) return
  const rect = overlayRef.value.getBoundingClientRect()
  drawCurrent.value = { x: e.clientX - rect.left, y: e.clientY - rect.top }
}

function endDrawing() {
  if (!drawing.value || !overlayRef.value) return
  drawing.value = false

  const rect = overlayRef.value.getBoundingClientRect()
  const x1 = Math.min(drawStart.value.x, drawCurrent.value.x)
  const y1 = Math.min(drawStart.value.y, drawCurrent.value.y)
  const x2 = Math.max(drawStart.value.x, drawCurrent.value.x)
  const y2 = Math.max(drawStart.value.y, drawCurrent.value.y)

  if (x2 - x1 < 10 || y2 - y1 < 10) return

  const scaleX = videoNativeSize.value.w / rect.width
  const scaleY = videoNativeSize.value.h / rect.height

  const region = {
    x: Math.round(x1 * scaleX),
    y: Math.round(y1 * scaleY),
    w: Math.round((x2 - x1) * scaleX),
    h: Math.round((y2 - y1) * scaleY),
  }

  if (layoutType.value === 'split_top_bottom') {
    if (cropRegions.value.length === 0) {
      cropRegions.value = [{ ...region, speaker: 'A' }]
    } else if (cropRegions.value.length === 1) {
      cropRegions.value = [...cropRegions.value, { ...region, speaker: 'B' }]
    } else {
      cropRegions.value = [{ ...region, speaker: 'A' }]
    }
  } else {
    cropRegions.value = [region]
  }
}

function removeRegion(idx) {
  cropRegions.value.splice(idx, 1)
}

function resetRegions() {
  cropRegions.value = []
}

function onLayoutTypeChange() {
  cropRegions.value = []
}

const drawBoxStyle = computed(() => {
  if (!drawing.value) return { display: 'none' }
  const x1 = Math.min(drawStart.value.x, drawCurrent.value.x)
  const y1 = Math.min(drawStart.value.y, drawCurrent.value.y)
  const w = Math.abs(drawCurrent.value.x - drawStart.value.x)
  const h = Math.abs(drawCurrent.value.y - drawStart.value.y)
  return { left: x1 + 'px', top: y1 + 'px', width: w + 'px', height: h + 'px' }
})

function regionDisplayStyle(region) {
  if (!videoNativeSize.value.w || !overlayRef.value) return { display: 'none' }
  const rect = overlayRef.value.getBoundingClientRect()
  const scaleX = rect.width / videoNativeSize.value.w
  const scaleY = rect.height / videoNativeSize.value.h
  return {
    left: (region.x * scaleX) + 'px',
    top: (region.y * scaleY) + 'px',
    width: (region.w * scaleX) + 'px',
    height: (region.h * scaleY) + 'px',
  }
}

async function saveLayoutConfig() {
  const required = layoutType.value === 'split_top_bottom' ? 2 : 1
  if (cropRegions.value.length < required) {
    alert(`Gambar dulu ${required} area crop di atas video sebelum menyimpan`)
    return
  }
  savingConfig.value = true
  try {
    await api.setRenderConfig(layoutClipId.value, {
      layout_type: layoutType.value,
      aspect_ratio: '9:16',
      crop_regions: cropRegions.value,
      subtitle_enabled: subtitleEnabled.value,
    })
    configSaved.value = true
    setTimeout(() => { configSaved.value = false }, 2000)
  } catch (err) {
    alert('Gagal menyimpan layout: ' + (err.response?.data?.error || err.message))
  } finally {
    savingConfig.value = false
  }
}

async function generateSubtitle() {
  subtitleGenerating.value = true
  try {
    await api.generateSubtitle(layoutClipId.value)
    subtitleGenerated.value = true
  } catch (err) {
    alert('Gagal generate subtitle: ' + (err.response?.data?.error || err.message))
  } finally {
    subtitleGenerating.value = false
  }
}

async function handleRender() {
  if (segments.value.length > 0 && !allSegmentsConfigured()) {
    alert('Masih ada segmen yang belum diatur layoutnya. Lengkapi dulu semua segmen sebelum render.')
    return
  }
  try {
    const res = await api.triggerRender(layoutClipId.value)
    renderJob.startPolling(res.data.job_id, async () => {
      const r = await api.getRenderedClip(layoutClipId.value)
      renderedClip.value = r.data
    })
  } catch (err) {
    alert('Gagal memulai render: ' + (err.response?.data?.error || err.message))
  }
}

function downloadRendered() {
  window.open(api.downloadClipUrl(layoutClipId.value), '_blank')
}

async function handleAddManual() {
  try {
    await api.createClip(projectId, {
      title: manualTitle.value || 'Clip tanpa judul',
      start_ms: timeLabelToMs(manualStart.value || '0:00'),
      end_ms: timeLabelToMs(manualEnd.value || '0:10'),
    })
    manualTitle.value = ''
    manualStart.value = ''
    manualEnd.value = ''
    addingManual.value = false
    await loadClips()
  } catch (err) {
    alert('Gagal menambah clip: ' + (err.response?.data?.error || err.message))
  }
}

const videoUrl = computed(() => api.videoStreamUrl(projectId))

onMounted(() => {
  loadProject()
  loadClips()
})
</script>

<template>
  <div class="max-w-5xl mx-auto px-4 py-10">
    <router-link :to="`/projects/${projectId}`" class="text-sm text-blue-600 hover:underline mb-4 inline-block">
      &larr; Kembali ke Project Detail
    </router-link>

    <div v-if="errorMsg" class="text-red-600">{{ errorMsg }}</div>

    <div v-else>
      <h1 class="text-xl font-bold text-gray-900 mb-6">Review & Edit Clip</h1>

      <!-- Video Preview -->
      <div class="bg-black rounded-xl overflow-hidden mb-6">
        <video ref="videoRef" :src="videoUrl" controls class="w-full max-h-[500px] mx-auto"></video>
      </div>

    <!-- Layout & Crop Editor -->
      <div v-if="layoutClipId" class="bg-white border border-gray-200 rounded-xl p-6 mb-6">
        <div class="flex items-center justify-between mb-4">
          <p class="text-sm font-medium text-gray-700">Atur Layout: {{ layoutClip?.title }}</p>
          <button @click="closeLayoutEditor" class="text-xs text-gray-400 hover:text-gray-600">Tutup</button>
        </div>

        <div class="mb-4 pb-4 border-b border-gray-100">
          <div class="flex items-center justify-between mb-2">
            <p class="text-sm font-medium text-gray-700">Layout Otomatis per Segmen</p>
            <button
              @click="handleDetectSegments"
              :disabled="detectingSegments"
              class="text-xs font-medium bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-300 text-white px-3 py-1.5 rounded-lg"
            >
              {{ detectingSegments ? 'Mendeteksi...' : (segments.length > 0 ? 'Deteksi Ulang' : 'Deteksi Perpindahan Kamera') }}
            </button>
          </div>

          <p v-if="segments.length === 0" class="text-xs text-gray-400">
            Belum ada segmen. Klik "Deteksi Perpindahan Kamera" untuk membagi clip ini otomatis berdasarkan potongan kamera,
            atau lewati untuk pakai 1 layout untuk seluruh clip (seperti sebelumnya).
          </p>

          <div v-else class="space-y-1">
            <div
              v-for="(seg, idx) in segments"
              :key="seg.id"
              @click="selectSegmentForEditing(seg)"
              :class="[
                'flex items-center justify-between px-3 py-2 rounded-lg cursor-pointer text-xs border',
                activeSegmentId === seg.id ? 'border-indigo-400 bg-indigo-50' : 'border-gray-200 hover:border-gray-300'
              ]"
            >
              <span class="font-medium text-gray-700">Segmen {{ idx + 1 }}</span>
              <span class="text-gray-500">{{ segmentDuration(seg) }}s</span>
              <span :class="['px-2 py-0.5 rounded-full', seg.layout_type === 'split_top_bottom' ? 'bg-purple-100 text-purple-700' : 'bg-blue-100 text-blue-700']">
                {{ seg.layout_type === 'split_top_bottom' ? 'Split' : 'Single' }}
              </span>
              <span :class="(seg.crop_regions || []).length > 0 ? 'text-green-600' : 'text-amber-600'">
                {{ (seg.crop_regions || []).length > 0 ? '✓ Diatur' : '⚠ Belum diatur' }}
              </span>
            </div>
          </div>

          <p v-if="segments.length > 0 && activeSegmentId" class="text-xs text-indigo-600 font-medium mt-3">
            Sedang mengedit Segmen {{ segments.findIndex(s => s.id === activeSegmentId) + 1 }} — atur layout & crop di bawah, lalu klik "Simpan Segmen Ini".
          </p>
        </div>

        <p class="text-xs text-gray-500 mb-2">
          Drag di atas video untuk menggambar area crop.
          {{ layoutType === 'split_top_bottom' ? 'Gambar 2 area berurutan (Speaker A dulu, lalu B).' : 'Gambar 1 area.' }}
          Area tergambar: {{ cropRegions.length }} / {{ layoutType === 'split_top_bottom' ? 2 : 1 }}
        </p>

        <div
          class="relative inline-block max-w-full select-none"
          ref="overlayRef"
          @mousedown="startDrawing"
          @mousemove="moveDrawing"
          @mouseup="endDrawing"
          @mouseleave="drawing = false"
        >
          <video
            :src="videoUrl"
            class="max-h-[400px] block pointer-events-none"
            @loadedmetadata="onVideoLoadedMeta"
          ></video>

          <div
            v-for="(region, idx) in cropRegions"
            :key="idx"
            class="absolute border-2 border-green-400 bg-green-400/10 flex items-start justify-end p-1"
            :style="regionDisplayStyle(region)"
          >
            <span class="bg-green-500 text-white text-[10px] px-1 rounded mr-1">{{ region.speaker || 'Crop' }}</span>
            <button @click.stop="removeRegion(idx)" class="bg-red-500 text-white text-[10px] px-1.5 rounded">x</button>
          </div>

          <div v-if="drawing" class="absolute border-2 border-blue-400 bg-blue-400/10 pointer-events-none" :style="drawBoxStyle"></div>
        </div>

        <button @click="resetRegions" class="text-xs text-gray-500 hover:text-red-600 mt-2 block">Reset area</button>

        <div class="flex items-center gap-2 mt-4">
          <input id="subtitle-toggle" type="checkbox" v-model="subtitleEnabled" />
          <label for="subtitle-toggle" class="text-sm text-gray-700">Aktifkan subtitle</label>
        </div>

        <div class="flex flex-wrap gap-2 mt-4">
          <button
            v-if="segments.length === 0"
            @click="saveLayoutConfig"
            :disabled="savingConfig"
            class="bg-blue-600 hover:bg-blue-700 disabled:bg-blue-300 text-white text-sm font-medium px-4 py-2 rounded-lg"
          >
            {{ configSaved ? 'Tersimpan ✓' : (savingConfig ? 'Menyimpan...' : 'Simpan Layout') }}
          </button>
          <button
            v-else
            @click="saveActiveSegment"
            :disabled="!activeSegmentId"
            class="bg-blue-600 hover:bg-blue-700 disabled:bg-blue-300 text-white text-sm font-medium px-4 py-2 rounded-lg"
          >
            {{ configSaved ? 'Tersimpan ✓' : 'Simpan Segmen Ini' }}
          </button>
          <button
            @click="generateSubtitle"
            :disabled="subtitleGenerating"
            class="border border-gray-300 hover:bg-gray-50 text-gray-700 text-sm font-medium px-4 py-2 rounded-lg"
          >
            {{ subtitleGenerated ? 'Subtitle Dibuat ✓' : (subtitleGenerating ? 'Membuat...' : 'Generate Subtitle') }}
          </button>
          <button
            @click="handleRender"
            :disabled="renderJob.isPolling.value"
            class="bg-purple-600 hover:bg-purple-700 disabled:bg-purple-300 text-white text-sm font-medium px-4 py-2 rounded-lg"
          >
            {{ renderJob.isPolling.value ? 'Rendering...' : 'Eksekusi / Render Clip' }}
          </button>
          <button
            v-if="renderedClip"
            @click="downloadRendered"
            class="bg-green-600 hover:bg-green-700 text-white text-sm font-medium px-4 py-2 rounded-lg"
          >
            Download Hasil
          </button>
        </div>

        <div v-if="renderJob.isPolling.value" class="w-full bg-gray-100 rounded-full h-2 mt-3">
          <div class="bg-purple-500 h-2 rounded-full transition-all" :style="{ width: renderJob.jobProgress.value + '%' }"></div>
        </div>
        <p v-if="renderJob.jobError.value" class="text-red-600 text-xs mt-2">{{ renderJob.jobError.value }}</p>
      </div>

      <!-- Import JSON -->
      <div class="bg-white border border-gray-200 rounded-xl p-6 mb-6">
        <p class="text-sm font-medium text-gray-700 mb-3">Import Clip Candidates dari JSON</p>
        <input type="file" accept=".json" @change="handleFileUpload" class="text-sm text-gray-500 mb-3" />
        <textarea
          v-model="jsonInput"
          rows="4"
          placeholder='{"clips": [{"title": "...", "start": "00:00:10", "end": "00:00:40", "reason": "...", "hook_score": 8}]}'
          class="w-full text-xs font-mono border border-gray-300 rounded-lg p-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
        ></textarea>
        <div class="flex items-center gap-3 mt-3">
          <button
            @click="handleImportJSON"
            :disabled="importing"
            class="bg-blue-600 hover:bg-blue-700 disabled:bg-blue-300 text-white text-sm font-medium px-4 py-2 rounded-lg transition"
          >
            {{ importing ? 'Mengimpor...' : 'Import Clips' }}
          </button>
          <p v-if="importError" class="text-red-600 text-xs">{{ importError }}</p>
        </div>
      </div>

      <!-- Clip List -->
      <div class="bg-white border border-gray-200 rounded-xl p-6">
        <div class="flex items-center justify-between mb-4">
          <p class="text-sm font-medium text-gray-700">Daftar Clip ({{ clips.length }})</p>
          <button
            @click="addingManual = !addingManual"
            class="text-xs font-medium text-blue-600 hover:underline"
          >
            + Tambah Manual
          </button>
        </div>

        <!-- Manual add form -->
        <div v-if="addingManual" class="bg-gray-50 border border-gray-200 rounded-lg p-4 mb-4 space-y-2">
          <input v-model="manualTitle" type="text" placeholder="Judul clip"
            class="w-full text-sm border border-gray-300 rounded-lg px-3 py-1.5" />
          <div class="flex gap-2">
            <input v-model="manualStart" type="text" placeholder="Start (mm:ss)"
              class="w-1/2 text-sm border border-gray-300 rounded-lg px-3 py-1.5" />
            <input v-model="manualEnd" type="text" placeholder="End (mm:ss)"
              class="w-1/2 text-sm border border-gray-300 rounded-lg px-3 py-1.5" />
          </div>
          <button @click="handleAddManual"
            class="bg-blue-600 hover:bg-blue-700 text-white text-xs font-medium px-3 py-1.5 rounded-lg">
            Simpan
          </button>
        </div>

        <div v-if="clips.length === 0" class="text-gray-400 text-sm">Belum ada clip. Import JSON di atas untuk mulai.</div>

        <div class="space-y-2">
          <div
            v-for="clip in clips"
            :key="clip.id"
            :class="[
              'border rounded-lg p-3 transition cursor-pointer',
              selectedClipId === clip.id ? 'border-blue-400 bg-blue-50' : 'border-gray-200 hover:border-gray-300'
            ]"
          >
            <!-- Edit mode -->
            <div v-if="editingClipId === clip.id" class="space-y-2" @click.stop>
              <input v-model="editTitle" type="text" class="w-full text-sm border border-gray-300 rounded-lg px-3 py-1.5" />
              <div class="flex gap-2">
                <input v-model="editStart" type="text" class="w-1/2 text-sm border border-gray-300 rounded-lg px-3 py-1.5" />
                <input v-model="editEnd" type="text" class="w-1/2 text-sm border border-gray-300 rounded-lg px-3 py-1.5" />
              </div>
              <div class="flex gap-2">
                <button @click="saveEdit(clip)" class="text-xs font-medium bg-green-600 hover:bg-green-700 text-white px-3 py-1.5 rounded-lg">Simpan</button>
                <button @click="cancelEdit" class="text-xs font-medium text-gray-500 px-3 py-1.5">Batal</button>
              </div>
            </div>

            <!-- Display mode -->
            <div v-else @click="selectClip(clip)">
              <div class="flex items-start justify-between">
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-gray-800 truncate">{{ clip.title }}</p>
                  <p class="text-xs text-gray-500 mt-0.5">
                    {{ msToTimeLabel(clip.start_ms) }} &rarr; {{ msToTimeLabel(clip.end_ms) }}
                    <span class="text-gray-300 mx-1">|</span>
                    {{ durationLabel(clip) }}
                    <span v-if="clip.hook_score" class="text-gray-300 mx-1">|</span>
                    <span v-if="clip.hook_score" class="text-amber-600">🔥 {{ clip.hook_score }}/10</span>
                  </p>
                  <p v-if="clip.reason" class="text-xs text-gray-400 mt-1">{{ clip.reason }}</p>
                </div>
                <div class="flex items-center gap-2 ml-3 flex-shrink-0">
                    <button @click.stop="openLayoutEditor(clip)" class="text-xs text-gray-500 hover:text-purple-600">Layout</button>
                    <button @click.stop="startEdit(clip)" class="text-xs text-gray-500 hover:text-blue-600">Edit</button>
                    <button @click.stop="handleDeleteClip(clip)" class="text-xs text-gray-500 hover:text-red-600">Hapus</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>