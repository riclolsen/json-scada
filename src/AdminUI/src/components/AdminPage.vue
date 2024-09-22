<template>
  <v-container fluid class="admin-page">
    <v-card class="fill-height d-flex flex-column">
      <v-tabs v-model="activeTab" color="primary" align-tabs="center">
        <v-tab :value="1">{{ $t('admin.tabs.userManagement') }}</v-tab>
        <v-tab :value="2">{{ $t('admin.tabs.rolesManagement') }}</v-tab>
        <v-tab :value="3">{{ $t('admin.tabs.systemSettings') }}</v-tab>
        <v-tab :value="4">{{ $t('admin.tabs.logs') }}</v-tab>
      </v-tabs>

      <v-card-text>
        <v-window v-model="activeTab">
          <v-window-item :value="1">
            <user-management-tab />
          </v-window-item>

          <v-window-item :value="2">
            <roles-management-tab />
          </v-window-item>

          <v-window-item :value="3">
            <system-settings-tab />
          </v-window-item>

          <v-window-item :value="4">
            <logs-tab />
          </v-window-item>
        </v-window>
      </v-card-text>
    </v-card>
  </v-container>
</template>

<script setup>
import { ref } from 'vue';
import UserManagementTab from './UserManagementTab.vue';
import RolesManagementTab from './RolesManagementTab.vue';
import SystemSettingsTab from './SystemSettingsTab.vue';
import LogsTab from './LogsTab.vue';

const activeTab = ref(1);
</script>

<style scoped>
.admin-page {
  padding-top: 2rem;
  padding-bottom: 2rem;
}
</style>