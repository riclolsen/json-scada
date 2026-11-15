<template>
  <v-navigation-drawer
    v-model="open"
    :rail="isRail"
    :permanent="!mobile"
    :temporary="mobile"
    :width="252"
    :rail-width="60"
    class="app-sidebar"
    :class="{ 'is-rail': isRail }"
  >
    <!-- brand -->
    <template #prepend>
      <router-link to="/dashboard" class="brand" :title="version" @click="closeIfMobile">
        <svg class="brand-mark" viewBox="0 0 32 32" aria-hidden="true">
          <rect width="32" height="32" rx="8" class="brand-mark-bg" />
          <text x="16" y="21" text-anchor="middle" class="brand-mark-text">j:s</text>
        </svg>
        <div v-if="!isRail" class="brand-text">
          <div class="brand-name">{json:scada}</div>
          <div class="brand-version">{{ version }}</div>
        </div>
      </router-link>
      <v-divider></v-divider>
    </template>

    <!-- navigation -->
    <v-list v-if="loggedInUser" density="compact" nav class="nav-list">
      <template v-for="(entry, i) in menu" :key="entry.key">
        <v-list-subheader v-if="entry.group && !isRail" class="nav-group">
          {{ $t('nav.groups.' + entry.group) }}
        </v-list-subheader>
        <v-divider v-else-if="entry.group && i > 0" class="my-1 mx-2"></v-divider>

        <v-menu
          v-if="entry.item"
          :disabled="!isRail"
          open-on-hover
          :open-on-click="false"
          location="end"
          offset="8"
          :open-delay="120"
          :close-delay="120"
        >
          <template #activator="{ props: menuProps }">
            <v-list-item
              v-bind="menuProps"
              :to="entry.item.route || undefined"
              :href="entry.item.route ? undefined : entry.item.page"
              :target="entry.item.route ? undefined : '_blank'"
              :title="isRail ? undefined : $t(entry.item.titleKey)"
              :aria-label="$t(entry.item.titleKey)"
              color="primary"
              rounded="lg"
              class="nav-item"
              @click="closeIfMobile"
            >
              <template #prepend>
                <NavIcon :name="entry.item.id" class="nav-item-icon" />
              </template>
              <template v-if="!isRail" #append>
                <v-btn
                  icon
                  variant="text"
                  size="x-small"
                  class="nav-newwin"
                  :href="entry.item.page"
                  target="_blank"
                  :title="$t('nav.openNewWindow')"
                  :aria-label="$t('nav.openNewWindow')"
                  @click.stop
                >
                  <NavIcon name="openNew" :size="16" />
                </v-btn>
              </template>
            </v-list-item>
          </template>

          <!-- rail flyout: name + same / new window -->
          <v-list density="compact" class="nav-flyout" elevation="6" rounded="lg">
            <v-list-subheader class="font-weight-bold">{{ $t(entry.item.titleKey) }}</v-list-subheader>
            <v-list-item
              v-if="entry.item.route"
              :to="entry.item.route"
              :title="$t('nav.openHere')"
              rounded="lg"
            >
              <template #prepend><NavIcon name="openHere" :size="18" class="mr-3" /></template>
            </v-list-item>
            <v-list-item :href="entry.item.page" target="_blank" :title="$t('nav.openNewWindow')" rounded="lg">
              <template #prepend><NavIcon name="openNew" :size="18" class="mr-3" /></template>
            </v-list-item>
          </v-list>
        </v-menu>
      </template>
    </v-list>

    <!-- user, preferences, collapse -->
    <template #append>
      <v-divider></v-divider>
      <div class="nav-footer" :class="{ stacked: isRail }">
        <!-- user -->
        <v-menu v-if="loggedInUser" location="end" offset="8">
          <template #activator="{ props: menuProps }">
            <button v-bind="menuProps" type="button" class="nav-user" :aria-label="loggedInUser">
              <v-avatar size="30" color="primary" variant="tonal" class="nav-avatar">{{ initials }}</v-avatar>
              <span v-if="!isRail" class="nav-user-name">{{ loggedInUser }}</span>
              <NavIcon v-if="!isRail" name="chevronRight" :size="16" class="nav-user-more" />
              <v-tooltip v-if="isRail" activator="parent" location="end">{{ loggedInUser }}</v-tooltip>
            </button>
          </template>
          <v-list density="compact" class="nav-flyout" elevation="6" rounded="lg">
            <v-list-subheader class="font-weight-bold">{{ loggedInUser }}</v-list-subheader>
            <v-list-item
              v-if="!isLDAPUser"
              :title="$t('login.changePassword')"
              rounded="lg"
              @click="$emit('change-password')"
            >
              <template #prepend><NavIcon name="password" :size="18" class="mr-3" /></template>
            </v-list-item>
            <v-list-item :title="$t('app.logout')" rounded="lg" @click="$emit('logout')">
              <template #prepend><NavIcon name="logout" :size="18" class="mr-3" /></template>
            </v-list-item>
          </v-list>
        </v-menu>

        <div class="nav-tools">
          <!-- language -->
          <v-menu location="end" offset="8">
            <template #activator="{ props: menuProps }">
              <v-btn v-bind="menuProps" icon variant="text" size="small" :aria-label="$t('nav.language')">
                <NavIcon name="language" :size="20" />
                <v-tooltip activator="parent" :location="isRail ? 'end' : 'top'">
                  {{ $t('nav.language') }}: {{ currentLocaleName }}
                </v-tooltip>
              </v-btn>
            </template>
            <v-list density="compact" class="nav-flyout" elevation="6" rounded="lg" max-height="70vh">
              <v-list-item
                v-for="l in availableLocales"
                :key="l.value"
                :title="l.title"
                :active="l.value === locale"
                color="primary"
                rounded="lg"
                @click="$emit('update:locale', l.value)"
              ></v-list-item>
            </v-list>
          </v-menu>

          <!-- theme -->
          <v-btn
            icon
            variant="text"
            size="small"
            :aria-label="$t(theme === 'light' ? 'nav.darkTheme' : 'nav.lightTheme')"
            @click="$emit('toggle-theme')"
          >
            <NavIcon :name="theme === 'light' ? 'moon' : 'sun'" :size="20" />
            <v-tooltip activator="parent" :location="isRail ? 'end' : 'top'">
              {{ $t(theme === 'light' ? 'nav.darkTheme' : 'nav.lightTheme') }}
            </v-tooltip>
          </v-btn>

          <!-- about -->
          <v-btn
            icon
            variant="text"
            size="small"
            :to="aboutItem.route"
            :color="route.path === aboutItem.route ? 'primary' : undefined"
            :aria-label="$t(aboutItem.titleKey)"
            @click="closeIfMobile"
          >
            <NavIcon :name="aboutItem.id" :size="20" />
            <v-tooltip activator="parent" :location="isRail ? 'end' : 'top'">{{ $t(aboutItem.titleKey) }}</v-tooltip>
          </v-btn>

          <!-- collapse / expand -->
          <v-btn
            v-if="!mobile"
            icon
            variant="text"
            size="small"
            :aria-label="$t(isRail ? 'nav.expand' : 'nav.collapse')"
            @click="toggleRail"
          >
            <NavIcon :name="isRail ? 'expand' : 'collapse'" :size="20" />
            <v-tooltip activator="parent" :location="isRail ? 'end' : 'top'">
              {{ $t(isRail ? 'nav.expand' : 'nav.collapse') }}
            </v-tooltip>
          </v-btn>
        </div>
      </div>
    </template>
  </v-navigation-drawer>

  <!-- small screens: the panel slides in over the page -->
  <v-btn
    v-if="mobile && !open"
    icon
    size="small"
    color="primary"
    class="nav-fab"
    :aria-label="$t('nav.menu')"
    @click="open = true"
  >
    <NavIcon name="menu" :size="20" />
  </v-btn>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import NavIcon from './NavIcon.vue'
