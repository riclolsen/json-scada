<template>
  <v-app :theme="theme">
    <AppSidebar
      v-if="!embedded"
      :logged-in-user="loggedInUser || ''"
      :isLDAPUser="isLDAPUser"
      :is-admin="isAdmin"
      :theme="theme"
      :locale="currentLocale"
      :available-locales="availableLocales"
      :version="version"
      @update:locale="(l) => (currentLocale = l)"
      @toggle-theme="toggleTheme"
      @logout="logout"
      @change-password="openChangePasswordDialog"
    />

    <!-- Change Password Dialog -->
    <v-dialog scrollable v-if="loggedInUser" v-model="showChangePasswordDialog" max-width="400px">
      <v-card>
        <v-card-title>{{ $t('login.changePassword') }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="loggedInUser" :label="$t('login.username')" type="text" disabled></v-text-field>

          <v-text-field v-model="currentPassword" :label="$t('login.currentPassword')" type="password"
            required></v-text-field>

          <v-text-field v-model="newPassword" :label="$t('login.newPassword')" type="password" required></v-text-field>
          <v-text-field v-model="confirmNewPassword" :label="$t('login.confirmNewPassword')" type="password"
            required></v-text-field>
        </v-card-text>
        <v-card-actions>
          <v-alert v-if="errorMessage" type="error" density="compact">
            {{ errorMessage }}
          </v-alert>
          <v-btn color="orange" variant="tonal" @click="showChangePasswordDialog = false">
            {{ $t('common.cancel') }}
          </v-btn>
          <v-btn color="primary" variant="tonal" @click="changePwd">
            {{ $t('common.save') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-main class="flex-grow-1" style="overflow-y: auto; scrollbar-width: none; -ms-overflow-style: none">
      <router-view></router-view>
    </v-main>
  </v-app>
</template>

<script setup>
import { ref, watch, provide, onMounted, computed } from 'vue'
import { useTheme } from 'vuetify'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { STORAGE_KEY, getStoredLocale } from './i18n'
import { getUserSetting, setUserSetting } from './lib/viewerHelpers'
import AppSidebar from './components/AppSidebar.vue'

const THEME_STORAGE_KEY = 'user-theme'

const version = ref('v0.72-alpha')
const router = useRouter()
const route = useRoute()
const vuetifyTheme = useTheme()
const { locale, t } = useI18n()

// theme and language are kept per user (see getUserSetting)
const getStoredTheme = () => getUserSetting(THEME_STORAGE_KEY) || 'dark'
const theme = ref(getStoredTheme())

// `?embed=1`: the page runs inside an iframe (Display Viewer hover preview,
// #foreign_object) or a bare popup — no side panel
// (read from the URL too: the first render happens before the router resolves)
const isEmbedFlag = (v) => v === '1' || v === 'true'
const embedded = computed(
  () =>
    isEmbedFlag(route.query.embed) ||
    isEmbedFlag(new URLSearchParams(window.location.hash.split('?')[1] || '').get('embed'))
)
provide('embedded', embedded)

// Map route paths to i18n keys for the browser tab title
const pageTitle = computed(() => {
  const map = {
    '/login': 'login.title',
    '/dashboard': 'nav.home',
    '/display-viewer': 'dashboard.displayViewer',
    '/alarms-viewer': 'dashboard.alarmsViewer',
    '/tabular-viewer': 'dashboard.tabularViewer',
    '/events-viewer': 'dashboard.eventsViewer',
    '/trend-viewer': 'trendViewer.title',
    '/log-viewer': 'dashboard.logViewer',
    '/about': 'dashboard.about',
    '/admin': 'dashboard.admin',
    '/cim-ems': 'dashboard.cimEms',
    '/grafana': 'dashboard.grafana',
    '/metabase': 'dashboard.metabase',
    '/nodered': 'dashboard.nodered',
    '/n8n': 'dashboard.n8n',
    '/apollo': 'dashboard.apollo',
    '/custom-developments': 'dashboard.customDevelopments',
    '/svg-edit': 'dashboard.svgedit',
  }
  const key = map[route.path]
  return key ? t(key) : ''
})
watch(
  pageTitle,
  (title) => (document.title = title ? title + ' · ' + t('app.title') : t('app.title')),
  { immediate: true }
)

const currentLocale = ref(locale.value)
const loggedInUser = ref('')
// a user who signs in gets their own theme and language back
watch(loggedInUser, (name) => {
  if (!name) return
  const th = getStoredTheme()
  if (th !== theme.value) {
    theme.value = th
    vuetifyTheme.global.name.value = th
  }
  const loc = getStoredLocale()
  if (loc !== currentLocale.value) currentLocale.value = loc
})
const isLDAPUser = ref(false)
const showChangePasswordDialog = ref(false)
const newPassword = ref('')
const confirmNewPassword = ref('')
const currentPassword = ref('')
const errorMessage = ref('')

// admin-only menu entries follow the signed-in user's rights (cookie)
const isAdmin = computed(() => {
  if (!loggedInUser.value) return false
  try {
    const ck = parseCookie(document.cookie)
    return !!JSON.parse(ck['json-scada-user'] || '{}')?.rights?.isAdmin
  } catch (e) {
    return false
  }
})

const availableLocales = [
  { title: 'English', value: 'en' },
  { title: 'Español', value: 'es' },
  { title: 'Français', value: 'fr' },
  { title: 'Italiano', value: 'it' },
  { title: 'Português', value: 'pt' },
  { title: 'Deutsch', value: 'de' },
  { title: 'Русский', value: 'ru' },
  { title: 'Українська', value: 'uk' },
  { title: 'العربية', value: 'ar' },
  { title: 'فارسی', value: 'fa' },
  { title: '日本語', value: 'ja' },
  { title: '中文', value: 'zh' },
  { title: 'پښتو', value: 'ps' },
]

onMounted(() => {
  vuetifyTheme.global.name.value = theme.value
  checkLogin()
  setInterval(checkLogin, 15000)
})

const toggleTheme = () => {
  theme.value = theme.value === 'light' ? 'dark' : 'light'
  vuetifyTheme.global.name.value = theme.value
  setUserSetting(THEME_STORAGE_KEY, theme.value, { alsoShared: true })
}

const setLoggedInUser = (username) => {
  loggedInUser.value = username
  let ck = parseCookie(document.cookie)
  if ('json-scada-user' in ck) {
    ck = JSON.parse(ck['json-scada-user'])
    if ('isLDAPUser' in ck) isLDAPUser.value = ck.isLDAPUser
  }
}

const logout = () => {
  loggedInUser.value = null
  fetch('/Invoke/auth/signout', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
  })
    .then((response) => response.json())
    .then((data) => {
      if (data.ok) {
        router.push('/login')
      } else {
        console.error('Logout failed!')
      }
    })
    .catch((error) => {
      console.error('Error during logout:', error)
    })
}

const openChangePasswordDialog = () => {
  errorMessage.value = ''
  currentPassword.value = newPassword.value = confirmNewPassword.value = ''
  showChangePasswordDialog.value = true
}

const changePwd = () => {
  // Validate passwords
  if (newPassword.value !== confirmNewPassword.value) {
    errorMessage.value = t('login.passwordMismatch')
    setTimeout(() => {
      errorMessage.value = ''
    }, 1500)
    return
  }

  if (
    newPassword.value.trim() === '' ||
    newPassword.value.trim().length < 4
  ) {
    errorMessage.value = t('login.invalidNewPassword')
    setTimeout(() => {
      errorMessage.value = ''
    }, 1500)
    return
  }

  let ck = parseCookie(document.cookie)
  if ('json-scada-user' in ck) {
    ck = JSON.parse(ck['json-scada-user'])
  }

  fetch('/Invoke/auth/changePassword', {
    method: 'post',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      username: ck.username,
      currentPassword: currentPassword.value.trim(),
      newPassword: newPassword.value.trim(),
    }),
  })
    .then((res) => res.json())
    .then((data) => {
      if (!('error' in data)) {
        errorMessage.value = ''
        showChangePasswordDialog.value = false
      } else {
        errorMessage.value = t('login.changePasswordFailed')
        setTimeout(() => {
          errorMessage.value = ''
        }, 1500)
      }
    })
    .catch((err) => {
      console.warn(err)
      errorMessage.value = t('login.changePasswordError')
      setTimeout(() => {
        errorMessage.value = ''
      }, 1500)
    })
}

