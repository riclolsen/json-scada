<template>
  <v-container fluid class="admin-page ma-0 pa-0">
    <v-card class="fill-height d-flex flex-column elevation-0">
      <v-tabs v-model="activeTab" color="primary" align-tabs="center">
        <v-tab :value="1" @click="fetchUsersAndRoles"><v-icon>mdi-account-multiple</v-icon> {{
          $t('admin.tabs.userManagement') }}</v-tab>
        <v-tab :value="2" @click="fetchRoles"><v-icon>mdi-shield-account</v-icon> {{ $t('admin.tabs.rolesManagement') }}</v-tab>
        <v-tab :value="3" @click="fetchProtocolDriverInstances"><v-icon>mdi-cogs</v-icon> {{ $t('admin.tabs.protocolDriverInstances') }}</v-tab>
        <v-tab :value="4" @click="fetchProtocolConnections"><v-icon>mdi-lan-connect</v-icon> {{ $t('admin.tabs.protocolConnections') }}</v-tab>
        <v-tab :value="5"><v-icon>mdi-cog</v-icon> {{ $t('admin.tabs.systemSettings') }}</v-tab>
        <v-tab :value="6"><v-icon>mdi-file-document-outline</v-icon> {{ $t('admin.tabs.logs') }}</v-tab>
        <v-tab :value="7" @click="fetchUserActions"><v-icon>mdi-account-clock</v-icon> {{ $t('admin.tabs.userActions') }}</v-tab>
      </v-tabs>

      <v-card-text class="ma-0 pa-0">
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
            <protocol-connections-tab ref="protocolConnectionsTabRef" />
          </v-window-item>

          <v-window-item :value="5">
            <system-settings-tab ref="systemSettingsTabRef" />
          </v-window-item>

          <v-window-item :value="6">
            <logs-tab ref="logsTab" />
          </v-window-item>

          <v-window-item :value="7">
            <user-actions-tab ref="userActionsTabRef" />
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
import ProtocolConnectionsTab from './ProtocolConnectionsTab.vue';
import SystemSettingsTab from './SystemSettingsTab.vue';
import LogsTab from './LogsTab.vue';
import UserActionsTab from './UserActions.vue';
import { ref } from 'vue';

const userManagementTabRef = ref(null);
const rolesManagementTabRef = ref(null);
const protocolDriverInstancesTabRef = ref(null);
const protocolConnectionsTabRef = ref(null);
const systemSettingsTabRef = ref(null);
const logsTab = ref(null);
const userActionsTabRef = ref(null);
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

const fetchProtocolConnections = async () => {
  if (protocolConnectionsTabRef?.value?.fetchProtocolConnections)
    await protocolConnectionsTabRef.value.fetchProtocolConnections();
}

const fetchUserActions = async () => {
  if (userActionsTabRef?.value?.fetchUserActions)
    await userActionsTabRef.value.fetchUserActions();
}

</script>

<style scoped>
.admin-page {
  padding-top: 2rem;
  padding-bottom: 2rem;
}
</style>