import { visibleGroups, aboutItem, homeItem } from '../lib/navItems'

const props = defineProps({
  loggedInUser: { type: String, default: '' },
  isLDAPUser: { type: Boolean, default: false },
  isAdmin: { type: Boolean, default: false },
  theme: { type: String, default: 'dark' },
  locale: { type: String, default: 'en' },
  availableLocales: { type: Array, default: () => [] },
  version: { type: String, default: '' },
})
defineEmits(['update:locale', 'toggle-theme', 'logout', 'change-password'])

useI18n()
const route = useRoute()
const { smAndDown } = useDisplay()
const mobile = computed(() => smAndDown.value)

// collapsed (icons only) state survives reloads; per browser
const RAIL_KEY = 'nav-rail'
function readRail() {
  try {
    return localStorage.getItem(RAIL_KEY) === '1'
  } catch (e) {
    return false
  }
}
const rail = ref(readRail())
const isRail = computed(() => rail.value && !mobile.value)
function toggleRail() {
  rail.value = !rail.value
  try {
    localStorage.setItem(RAIL_KEY, rail.value ? '1' : '0')
  } catch (e) {
    /* storage unavailable: state lasts for this page only */
  }
}

// permanent on desktop, slide-in on small screens
const open = ref(!mobile.value)
watch(mobile, (m) => (open.value = !m))
function closeIfMobile() {
  if (mobile.value) open.value = false
}

