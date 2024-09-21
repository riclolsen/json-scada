<template>
    <div class="logs-tab">
      <h2 class="text-h5 mb-4">{{ $t('admin.logs.title') }}</h2>
      
      <v-card>
        <v-card-title>
          <v-text-field
            v-model="search"
            :label="$t('admin.logs.search')"
            prepend-icon="mdi-magnify"
            single-line
            hide-details
          ></v-text-field>
        </v-card-title>
        
        <v-data-table
          :headers="headers"
          :items="logs"
          :search="search"
          :items-per-page="10"
        >
          <template #[`item.level`]="{ item }">
            <v-chip
              :color="getLevelColor(item.level)"
              text-color="white"
            >
              {{ item.level }}
            </v-chip>
          </template>
        </v-data-table>
      </v-card>
    </div>
  </template>
  
  <script setup>
  import { ref, computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  
  const { t } = useI18n();
  
  const search = ref('');
  
  const headers = computed(() => [
    { title: t('admin.logs.headers.timestamp'), align: 'start', key: 'timestamp' },
    { title: t('admin.logs.headers.level'), key: 'level' },
    { title: t('admin.logs.headers.message'), key: 'message' },
  ]);
  
  const logs = ref([
    { timestamp: '2023-05-01 10:00:00', level: 'INFO', message: 'System started' },
    { timestamp: '2023-05-01 10:05:00', level: 'WARNING', message: 'High CPU usage detected' },
    { timestamp: '2023-05-01 10:10:00', level: 'ERROR', message: 'Database connection failed' },
    { timestamp: '2023-05-01 10:15:00', level: 'INFO', message: 'New user registered' },
    { timestamp: '2023-05-01 10:20:00', level: 'DEBUG', message: 'Cache cleared' },
  ]);
  
  const getLevelColor = (level) => {
    const colors = {
      INFO: 'info',
      WARNING: 'warning',
      ERROR: 'error',
      DEBUG: 'success',
    };
    return colors[level] || 'grey';
  };
  </script>