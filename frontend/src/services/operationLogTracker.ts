import type { RouteLocationNormalizedLoaded, Router } from "vue-router"

import { pinia, useAuthStore, usePermissionStore } from "../stores"
import type { MenuItem } from "../types/navigation"
import type { OperationLogTrackPayload } from "../types/operation-log"

type PageContext = {
  menuCode: string
  menuName: string
  routePath: string
  pageTitle: string
  pageComponent: string
  objectType: string
}

type ActionContext = {
  actionCode: string
  actionName: string
  operationType: string
  objectType: string
  objectName?: string
  objectLocation?: string
}

const TOKEN_KEY = "steel-monitor-access-token"
const ACTION_CONTEXT_TTL = 2000

const trackerState: {
  installed: boolean
  router: Router | null
  lastActionAt: number
  lastAction: ActionContext | null
  lastRouteKey: string
} = {
  installed: false,
  router: null,
  lastActionAt: 0,
  lastAction: null,
  lastRouteKey: "",
}

const getApiBaseUrl = () => {
  const configured = import.meta.env.VITE_API_BASE_URL ?? "http://127.0.0.1:8000/api"
  if (configured.startsWith("http")) {
    return configured.replace(/\/$/, "")
  }
  const normalized = configured.startsWith("/") ? configured : `/${configured}`
  return `${window.location.origin}${normalized}`.replace(/\/$/, "")
}

const getTrackUrl = () => `${getApiBaseUrl()}/operation-logs/track`

const compactText = (value: string) => value.replace(/\s+/g, " ").trim()

const slugify = (value: string) =>
  compactText(value)
    .toLowerCase()
    .replace(/[^a-z0-9\u4e00-\u9fa5]+/g, "-")
    .replace(/^-+|-+$/g, "")

const findMenuPath = (items: MenuItem[], routeName: string, parents: string[] = []): PageContext | null => {
  for (const item of items) {
    const labels = [...parents, item.label]
    if (String(item.routeName ?? "") === routeName) {
      return {
        menuCode: item.key,
        menuName: labels.join(" / "),
        routePath: item.path ?? "",
        pageTitle: labels.join(" / "),
        pageComponent: routeName,
        objectType: inferObjectType(routeName),
      }
    }
    if (item.children?.length) {
      const found = findMenuPath(item.children, routeName, labels)
      if (found) {
        return found
      }
    }
  }
  return null
}

const resolvePageContext = (route: RouteLocationNormalizedLoaded): PageContext => {
  const permissionStore = usePermissionStore(pinia)
  const routeName = String(route.name ?? "")
  const matched = routeName ? findMenuPath(permissionStore.allMenuItems, routeName) : null
  return {
    menuCode: matched?.menuCode ?? routeName,
    menuName: matched?.menuName ?? String(route.meta.title ?? routeName),
    routePath: matched?.routePath ?? route.path,
    pageTitle: String(route.meta.title ?? matched?.pageTitle ?? routeName),
    pageComponent: matched?.pageComponent ?? routeName,
    objectType: matched?.objectType ?? inferObjectType(routeName),
  }
}

const inferObjectType = (routeName: string) => {
  if (routeName.includes("camera")) return "摄像头设备"
  if (routeName.includes("recorder")) return "录像机设备"
  if (routeName.includes("channel")) return "监控通道"
  if (routeName.includes("alarm")) return "告警记录"
  if (routeName.includes("role")) return "角色"
  if (routeName.includes("user")) return "用户"
  if (routeName.includes("push")) return "系统配置"
  if (routeName.includes("playback")) return "录像文件"
  return "系统配置"
}

