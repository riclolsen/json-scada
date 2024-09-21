<template>
    <div class="system-settings-tab">
      <h2 class="text-h5 mb-4">{{ $t('admin.systemSettings.title') }}</h2>
      
      <v-form @submit.prevent="saveSettings">
        <v-text-field
          v-model="settings.siteName"
          :label="$t('admin.systemSettings.siteName')"
          required
        ></v-text-field>
  
        <v-select
          v-model="settings.defaultLanguage"
          :items="languageOptions"
          :label="$t('admin.systemSettings.defaultLanguage')"
          required
        ></v-select>
  
        <v-switch
          v-model="settings.enableNotifications"
          :label="$t('admin.systemSettings.enableNotifications')"
        ></v-switch>
  
        <v-text-field
          v-model="settings.dataRetentionDays"
          type="number"
          :label="$t('admin.systemSettings.dataRetentionDays')"
          required
        ></v-text-field>
  
        <v-btn
          type="submit"
          color="primary"
          class="mt-4"
        >
          {{ $t('admin.systemSettings.saveSettings') }}
        </v-btn>
      </v-form>
    </div>
  </template>
  
  <script setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  
  const { t } = useI18n();
  
  const languageOptions = [
    { title: 'English', value: 'en' },
    { title: 'Español', value: 'es' },
    { title: 'Français', value: 'fr' },
  ];
  
  const settings = ref({
    siteName: 'SCADA/IOT Dashboard',
    defaultLanguage: 'en',
    enableNotifications: true,
    dataRetentionDays: 30,
  });
  
  const saveSettings = () => {
    // Implement save settings logic
    console.log('Save settings:', settings.value);
  };
  </script>