watch(currentLocale, (newLocale) => {
  locale.value = newLocale
  setUserSetting(STORAGE_KEY, newLocale, { alsoShared: true })
})

function checkLogin() {
  let ck = parseCookie(document.cookie)
  if ('json-scada-user' in ck) {
    ck = JSON.parse(ck['json-scada-user'])
  }

  fetch('/Invoke/test/user', {
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    method: 'GET',
  })
    .then((resp) => resp.json())
    .then((data) => {
      if ('ok' in data && data.ok) {
        if (loggedInUser.value !== ck.username)
          loggedInUser.value = ck.username
        // console.log('User is logged in')
      } else if (data.unavailable) {
        // server database briefly unavailable: keep the session, check again later
      } else {
        router.push('/login')
        // console.log('User is not logged in')
      }
    })
    .catch((error) => {
      console.log(
        'There has been a problem with your fetch operation: ' + error.message
      )
    })
}

const parseCookie = (str) => {
  if (str === '') return {}
  return str
    .split(';')
    .map((v) => v.split('='))
    .reduce((acc, v) => {
      acc[decodeURIComponent(v[0].trim())] = decodeURIComponent(v[1].trim())
      return acc
    }, {})
}

// Provide the setLoggedInUser function to be used by child components
provide('setLoggedInUser', setLoggedInUser)
</script>

<style>
html {
  overflow-y: auto;
}
</style>