// flat list: home, then each group (header + items)
const menu = computed(() => {
  const out = [{ key: 'home', item: homeItem }]
  for (const g of visibleGroups(props.isAdmin)) {
    out.push({ key: 'g-' + g.key, group: g.key })
    for (const it of g.items) out.push({ key: it.id, item: it })
  }
  return out
})

const currentLocaleName = computed(
  () => (props.availableLocales.find((l) => l.value === props.locale) || {}).title || props.locale
)

const initials = computed(() => (props.loggedInUser || '?').slice(0, 2).toUpperCase())
</script>

<style scoped>
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 56px;
  padding: 0 14px;
  color: inherit;
  text-decoration: none;
  overflow: hidden;
}
.is-rail .brand {
  padding: 0;
  justify-content: center;
}
.brand-mark {
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
}
.brand-text {
  min-width: 0;
  line-height: 1.15;
}
.brand-name {
  font-weight: 700;
  font-size: 1.02rem;
  letter-spacing: 0.01em;
  white-space: nowrap;
}
.brand-version {
  font-size: 0.72rem;
  opacity: 0.6;
  white-space: nowrap;
}
.nav-list {
  padding: 6px 8px;
}
.nav-group {
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  opacity: 0.65;
  min-height: 30px;
  padding-inline-start: 12px !important;
}
.nav-item {
  min-height: 38px !important;
  margin-bottom: 2px;
}
.nav-item-icon {
  margin-inline-end: 14px;
  opacity: 0.85;
}
.nav-item.v-list-item--active .nav-item-icon {
  opacity: 1;
}
.is-rail .nav-item {
  grid-template-columns: 1fr;
  justify-items: center;
  padding-inline: 0 !important;
}
.is-rail .nav-item :deep(.v-list-item__content),
.is-rail .nav-item :deep(.v-list-item__spacer) {
  display: none;
}
.is-rail .nav-item-icon {
  margin-inline-end: 0;
}
.brand-mark-bg {
  fill: rgb(var(--v-theme-primary));
}
.brand-mark-text {
  fill: rgb(var(--v-theme-on-primary));
  font: 700 13px ui-monospace, 'Cascadia Mono', Consolas, monospace;
}
/* footer: user row + tool buttons (stacked in rail mode) */
.nav-footer {
  padding: 6px 8px 8px;
}
.nav-user {
  border: 0;
  background: transparent;
  cursor: pointer;
  font: inherit;
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 5px 6px;
  border-radius: 8px;
  color: inherit;
  text-align: start;
}
.nav-user:hover,
.nav-user:focus-visible,
.nav-user[aria-expanded='true'] {
  background: rgba(var(--v-theme-on-surface), 0.06);
}
.stacked .nav-user {
  justify-content: center;
  padding: 5px 0;
}
.nav-avatar {
  font-size: 0.7rem;
  font-weight: 700;
}
.nav-user-name {
  flex: 1 1 auto;
  min-width: 0;
  font-size: 0.875rem;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.nav-user-more {
  opacity: 0.5;
}
.nav-tools {
  display: flex;
  justify-content: space-between;
  margin-top: 4px;
}
.stacked .nav-tools {
  flex-direction: column;
  align-items: center;
  gap: 2px;
}
/* thin scrollbar matching the theme when the menu does not fit */
.app-sidebar :deep(.v-navigation-drawer__content) {
  scrollbar-width: thin;
  scrollbar-color: rgba(var(--v-theme-on-surface), 0.25) transparent;
}
/* "open in new window" appears on hover/focus (always on touch screens) */
.nav-newwin {
  opacity: 0;
  transition: opacity 0.15s;
}
.nav-item:hover .nav-newwin,
.nav-item:focus-within .nav-newwin,
.nav-newwin:focus-visible {
  opacity: 0.8;
}
@media (hover: none) {
  .nav-newwin {
    opacity: 0.7;
  }
}
.nav-flyout {
  min-width: 200px;
}
.nav-fab {
  position: fixed;
  left: 10px;
  bottom: 10px;
  z-index: 1010;
}
</style>
