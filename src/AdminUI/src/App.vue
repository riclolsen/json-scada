<template>
  <v-app :theme="theme">
    <v-app-bar app height="48">
      <v-app-bar-title class="text-subtitle-1">
        <router-link to="/" class="text-decoration-none">
          <v-tooltip :text="version">
            <template v-slot:activator="{ props }">
              <span v-bind="props">{{ $t('app.title') }}</span>
            </template>
          </v-tooltip>
        </router-link>
      </v-app-bar-title>
      <v-spacer></v-spacer>
      <v-select v-model="currentLocale" :items="availableLocales" variant="outlined" density="compact" hide-details
        class="mr-2" style="max-width: 120px"></v-select>
      <v-btn icon size="small" @click="toggleTheme">
        <v-icon>{{
          theme === 'light' ? 'mdi-weather-night' : 'mdi-weather-sunny'
        }}</v-icon>
      </v-btn>
      <v-menu v-if="loggedInUser" offset-y>
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" text class="ml-2">
            <v-icon right size="small">mdi-account-circle</v-icon>
            {{ loggedInUser }}
          </v-btn>
        </template>
        <v-list>
          <v-list-item @click="logout">
            <v-list-item-title>{{ $t('app.logout') }} <v-icon dark> mdi-logout </v-icon>
            </v-list-item-title>
          </v-list-item>
          <v-list-item v-if="!isLDAPUser" @click="openChangePasswordDialog">
            <v-list-item-title>{{ $t('login.changePassword') }}
              <v-icon dark> mdi-lock-reset </v-icon>
            </v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>
    </v-app-bar>

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
          <v-alert v-if="errorMessage" type="error" dense>
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
import { ref, watch, provide, onMounted } from 'vue'
import { useTheme } from 'vuetify'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { STORAGE_KEY } from './i18n'

const version = ref('v0.51-alpha')
const router = useRouter()
const theme = ref('dark')
const vuetifyTheme = useTheme()
const { locale, t } = useI18n()

const currentLocale = ref(locale.value)
const loggedInUser = ref('')
const isLDAPUser = ref(false)
const showChangePasswordDialog = ref(false)
const newPassword = ref('')
const confirmNewPassword = ref('')
const currentPassword = ref('')
const errorMessage = ref('')

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
  checkLogin()
  setInterval(checkLogin, 15000)
})

const toggleTheme = () => {
  theme.value = theme.value === 'light' ? 'dark' : 'light'
  vuetifyTheme.global.name.value = theme.value
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
  localStorage.setItem(STORAGE_KEY, newLocale)
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

<style scoped>
.text-decoration-none {
  text-decoration: none;
  color: inherit;
}
</style>
