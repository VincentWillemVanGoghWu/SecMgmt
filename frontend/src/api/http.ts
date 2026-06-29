import axios from "axios"
import { ElMessage } from "element-plus"

const TOKEN_KEY = "steel-monitor-access-token"
export const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "http://127.0.0.1:8000/api"

const errorMessageMap: Record<string, string> = {
  "username or password is incorrect": "用户名或密码错误",
  "invalid login payload": "登录请求参数不完整",
  "missing authorization header": "缺少登录凭证，请重新登录",
  "invalid authorization header": "登录凭证格式无效，请重新登录",
  "invalid token": "登录状态已失效，请重新登录",
}

const normalizeResponseMessage = (message: unknown): string | undefined => {
  if (typeof message !== "string") {
    return undefined
  }
  const normalized = message.trim()
  if (!normalized) {
    return undefined
  }
  return errorMessageMap[normalized] ?? normalized
}

export const http = axios.create({
  baseURL: apiBaseUrl,
  timeout: 10000,
})

export const buildApiUrl = (path: string): string => {
  const normalizedBase = apiBaseUrl.startsWith("http")
    ? apiBaseUrl
    : `${window.location.origin}${apiBaseUrl.startsWith("/") ? "" : "/"}${apiBaseUrl}`
  const base = normalizedBase.endsWith("/") ? normalizedBase : `${normalizedBase}/`
  return new URL(path.replace(/^\//, ""), base).toString()
}

export const downloadFile = async (url: string, params?: Record<string, unknown>): Promise<void> => {
  const response = await http.get<Blob>(url, {
    params,
    responseType: "blob",
  })

  const disposition = response.headers["content-disposition"] as string | undefined
  const encodedMatch = disposition?.match(/filename\*=UTF-8''([^;]+)/i)
  const plainMatch = disposition?.match(/filename="?([^"]+)"?/i)
  const filename = encodedMatch?.[1] ? decodeURIComponent(encodedMatch[1]) : (plainMatch?.[1] ?? `export_${Date.now()}.xlsx`)
  const blob = response.data instanceof Blob ? response.data : new Blob([response.data])
  const link = document.createElement("a")
  const objectUrl = window.URL.createObjectURL(blob)
  link.href = objectUrl
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(objectUrl)
}

http.interceptors.request.use((config) => {
  const token = window.localStorage.getItem(TOKEN_KEY)
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (response) => response,
  async (error) => {
    const responseMessage = normalizeResponseMessage(error.response?.data?.message)
    if (responseMessage) {
      error.message = responseMessage
    }
    if (error.response?.status === 401) {
      window.localStorage.removeItem(TOKEN_KEY)
      if (window.location.pathname !== "/login") {
        window.location.replace("/login")
      }
    }
    if (error.response?.status === 403) {
      ElMessage.warning(typeof responseMessage === "string" && responseMessage ? responseMessage : "当前操作无权限")
    }
    return Promise.reject(error)
  },
)
