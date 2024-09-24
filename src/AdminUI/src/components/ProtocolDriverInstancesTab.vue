<template>
  <v-container fluid class="protocol-driver-instances-tab">

    <v-btn color="primary" class="mt-4" @click="openAddProtocolDriverInstanceDialog">{{
      $t('admin.protocolDriverInstances.addProtocolDriverInstance') }}
      <v-icon>mdi-plus</v-icon>
    </v-btn>
    <v-data-table :headers="headers" :items="driverInstances" :items-per-page="5" class="elevation-1"
      :load-children="fetchDriverInstances" :items-per-page-text="$t('common.itemsPerPageText')">
      <template #[`item.enabled`]="{ item }">
        <v-icon v-if="item.enabled" color="green">mdi-check</v-icon>
        <v-icon v-else color="red">mdi-close</v-icon>
      </template>
      <template #[`item.actions`]="{ item }">
        <v-icon size="small" class="me-2" @click="openEditProtocolDriverInstanceDialog(item)">
          mdi-pencil
        </v-icon>
        <v-icon v-if="item.name !== 'admin'" size="small" @click="openDeleteProtocolDriverInstanceDialog(item)">
          mdi-delete
        </v-icon>
      </template>
    </v-data-table>

  </v-container>

</template>
<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const headers = computed(() => [
  { title: '#', key: 'id' },
  { title: t('admin.protocolDriverInstances.headers.protocolDriver'), align: 'start', key: 'protocolDriver' },
  { title: t('admin.protocolDriverInstances.headers.protocolDriverInstanceNumber'), key: 'protocolDriverInstanceNumber' },
  { title: t('admin.protocolDriverInstances.headers.enabled'), key: 'enabled' },
  { title: t('admin.protocolDriverInstances.headers.logLevel'), key: 'logLevel' },
  { title: t('admin.protocolDriverInstances.headers.allowedNodesList'), key: 'nodesText' },
  { title: t('admin.protocolDriverInstances.headers.actions'), key: 'actions', sortable: false },
]);

const dialogAddNode = ref(false);
const dialogDelInst = ref(false);
const active = ref([]);
const newNode = ref("");
const nodeNames = ref([]);
const driverNameItems = [
  "IEC60870-5-104",
  "IEC60870-5-104_SERVER",
  "IEC60870-5-101",
  "IEC60870-5-101_SERVER",
  "IEC61850",
  "IEC61850_SERVER",
  "DNP3",
  "DNP3_SERVER",
  "MQTT-SPARKPLUG-B",
  "OPC-UA",
  "OPC-UA_SERVER",
  "OPC-DA",
  "OPC-DA_SERVER",
  "PLCTAG",
  "PLC4X",
  "TELEGRAF-LISTENER",
  "ICCP",
  "ICCP_SERVER",
  "I104M",
  "PI_DATA_ARCHIVE_INJECTOR",
  "PI_DATA_ARCHIVE_CLIENT",
];
const driverInstances = ref([]);

const selected = computed(() => {
  if (!active.value.length) return undefined;
  return driverInstances.value.find((item) => item.id === active.value[0]);
});

onMounted(() => {
  fetchDriverInstances();
  document.documentElement.style.overflowY = 'scroll';
});

onUnmounted(() => {
  document.documentElement.style.overflowY = 'auto';  
});

async function fetchNodes() {
  try {
    const res = await fetch("/Invoke/auth/listNodes");
    const json = await res.json();
    nodeNames.value = json;
  } catch (err) {
    console.warn(err);
  }
}

async function fetchDriverInstances() {
  await fetchNodes();
  try {
    const res = await fetch("/Invoke/auth/listProtocolDriverInstances");
    const json = await res.json();
    json.forEach((item, index) => {
      item.id = index + 1;
      item.nodesText = item.nodeNames.join(', ');
      item.driverNameInstance = `${item.protocolDriver} ( ${item.protocolDriverInstanceNumber} )`;
    });
    driverInstances.value = json;
  } catch (err) {
    console.warn(err);
  }
}

async function deleteDriverInstance() {
  try {
    const res = await fetch("/Invoke/auth/deleteProtocolDriverInstance", {
      method: "post",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        protocolDriver: selected.value.protocolDriver,
        protocolDriverInstanceNumber: selected.value.protocolDriverInstanceNumber,
        _id: selected.value._id,
      }),
    });
    const json = await res.json();
    if (json.error) console.log(json);
    fetchDriverInstances(); // refreshes instances
  } catch (err) {
    console.warn(err);
  }
}

async function createDriverInstance() {
  try {
    const res = await fetch("/Invoke/auth/createProtocolDriverInstance", {
      method: "post",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        protocolDriver: "IEC60870-5-104", // Default value, can be changed later
        protocolDriverInstanceNumber: 1, // Default value, can be changed later
        enabled: true,
        logLevel: "INFO",
        nodeNames: [],
      }),
    });
    const json = await res.json();
    if (json.error) console.log(json);
    fetchDriverInstances(); // Refresh instances
  } catch (err) {
    console.warn(err);
  }
}

async function updateProtocolDriverInstance() {
  if (!selected.value) return;
  try {
    const res = await fetch("/Invoke/auth/updateProtocolDriverInstance", {
      method: "post",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify(selected.value),
    });
    const json = await res.json();
    if (json.error) console.log(json);
    fetchDriverInstances(); // Refresh instances
  } catch (err) {
    console.warn(err);
  }
}

async function addNewNode() {
  if (!selected.value || !newNode.value) return;
  try {
    const updatedInstance = {
      ...selected.value,
      nodeNames: [...selected.value.nodeNames, newNode.value],
    };
    const res = await fetch("/Invoke/auth/updateProtocolDriverInstance", {
      method: "post",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify(updatedInstance),
    });
    const json = await res.json();
    if (json.error) console.log(json);
    newNode.value = ""; // Clear the input
    fetchDriverInstances(); // Refresh instances
  } catch (err) {
    console.warn(err);
  }
}

const openAddProtocolDriverInstanceDialog = () => {
  dialogAddNode.value = true;
};

const openEditProtocolDriverInstanceDialog = (item) => {
  selected.value = item;
  dialogAddNode.value = true;
};

</script>
