import type { Component } from "vue"

import {
  Bell,
  CircleCheck,
  Connection,
  DataAnalysis,
  Files,
  Grid,
  Histogram,
  House,
  Lock,
  Monitor,
  OfficeBuilding,
  Operation,
  Postcard,
  Setting,
  Share,
  SwitchButton,
  User,
  VideoCamera,
  Warning,
} from "@element-plus/icons-vue"

const menuIconMap: Record<string, Component> = {
  House,
  Warning,
  Bell,
  Histogram,
  Share,
  Monitor,
  VideoCamera,
  Operation,
  Connection,
  Setting,
  SwitchButton,
  Grid,
  DataAnalysis,
  OfficeBuilding,
  Postcard,
  Files,
  Lock,
  User,
  CircleCheck,
}

export const resolveMenuIcon = (iconName?: string | null): Component => {
  if (!iconName) {
    return Grid
  }
  return menuIconMap[iconName] ?? Grid
}
