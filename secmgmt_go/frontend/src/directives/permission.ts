import type { Directive } from "vue"

import { hasPermission } from "../utils/permission"

export const permissionDirective: Directive<HTMLElement, string> = {
  mounted(el, binding) {
    const permissionCode = binding.value
    if (!permissionCode) {
      return
    }

    if (!hasPermission(permissionCode)) {
      el.remove()
    }
  },
}
