import axios from 'axios'

const api = axios.create({
  baseURL: 'http://localhost:8080/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

export default {
  // Projects
  createProject(youtubeUrl) {
    return api.post('/projects', { youtube_url: youtubeUrl })
  },
  listProjects() {
    return api.get('/projects')
  },
  getProject(id) {
    return api.get(`/projects/${id}`)
  },
  deleteProject(id) {
    return api.delete(`/projects/${id}`)
  },
  triggerDownload(projectId) {
    return api.post(`/projects/${projectId}/download`)
  },
  triggerTranscribe(projectId) {
    return api.post(`/projects/${projectId}/transcribe`)
  },
  getTranscript(projectId) {
    return api.get(`/projects/${projectId}/transcript`)
  },
  downloadTranscriptUrl(projectId, format = 'txt') {
    return `http://localhost:8080/api/projects/${projectId}/transcript/export?format=${format}`
  },

  // Jobs
  getJob(jobId) {
    return api.get(`/jobs/${jobId}`)
  },

  // Clips
  importClips(projectId, clipsPayload) {
    return api.post(`/projects/${projectId}/clips/import`, clipsPayload)
  },
  listClips(projectId) {
    return api.get(`/projects/${projectId}/clips`)
  },
  createClip(projectId, clip) {
    return api.post(`/projects/${projectId}/clips`, clip)
  },
  updateClip(clipId, updates) {
    return api.put(`/clips/${clipId}`, updates)
  },
  deleteClip(clipId) {
    return api.delete(`/clips/${clipId}`)
  },

  // Render config
  getRenderConfig(clipId) {
    return api.get(`/clips/${clipId}/render-config`)
  },
  setRenderConfig(clipId, config) {
    return api.put(`/clips/${clipId}/render-config`, config)
  },

  // Subtitle
  generateSubtitle(clipId) {
    return api.post(`/clips/${clipId}/subtitle/generate`)
  },

  // Render
  triggerRender(clipId) {
    return api.post(`/clips/${clipId}/render`)
  },
  getRenderedClip(clipId) {
    return api.get(`/clips/${clipId}/rendered`)
  },
  downloadClipUrl(clipId) {
    return `http://localhost:8080/api/clips/${clipId}/download`
  },
  videoStreamUrl(projectId) {
    return `http://localhost:8080/api/projects/${projectId}/video`
  },
  detectSegments(clipId) {
    return api.post(`/clips/${clipId}/segments/detect`)
  },
  listSegments(clipId) {
    return api.get(`/clips/${clipId}/segments`)
  },
  updateSegment(segmentId, data) {
    return api.put(`/segments/${segmentId}`, data)
  },
}