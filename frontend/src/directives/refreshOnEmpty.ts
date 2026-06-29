import type { Directive } from "vue"

type RefreshHandler = (() => void | Promise<void>) | undefined

type RefreshOnEmptyElement = HTMLSelectElement & {
  __refreshOnEmptyHandler__?: EventListener
}

const bindListener = (el: RefreshOnEmptyElement, handler: RefreshHandler) => {
  const listener: EventListener = () => {
    if (el.value === "") {
      void handler?.()
    }
  }

  el.__refreshOnEmptyHandler__ = listener
  el.addEventListener("change", listener)
}

const unbindListener = (el: RefreshOnEmptyElement) => {
  if (!el.__refreshOnEmptyHandler__) {
    return
  }
  el.removeEventListener("change", el.__refreshOnEmptyHandler__)
  delete el.__refreshOnEmptyHandler__
}

export const refreshOnEmptyDirective: Directive<HTMLSelectElement, RefreshHandler> = {
  mounted(el, binding) {
    bindListener(el as RefreshOnEmptyElement, binding.value)
  },
  updated(el, binding) {
    const target = el as RefreshOnEmptyElement
    if (binding.value === binding.oldValue) {
      return
    }
    unbindListener(target)
    bindListener(target, binding.value)
  },
  unmounted(el) {
    unbindListener(el as RefreshOnEmptyElement)
  },
}
