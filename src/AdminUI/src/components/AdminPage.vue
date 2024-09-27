<template>
  <v-container fluid class="admin-page">
    <v-card class="fill-height d-flex flex-column">
      <v-tabs v-model="activeTab" color="primary" align-tabs="center">
        <v-tab :value="1" @click="fetchUsersAndRoles">{{
          $t('admin.tabs.userManagement') }}</v-tab>
        <v-tab :value="2" @click="fetchRoles">{{ $t('admin.tabs.rolesManagement') }}</v-tab>
        <v-tab :value="3" @click="fetchProtocolDriverInstances">{{ $t('admin.tabs.protocolDriverInstances') }}</v-tab>
        <v-tab :value="4">{{ $t('admin.tabs.systemSettings') }}</v-tab>
        <v-tab :value="5">{{ $t('admin.tabs.logs') }}</v-tab>
      </v-tabs>

      <v-card-text>
        <v-window v-model="activeTab">
          <v-window-item :value="1">
            <user-management-tab ref="userManagementTabRef" />
          </v-window-item>

          <v-window-item :value="2">
            <roles-management-tab ref="rolesManagementTabRef" />
          </v-window-item>

          <v-window-item :value="3">
            <protocol-driver-instances-tab ref="protocolDriverInstancesTabRef" />
          </v-window-item>

          <v-window-item :value="4">
            <system-settings-tab ref="systemSettingsTabRef" />
          </v-window-item>

          <v-window-item :value="5">
            <logs-tab ref="logsTab" />
          </v-window-item>
        </v-window>
      </v-card-text>
    </v-card>
  </v-container>
</template>


<script setup>
import UserManagementTab from './UserManagementTab.vue';
import RolesManagementTab from './RolesManagementTab.vue';
import ProtocolDriverInstancesTab from './ProtocolDriverInstancesTab.vue';
import SystemSettingsTab from './SystemSettingsTab.vue';
import LogsTab from './LogsTab.vue';
import { ref } from 'vue';

const userManagementTabRef = ref(1);
const rolesManagementTabRef = ref(2);
const protocolDriverInstancesTabRef = ref(3);
const systemSettingsTabRef = ref(4);
const logsTab = ref(5);
const activeTab = ref(1);

const fetchUsersAndRoles = async () => {
  if (userManagementTabRef?.value?.fetchUsers)
    await userManagementTabRef.value.fetchUsers();
  if (userManagementTabRef?.value?.fetchRoles)
    await userManagementTabRef.value.fetchRoles();
}

const fetchRoles = async () => {
  if (rolesManagementTabRef?.value?.fetchRoles)
    await rolesManagementTabRef.value.fetchRoles();
}

const fetchProtocolDriverInstances = async () => {
  if (protocolDriverInstancesTabRef?.value?.fetchProtocolDriverInstances)
    await protocolDriverInstancesTabRef.value.fetchProtocolDriverInstances();
}

</script>

<style scoped>
.admin-page {
  padding-top: 2rem;
  padding-bottom: 2rem;
}
</style>
