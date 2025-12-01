import axios from 'axios'

const client = axios.create({
  baseURL: '/api/v1'
})

export default {
  async createJob(payload) {
    const { data } = await client.post('/jobs', payload)
    return data
  },
  async listJobs() {
    const { data } = await client.get('/jobs')
    return data
  },
  async cancelJob(id) {
    const { data } = await client.post(`/jobs/${id}/cancel`)
    return data
  },
  async deleteJob(id) {
    const { data } = await client.delete(`/jobs/${id}`)
    return data
  },
  async deleteAllJobs() {
    const { data } = await client.delete('/jobs')
    return data
  },
  async listBGM() {
    const { data } = await client.get('/presets/bgm')
    return data
  },
  async listFonts() {
    const { data } = await client.get('/fonts')
    return data
  },
  async listVoices(provider) {
    const { data } = await client.get('/tts/voices', { params: { provider } })
    return data
  },
  async previewSubtitle(payload) {
    const response = await client.post('/preview/subtitle', payload, {
      responseType: 'blob'
    })
    return URL.createObjectURL(response.data)
  },
  async uploadFile(file) {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await client.post('/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
    return data
  }
}
