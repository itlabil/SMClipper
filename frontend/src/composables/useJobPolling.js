import { ref } from 'vue'
import api from '@/services/api'

export function useJobPolling() {
  const jobStatus = ref(null) // null | 'queued' | 'processing' | 'done' | 'failed'
  const jobProgress = ref(0)
  const jobError = ref('')
  const isPolling = ref(false)

  let intervalId = null

  function startPolling(jobId, onDone) {
    isPolling.value = true
    jobStatus.value = 'queued'
    jobProgress.value = 0
    jobError.value = ''

    intervalId = setInterval(async () => {
      try {
        const res = await api.getJob(jobId)
        const job = res.data
        jobStatus.value = job.status
        jobProgress.value = job.progress

        if (job.status === 'done') {
          stopPolling()
          if (onDone) onDone()
        } else if (job.status === 'failed') {
          jobError.value = job.error_message || 'Job gagal tanpa pesan error'
          stopPolling()
        }
      } catch (err) {
        jobError.value = 'Gagal memeriksa status job'
        stopPolling()
      }
    }, 2000)
  }

  function stopPolling() {
    isPolling.value = false
    if (intervalId) {
      clearInterval(intervalId)
      intervalId = null
    }
  }

  return { jobStatus, jobProgress, jobError, isPolling, startPolling, stopPolling }
}