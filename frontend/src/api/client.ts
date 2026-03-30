import axios from 'axios'
import type {
  FileTreeNode,
  PodsResponse,
  Pod,
  Container,
  DependenciesResponse,
} from '../types'

const http = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

export async function setProject(path: string) {
  const { data } = await http.post('/project', { path })
  return data as { message: string; path: string; podCount: number }
}

export async function getFileTree() {
  const { data } = await http.get<FileTreeNode>('/filetree')
  return data
}

export async function getPods() {
  const { data } = await http.get<PodsResponse>('/pods')
  return data
}

export async function getPod(path: string) {
  const { data } = await http.get<Pod>(`/pod/${path}`)
  return data
}

export async function getContainers(path: string) {
  const { data } = await http.get<Container[]>(`/containers/${path}`)
  return data
}

export async function getContainer(path: string, name: string) {
  const { data } = await http.get<Container>(`/container/${path}`, {
    params: { name },
  })
  return data
}

export async function getDependencies(path: string, depth: number = 1) {
  const { data } = await http.get<DependenciesResponse>(`/dependencies/${path}`, {
    params: { depth },
  })
  return data
}