const inferOperationTypeFromLabel = (label: string, kind: "button" | "tab" | "pagination") => {
  if (kind === "tab") return "标签页切换"
  if (kind === "pagination") return "分页切换"
  if (label.includes("查询") || label.includes("检索")) return "查询"
  if (label.includes("新增")) return "新增"
  if (label.includes("编辑") || label.includes("保存")) return "编辑"
  if (label.includes("删除")) return "删除"
  if (label.includes("导出")) return "导出"
  if (label.includes("预览")) return "预览"
  if (label.includes("回放") || label.includes("录像")) return "回放"
  if (label.includes("下载")) return "下载"
  if (label.includes("截图") || label.includes("抓图")) return "截图"
  if (label.includes("重置")) return "筛选重置"
  if (label.includes("权限")) return "权限分配"
  if (label.includes("密码")) return "密码修改"
  if (label.includes("登出") || label.includes("退出")) return "退出"
  if (label.includes("登录")) return "登录"
  if (label.includes("配置")) return "设备配置"
  return "按钮点击"
}

const createTraceId = () => `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`

const resolveClientOS = () => {
  const ua = navigator.userAgent.toLowerCase()
  if (ua.includes("windows")) return "Windows"
  if (ua.includes("mac os")) return "macOS"
  if (ua.includes("android")) return "Android"
  if (ua.includes("iphone") || ua.includes("ipad") || ua.includes("ios")) return "iOS"
  if (ua.includes("linux")) return "Linux"
  return "未知"
}

const isTrackingEnabled = () => {
  const authStore = useAuthStore(pinia)
  return Boolean(authStore.token && trackerState.router && trackerState.router.currentRoute.value.name !== "login")
}

const setLastAction = (action: ActionContext) => {
  trackerState.lastAction = action
  trackerState.lastActionAt = Date.now()
}

const getRecentAction = () => {
  if (!trackerState.lastAction) return null
  if (Date.now() - trackerState.lastActionAt > ACTION_CONTEXT_TTL) {
    trackerState.lastAction = null
    return null
  }
  return trackerState.lastAction
}

const getElementText = (element: HTMLElement) => {
  const datasetText = compactText(element.dataset.logActionName ?? "")
  if (datasetText) return datasetText
  const ariaText = compactText(element.getAttribute("aria-label") ?? "")
  if (ariaText) return ariaText
  const titleText = compactText(element.getAttribute("title") ?? "")
  if (titleText) return titleText
  const text = compactText(element.innerText ?? element.textContent ?? "")
  if (text) return text
  if (element.classList.contains("app-header__toggle")) return "切换侧栏"
  if (element.classList.contains("el-dialog__headerbtn")) return "关闭弹窗"
  return ""
}

const buildActionContext = (
  kind: "button" | "tab" | "pagination",
  label: string,
  page: PageContext,
  element?: HTMLElement,
): ActionContext => {
  const actionName = compactText(label || element?.dataset.logActionName || "按钮点击")
  const actionCode =
    compactText(element?.dataset.logAction ?? "") ||
    `${kind}-${slugify(page.menuCode || page.pageComponent || "page")}-${slugify(actionName || "action")}`
  const objectType = compactText(element?.dataset.objectType ?? "") || page.objectType
  const objectName = compactText(element?.dataset.objectName ?? "")
  const objectLocation = compactText(element?.dataset.objectLocation ?? "")
  return {
    actionCode,
    actionName,
    operationType: inferOperationTypeFromLabel(actionName, kind),
    objectType,
    objectName: objectName || undefined,
    objectLocation: objectLocation || undefined,
  }
}

const sendTrack = async (payload: OperationLogTrackPayload, keepalive = false) => {
  const token = window.localStorage.getItem(TOKEN_KEY)
  if (!token) return
  try {
    await fetch(getTrackUrl(), {
      method: "POST",
      keepalive,
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
        "X-Trace-Id": createTraceId(),
        "X-Client-OS": resolveClientOS(),
      },
      body: JSON.stringify(payload),
    })
  } catch {
    // Ignore telemetry failures to keep UI responsive.
  }
}

