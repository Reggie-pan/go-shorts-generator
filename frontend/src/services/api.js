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
  async listBGM() {
    const { data } = await client.get('/presets/bgm')
    return data
  }
}
