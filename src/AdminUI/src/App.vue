<template>
  <v-app :theme="theme">
    <v-app-bar app height="48">
      <v-app-bar-title class="text-subtitle-1">
        <router-link to="/" class="text-decoration-none">
          {{ $t('app.title') }}
        </router-link>
      </v-app-bar-title>
      <v-spacer></v-spacer>
      <v-select v-model="currentLocale" :items="availableLocales" variant="outlined" density="compact" hide-details
        class="mr-2" style="max-width: 120px;"></v-select>
      <v-btn icon size="small" @click="toggleTheme">
        <v-icon>{{ theme === 'light' ? 'mdi-weather-night' : 'mdi-weather-sunny' }}</v-icon>
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
            <v-list-item-title>{{ $t('app.logout') }}</v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>
    </v-app-bar>

    <v-main class="flex-grow-1" style="overflow-y: auto; scrollbar-width: none; -ms-overflow-style: none;">
      <router-view></router-view>
    </v-main>
  </v-app>
</template>

<script setup>
import { ref, watch, provide } from 'vue';
import { useTheme } from 'vuetify';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { STORAGE_KEY } from './i18n';

const router = useRouter();
const theme = ref('dark');
const vuetifyTheme = useTheme();
const { locale, t } = useI18n();

const currentLocale = ref(locale.value);
const loggedInUser = ref("?");

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
];

const toggleTheme = () => {
  theme.value = theme.value === 'light' ? 'dark' : 'light';
  vuetifyTheme.global.name.value = theme.value;
};

const setLoggedInUser = (username) => {
  loggedInUser.value = username;
};

const logout = () => {
  loggedInUser.value = null;
  fetch('/Invoke/auth/signout', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
  })
    .then(response => response.json())
    .then(data => {
      if (data.ok) {
        router.push('/login');
      } else {
        console.error('Logout failed!');
      }
    })
    .catch(error => {
      console.error('Error during logout:', error);
    });
};

watch(currentLocale, (newLocale) => {
  locale.value = newLocale;
  localStorage.setItem(STORAGE_KEY, newLocale);
});

// Provide the setLoggedInUser function to be used by child components
provide('setLoggedInUser', setLoggedInUser);
</script>

<style>
html {
  overflow-y: auto
}
</style>

<style scoped>
.text-decoration-none {
  text-decoration: none;
  color: inherit;
}
</style>