const trackRouteChange = async (to: RouteLocationNormalizedLoaded, from: RouteLocationNormalizedLoaded) => {
  if (!isTrackingEnabled()) return
  const page = resolvePageContext(to)
  const currentKey = `${String(to.name ?? "")}|${to.fullPath}`
  if (trackerState.lastRouteKey === currentKey) return
  trackerState.lastRouteKey = currentKey

  if (from.name && from.fullPath !== to.fullPath) {
    await sendTrack({
      source: "ui",
      ...page,
      actionCode: `menu-switch-${slugify(String(to.name ?? "page"))}`,
      actionName: "菜单切换",
      operationType: "菜单切换",
      objectType: page.objectType,
    })
    await sendTrack({
      source: "ui",
      ...page,
      actionCode: `page-open-${slugify(String(to.name ?? "page"))}`,
      actionName: "打开页面",
      operationType: "标签页切换",
      objectType: page.objectType,
    })
    return
  }

  await sendTrack({
    source: "ui",
    ...page,
    actionCode: `page-open-${slugify(String(to.name ?? "page"))}`,
    actionName: "打开页面",
    operationType: "打开页面",
    objectType: page.objectType,
  })
}

const getNavigationType = () => {
  const entry = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming | undefined
  return entry?.type ?? "navigate"
}

const handleDocumentClick = (event: MouseEvent) => {
  if (!isTrackingEnabled() || !trackerState.router) return
  const target = event.target
  if (!(target instanceof Element)) return
  if (target.closest(".app-sidebar")) return
  const page = resolvePageContext(trackerState.router.currentRoute.value)

  const tab = target.closest(".el-tabs__item") as HTMLElement | null
  if (tab) {
    const label = getElementText(tab)
    if (!label) return
    const action = buildActionContext("tab", label, page, tab)
    setLastAction(action)
    void sendTrack({ source: "ui", ...page, ...action })
    return
  }

  const pagination = target.closest(".el-pagination button") as HTMLElement | null
  if (pagination) {
    let label = getElementText(pagination)
    if (!label) {
      if (pagination.classList.contains("btn-prev")) label = "上一页"
      if (pagination.classList.contains("btn-next")) label = "下一页"
    }
    if (!label) return
    const action = buildActionContext("pagination", label, page, pagination)
    setLastAction(action)
    void sendTrack({ source: "ui", ...page, ...action })
    return
  }

  const button = target.closest("button") as HTMLElement | null
  if (!button) return
  const label = getElementText(button)
  if (!label) return
  const action = buildActionContext("button", label, page, button)
  setLastAction(action)
  void sendTrack({ source: "ui", ...page, ...action })
}

export const installOperationLogTracker = (router: Router) => {
  if (trackerState.installed) return
  trackerState.installed = true
  trackerState.router = router

  router.afterEach((to, from) => {
    void trackRouteChange(to, from)
  })

  document.addEventListener("click", handleDocumentClick, true)

  void router.isReady().then(() => {
    if (!isTrackingEnabled()) return
    if (getNavigationType() !== "reload") return
    const page = resolvePageContext(router.currentRoute.value)
    void sendTrack(
      {
        source: "ui",
        ...page,
        actionCode: `page-refresh-${slugify(String(router.currentRoute.value.name ?? "page"))}`,
        actionName: "页面刷新",
        operationType: "页面刷新",
        objectType: page.objectType,
      },
      true,
    )
  })
}

export const getOperationTrackingHeaders = (): Record<string, string> => {
  if (!isTrackingEnabled() || !trackerState.router) {
    return {}
  }
  const route = trackerState.router.currentRoute.value
  const page = resolvePageContext(route)
  const action = getRecentAction()
  return {
    "X-Track-Source": "frontend-ui",
    "X-Menu-Code": page.menuCode,
    "X-Menu-Name": page.menuName,
    "X-Page-Route": page.routePath,
    "X-Page-Title": page.pageTitle,
    "X-Page-Component": page.pageComponent,
    "X-Action-Code": action?.actionCode ?? `page-load-${slugify(String(route.name ?? "page"))}`,
    "X-Action-Name": action?.actionName ?? "页面加载",
    "X-Operation-Type": action?.operationType ?? "浏览页面",
    "X-Object-Type": action?.objectType ?? page.objectType,
    "X-Object-Id": "",
    "X-Object-Name": action?.objectName ?? "",
    "X-Object-Location": action?.objectLocation ?? "",
    "X-Trace-Id": createTraceId(),
    "X-Client-OS": resolveClientOS(),
  }
}
