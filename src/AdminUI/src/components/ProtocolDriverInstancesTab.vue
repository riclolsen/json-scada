<template>
  <v-container fluid class="protocol-driver-instances-tab">
    <v-btn
      color="primary"
      size="small"
      class="mt-0 me-2"
      @click="openAddProtocolDriverInstanceDialog"
      >{{ $t('admin.protocolDriverInstances.addProtocolDriverInstance') }}
      <v-icon>mdi-plus</v-icon>
    </v-btn>

    <v-btn
      color="secondary"
      size="small"
      class="mt-0"
      @click="fetchProtocolDriverInstances"
    >
      {{ $t('common.refresh') }}
      <v-icon>mdi-refresh</v-icon>
    </v-btn>

    <v-data-table
      :headers="headers"
      :items="driverInstances"
      :items-per-page="5"
      class="mt-4 elevation-1"
      :load-children="fetchProtocolDriverInstances"
      :items-per-page-text="$t('common.itemsPerPageText')"
    >
      <template #[`item.enabled`]="{ item }">
        <v-icon v-if="item.enabled" color="green">mdi-check</v-icon>
        <v-icon v-else color="red">mdi-close</v-icon>
      </template>
      <template #[`item.stats`]="{ item }">
        <span class="text-caption">{{ item.stats }}</span>
      </template>
      <template #[`item.actions`]="{ item }">
        <v-icon
          size="small"
          class="me-2"
          @click="openEditProtocolDriverInstanceDialog(item)"
        >
          mdi-pencil
        </v-icon>
        <v-icon
          size="small"
          @click="openDeleteProtocolDriverInstanceDialog(item)"
        >
          mdi-delete
        </v-icon>
      </template>
      <template #[`item.running`]="{ item }">
        <v-icon v-if="item.running" color="green">mdi-check</v-icon>
        <v-icon v-else color="red">mdi-close</v-icon>
      </template>
      <template #[`item.process`]="{ item }">
        <v-icon
          v-if="!item.running"
          disabled
          size="small"
          class="me-2"
          @click="startProtocolDriverInstance(item)"
        >
          mdi-play
        </v-icon>
        <v-icon
          v-if="item.running"
          disabled
          size="small"
          class="me-2"
          @click="stopProtocolDriverInstance(item)"
        >
          mdi-stop
        </v-icon>
      </template>
    </v-data-table>
    <div>
      <v-chip v-if="error" color="red darken-1">{{
        $t('common.error')
      }}</v-chip>
    </div>
  </v-container>

  <v-dialog v-model="dialogEditInstance" scrollable max-width="500px">
    <v-card>
      <v-card-title>
        <span class="text-h5">{{
          $t('admin.protocolDriverInstances.editInstance')
        }}</span>
      </v-card-title>
      <v-card-text>
        <v-container>
          <v-select
            required
            v-model="editedInstance.protocolDriver"
            :items="driverNameItems"
            item-title="name"
            variant="outlined"
            :label="$t('admin.protocolDriverInstances.headers.protocolDriver')"
            @update:model-value="onDriverNameChange"
          ></v-select>
          <v-text-field
            v-model="editedInstance.protocolDriverInstanceNumber"
            variant="outlined"
            :label="
              $t(
                'admin.protocolDriverInstances.headers.protocolDriverInstanceNumber'
              )
            "
            type="number"
            disabled
          ></v-text-field>
          <v-switch
            v-model="editedInstance.enabled"
            inset
            color="primary"
            :label="`${$t('common.enabled')}${
              editedInstance.enabled ? $t('common.true') : $t('common.false')
            }`"
            class="mb-0"
          ></v-switch>
          <v-select
            v-model="editedInstance.logLevel"
            :items="[
              { text: $t('common.logLevels.minimum'), value: 0 },
              { text: $t('common.logLevels.basic'), value: 1 },
              { text: $t('common.logLevels.detailed'), value: 2 },
              { text: $t('common.logLevels.maximum'), value: 3 },
            ]"
            item-title="text"
            item-value="value"
            :label="$t('admin.protocolDriverInstances.headers.logLevel')"
            variant="outlined"
          ></v-select>
          <v-select
            v-model="editedInstance.nodeNames"
            :items="nodeNames"
            item-title="name"
            variant="outlined"
            chips
            closable-chips
            small-chips
            :label="
              $t('admin.protocolDriverInstances.headers.allowedNodesList')
            "
            multiple
          ></v-select>
          <v-btn
            class="mt-4 me-2"
            color="blue"
            text
            variant="tonal"
            @click="dialogAddNode = true"
          >
            {{ $t('admin.protocolDriverInstances.addNewNode') }}
            <v-icon>mdi-plus</v-icon>
          </v-btn>
        </v-container>
      </v-card-text>

      <v-card-actions>
        <v-chip v-if="error" color="red darken-1">{{
          $t('common.error')
        }}</v-chip>
        <v-spacer></v-spacer>
        <v-btn
          color="orange darken-1"
          text
          variant="tonal"
          @click="dialogEditInstance = false"
        >
          {{ $t('common.cancel') }}
        </v-btn>
        <v-btn
          color="blue darken-1"
          text
          variant="tonal"
          @click="updateOrCreateProtocolDriverInstance"
        >
          {{ $t('common.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="dialogAddNode" max-width="400" class="pa-8">
    <v-card>
      <v-card-title class="headline">
        {{ $t('admin.protocolDriverInstances.addNewNode') }}
      </v-card-title>

      <v-card-title class="headline">
        <v-text-field
          autofocus
          :label="$t('admin.protocolDriverInstances.newNodeName')"
          v-model="newNode"
        ></v-text-field>
      </v-card-title>

      <v-card-actions>
        <v-spacer></v-spacer>

        <v-btn
          color="orange darken-1"
          text
          variant="tonal"
          @click="dialogAddNode = false"
        >
          {{ $t('common.cancel') }}
        </v-btn>

        <v-btn
          color="blue darken-1"
          text
          variant="tonal"
          @click="addNewNode(newNode)"
        >
          {{ $t('common.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="dialogDeleteInstance" max-width="400px">
    <v-card>
      <v-card-title>{{
        $t('admin.protocolDriverInstances.deleteInstance')
      }}</v-card-title>
      <v-card-text>
        {{ $t('admin.protocolDriverInstances.deleteInstanceConfirm') }}
      </v-card-text>
      <v-card-text>
        {{
          editedInstance.protocolDriver +
          ' / ' +
          editedInstance.protocolDriverInstanceNumber
        }}
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="orange darken-1"
          text
          variant="tonal"
          @click="dialogDeleteInstance = false"
        >
          {{ $t('common.cancel') }}
        </v-btn>
        <v-btn
          color="red darken-1"
          text
          variant="tonal"
          @click="deleteDriverInstance(editedInstance)"
        >
          {{ $t('common.delete') }}
        </v-btn>
      </v-card-actions>
      <v-chip v-if="error" color="red darken-1">{{
        $t('common.error')
      }}</v-chip>
    </v-card>
  </v-dialog>
</template>
<script setup>
  import { ref, computed, onMounted, onUnmounted } from 'vue'
  import { useI18n } from 'vue-i18n'

  const { t } = useI18n()
  const headers = computed(() => [
    { title: '#', key: 'id', align: 'end' },
    {
      title: t('admin.protocolDriverInstances.headers.protocolDriver'),
      align: 'start',
      key: 'protocolDriver',
    },
    {
      title: t(
        'admin.protocolDriverInstances.headers.protocolDriverInstanceNumber'
      ),
      align: 'end',
      key: 'protocolDriverInstanceNumber',
    },
    {
      title: t('admin.protocolDriverInstances.headers.enabled'),
      key: 'enabled',
    },
    {
      title: t('admin.protocolDriverInstances.headers.logLevel'),
      key: 'logLevel',
    },
    {
      title: t('admin.protocolDriverInstances.headers.allowedNodesList'),
      key: 'nodesText',
    },
    {
      title: t('admin.protocolDriverInstances.headers.actions'),
      key: 'actions',
      sortable: false,
    },
    {
      title: t('admin.protocolDriverInstances.headers.running'),
      key: 'running',
    },
    {
      title: t(
        'admin.protocolDriverInstances.headers.activeNodeKeepAliveTimeTag'
      ),
      key: 'localTimeUpdate',
    },
    {
      title: t('admin.protocolDriverInstances.headers.stats'),
      key: 'stats',
      sortable: false,
    },
    {
      title: t('admin.protocolDriverInstances.headers.process'),
      key: 'process',
      sortable: false,
    },
  ])

  const dialogEditInstance = ref(false)
  const editedInstanceDefault = ref({
    protocolDriver: '',
    protocolDriverInstanceNumber: '',
    enabled: true,
    logLevel: 1,
    activeNodeKeepAliveTimeTag: new Date(0),
    keepProtocolRunningWhileInactive: false,
    softwareVersion: '',
    nodeNames: [],
    activeNodeName: '',
    stats: '',
    id: -1,
    nodesText: '',
    running: false,
    localTimeUpdate: '',
    process: '',
  })
  const editedInstance = ref({ ...editedInstanceDefault.value })

  const dialogDeleteInstance = ref(false)
  const dialogAddNode = ref(false)
  const newNode = ref('')
  const nodeNames = ref([])
  const driverNameItems = [
    'IEC60870-5-104',
    'IEC60870-5-104_SERVER',
    'IEC60870-5-101',
    'IEC60870-5-101_SERVER',
    'IEC61850',
    'IEC61850_SERVER',
    'DNP3',
    'DNP3_SERVER',
    'MQTT-SPARKPLUG-B',
    'OPC-UA',
    'OPC-UA_SERVER',
    'OPC-DA',
    'OPC-DA_SERVER',
    'PLCTAG',
    'PLC4X',
    'TELEGRAF-LISTENER',
    'ICCP',
    'ICCP_SERVER',
    'I104M',
    'PI_DATA_ARCHIVE_INJECTOR',
    'PI_DATA_ARCHIVE_CLIENT',
  ]
  const driverInstances = ref([])
  const error = ref(false)

  onMounted(() => {
    fetchProtocolDriverInstances()
    document.documentElement.style.overflowY = 'scroll'
  })

  onUnmounted(() => {
    document.documentElement.style.overflowY = 'auto'
  })

  async function fetchNodes() {
    try {
      const res = await fetch('/Invoke/auth/listNodes')
      const json = await res.json()
      if (json.error) {
        console.warn(json)
        return
      }
      nodeNames.value = json
    } catch (err) {
      console.warn(err)
      // error.value = true;
    }
  }

  async function fetchProtocolDriverInstances() {
    await fetchNodes()
    try {
      const res = await fetch('/Invoke/auth/listProtocolDriverInstances')
      const json = await res.json()
      if (json.error) {
        console.wanr(json)
        return
      }
      json.forEach((item, index) => {
        item.id = index + 1
        item.nodesText = item.nodeNames.join(', ')
        item.localTimeUpdate =
          item.activeNodeName +
          ' ' +
          (item.activeNodeKeepAliveTimeTag
            ? new Date(item.activeNodeKeepAliveTimeTag).toLocaleString()
            : '')
        item.running =
          new Date(item.activeNodeKeepAliveTimeTag).getTime() >
          new Date().getTime() - 10000
      })
      driverInstances.value = json
    } catch (err) {
      console.warn(err)
      // error.value = true;
    }
  }

  async function startProtocolDriverInstance(item) {
    try {
      const res = await fetch('/Invoke/auth/startProtocolDriverInstance', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          protocolDriver: item.protocolDriver,
          protocolDriverInstanceNumber: item.protocolDriverInstanceNumber,
        }),
      })
      const json = await res.json()
      if (json.error) {
        console.warn(json)
        error.value = true
      }
    } catch (err) {
      console.warn(err)
      error.value = true
    }
    setTimeout(() => {
      fetchProtocolDriverInstances() // Refresh instances
    }, 3000)
  }

  async function stopProtocolDriverInstance(item) {
    try {
      const res = await fetch('/Invoke/auth/stopProtocolDriverInstance', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          protocolDriver: item.protocolDriver,
          protocolDriverInstanceNumber: item.protocolDriverInstanceNumber,
        }),
      })
      const json = await res.json()
      if (json.error) {
        console.warn(json)
        error.value = true
      }
    } catch (err) {
      console.warn(err)
      error.value = true
    }
    setTimeout(() => {
      fetchProtocolDriverInstances() // Refresh instances
    }, 3000)
  }

  // make sure the protocol driver instance number is unique for the protocol driver
  async function onDriverNameChange(name) {
    let instanceNumber = 1
    for (const instance of driverInstances.value) {
      if (instance.protocolDriver === name) {
        instanceNumber = instance.protocolDriverInstanceNumber + 1
        break
      }
    }
    editedInstance.value.protocolDriverInstanceNumber = instanceNumber
  }

  async function deleteDriverInstance(item) {
    try {
      const res = await fetch('/Invoke/auth/deleteProtocolDriverInstance', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          protocolDriver: item.protocolDriver,
          protocolDriverInstanceNumber: item.protocolDriverInstanceNumber,
          _id: item._id,
        }),
      })
      const json = await res.json()
      if (json.error) {
        console.warn(json)
        return
      }
      dialogDeleteInstance.value = false
    } catch (err) {
      console.warn(err)
      error.value = true
    }
    fetchProtocolDriverInstances() // refreshes instances
  }

  async function createDriverInstance() {
    try {
      const dup = Object.assign({}, editedInstance.value)
      if (dup._id) {
        delete dup._id
      }
      const res = await fetch('/Invoke/auth/createProtocolDriverInstance', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({}),
      })
      const json = await res.json()
      if (json.error || !json._id) {
        console.warn(json)
        error.value = true
        return
      }
      editedInstance.value._id = json._id
      await updateProtocolDriverInstance()
      dialogEditInstance.value = false
    } catch (err) {
      console.warn(err)
      error.value = true
    }
    fetchProtocolDriverInstances() // Refresh instances
  }

  async function updateProtocolDriverInstance() {
    if (editedInstance.value.protocolDriver === '') return
    const dup = Object.assign({}, editedInstance.value)
    if ('id' in dup) {
      delete dup.id
    }
    if ('nodesText' in dup) {
      delete dup.nodesText
    }
    if ('running' in dup) {
      delete dup.running
    }
    if ('localTimeUpdate' in dup) {
      delete dup.localTimeUpdate
    }
    if ('process' in dup) {
      delete dup.process
    }
    try {
      const res = await fetch('/Invoke/auth/updateProtocolDriverInstance', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(dup),
      })
      const json = await res.json()
      if (json.error) {
        console.log(json)
        error.value = true
        return
      }
      dialogEditInstance.value = false
    } catch (err) {
      console.warn(err)
      error.value = true
    }
    fetchProtocolDriverInstances() // Refresh instances
  }

  async function updateOrCreateProtocolDriverInstance() {
    if (editedInstance.value._id) {
      await updateProtocolDriverInstance()
    } else {
      await createDriverInstance()
    }
  }

  async function addNewNode() {
    dialogAddNode.value = false
    if (!newNode.value || newNode.value === '') return
    nodeNames.value.push(newNode.value)
    editedInstance.value.nodeNames.push(newNode.value)
    editedInstance.value.nodeNames = [
      ...new Set(editedInstance.value.nodeNames),
    ]
    newNode.value = ''
  }

  const openAddProtocolDriverInstanceDialog = () => {
    error.value = false
    editedInstance.value = Object.assign({}, editedInstanceDefault.value)
    dialogEditInstance.value = true
  }

  const openEditProtocolDriverInstanceDialog = (item) => {
    editedInstance.value = Object.assign({}, item)
    dialogEditInstance.value = true
  }

  const openDeleteProtocolDriverInstanceDialog = (item) => {
    editedInstance.value = Object.assign({}, item)
    dialogDeleteInstance.value = true
  }

  defineExpose({ fetchProtocolDriverInstances })
</script>
