<template>
  <div class="system-settings-tab">

    <v-card class="mb-4" max-width="500">
      <v-card-title>
        {{ $t('admin.systemSettings.title') }}
      </v-card-title>
      <v-card-text>

        <v-form @submit.prevent="saveSettings">

          <v-text-field v-model="settings.siteName" :label="$t('admin.systemSettings.siteName')"
            required></v-text-field>

          <v-text-field v-model="settings.dataRetentionDays" type="number"
            :label="$t('admin.systemSettings.dataRetentionDays')" required></v-text-field>

          <v-btn type="submit" color="primary" variant="tonal" class="mt-4">
            {{ $t('admin.systemSettings.saveSettings') }}
          </v-btn>
        </v-form>


      </v-card-text>
    </v-card>


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