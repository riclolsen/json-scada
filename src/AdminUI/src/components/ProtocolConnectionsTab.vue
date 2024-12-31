<template>
  <v-container fluid class="protocol-connections-tab">
    <v-btn
      class="mt-0 me-2"
      size="small"
      color="blue"
      @click="openAddProtocolConnectionDialog"
    >
      {{ $t('admin.protocolConnections.addProtocolConnection') }}
      <v-icon> mdi-plus </v-icon>
    </v-btn>

    <v-btn
      color="secondary"
      size="small"
      class="mt-0"
      @click="fetchProtocolConnections"
    >
      {{ $t('common.refresh') }}
      <v-icon>mdi-refresh</v-icon>
    </v-btn>

    <v-data-table
      :headers="headers"
      :items="protocolConnections"
      :items-per-page="5"
      class="mt-4 elevation-1"
      :load-children="fetchProtocolConnections"
      :items-per-page-text="$t('common.itemsPerPageText')"
    >
      <template #[`item.icon`]="{ item }">
        <v-icon v-if="!item.children"> mdi-swap-horizontal </v-icon>
      </template>
      <template #[`item.protocolDriver`]="{ item }">
        <span class="text-caption"
          >{{ item.protocolDriver }} <br />
          {{ item.protocolDriverInstanceNumber }}</span
        >
      </template>
      <template #[`item.stats`]="{ item }">
        <span class="text-caption">{{ item.stats }}</span>
      </template>
      <template #[`item.enabled`]="{ item }">
        <v-icon v-if="item.enabled" color="green">mdi-check</v-icon>
        <v-icon v-else color="red">mdi-close</v-icon>
      </template>

      <template #[`item.actions`]="{ item }">
        <v-icon
          size="small"
          class="me-2"
          @click="openEditProtocolConnectionDialog(item)"
        >
          mdi-pencil
        </v-icon>
        <v-icon size="small" @click="openDeleteProtocolConnectionDialog(item)">
          mdi-delete
        </v-icon>
      </template>
    </v-data-table>
  </v-container>

  <v-dialog scrollable v-model="dialogEditConnection" max-width="750px">
    <v-card class="mx-n8">
      <v-card-title>
        <span class="text-h5">{{
          $t('admin.protocolConnections.editProtocolConnection')
        }}</span>
      </v-card-title>

      <v-card-text class="">
        <v-text-field
          prepend-inner-icon="mdi-lan-connect"
          type="text"
          variant="outlined"
          clearable
          :label="$t('admin.protocolConnections.protocolConnectionName')"
          v-model="editedConnection.name"
        ></v-text-field>

        <v-text-field
          prepend-inner-icon="mdi-numeric"
          type="number"
          variant="outlined"
          min="1"
          clearable
          :label="$t('admin.protocolConnections.protocolConnectionNumber')"
          v-model="editedConnection.protocolConnectionNumber"
          class="pt-0 mx-auto"
        ></v-text-field>

        <v-text-field
          prepend-inner-icon="mdi-text"
          type="text"
          variant="outlined"
          clearable
          :label="$t('admin.protocolConnections.protocolConnectionDescription')"
          v-model="editedConnection.description"
          class="pt-0 mx-auto"
        ></v-text-field>

        <v-select
          prepend-inner-icon="mdi-cogs"
          :items="driverNameItems"
          variant="outlined"
          :label="$t('admin.protocolConnections.protocolDriver')"
          v-model="editedConnection.protocolDriver"
          class="pt-0"
        ></v-select>

        <v-select
          prepend-inner-icon="mdi-cogs"
          :items="driverInstancesByType[editedConnection.protocolDriver]"
          variant="outlined"
          :label="$t('admin.protocolConnections.protocolDriverInstanceNumber')"
          v-model="editedConnection.protocolDriverInstanceNumber"
          class="pt-0"
        ></v-select>

        <v-switch
          v-model="editedConnection.enabled"
          inset
          color="primary"
          :label="`${$t('common.enabled')}${
            editedConnection.enabled ? $t('common.true') : $t('common.false')
          }`"
          class="mt-n3"
        ></v-switch>

        <v-switch
          v-if="
            !['TELEGRAF-LISTENER'].includes(editedConnection.protocolDriver)
          "
          v-model="editedConnection.commandsEnabled"
          inset
          color="primary"
          :label="`${$t('admin.protocolConnections.commandsEnabled')}${
            editedConnection.commandsEnabled
              ? $t('common.true')
              : $t('common.false')
          }`"
          class="mt-n6"
        ></v-switch>

        <v-card class="mt-n4" tile variant="outlined">
          <v-card-title>
            <span class="text-h5">
              {{ $t('admin.protocolConnections.protocolConnectionParameters') }}
            </span>
          </v-card-title>
          <v-list density="compact">
            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-101_SERVER',
                  'IEC60870-5-104',
                  'IEC60870-5-104_SERVER',
                  'PLCTAG',
                  'MODBUS',
                  'DNP3',
                  'DNP3_SERVER',
                  'I104M',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :model-value="isActive"
                        :label="
                          $t('admin.protocolConnections.localLinkAddress')
                        "
                        hide-details="auto"
                        v-model="editedConnection.localLinkAddress"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t('admin.protocolConnections.localLinkAddressTitle')
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.localLinkAddressHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-101_SERVER',
                  'IEC60870-5-104',
                  'IEC60870-5-104_SERVER',
                  'PLCTAG',
                  'MODBUS',
                  'DNP3',
                  'DNP3_SERVER',
                  'I104M',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :model-value="isActive"
                        :label="
                          $t('admin.protocolConnections.remoteLinkAddress')
                        "
                        hide-details="auto"
                        v-model="editedConnection.remoteLinkAddress"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t('admin.protocolConnections.remoteLinkAddressTitle')
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{
                        $t('admin.protocolConnections.remoteLinkAddressHint')
                      }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['OPC-UA', 'MQTT-SPARKPLUG-B', 'PLC4X', 'OPC-DA'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <v-row>
                <v-col>
                  <v-select
                    v-model="editedConnection.endpointURLs"
                    :items="editedConnection.endpointURLs"
                    chips
                    closable-chips
                    :label="$t('admin.protocolConnections.remoteEndpointsUrls')"
                    multiple
                  ></v-select>
                </v-col>
                <v-col>
                  <v-btn
                    color="blue"
                    text
                    variant="tonal"
                    @click="dialogAddURL = true"
                  >
                    <v-icon dark> mdi-plus </v-icon>
                    {{ $t('admin.protocolConnections.remoteEndpointsAddNew') }}
                  </v-btn>
                </v-col>
              </v-row>
              <v-dialog v-model="dialogAddURL" max-width="450" class="pa-8">
                <v-card>
                  <v-card-title class="text-h5">
                    {{ $t('admin.protocolConnections.remoteEndpointsAddNew') }}
                  </v-card-title>

                  <v-card-title class="text-h5">
                    <v-text-field
                      autofocus
                      :label="
                        $t('admin.protocolConnections.remoteEndpointsNewUrl')
                      "
                      v-model="newURL"
                      :rules="[
                        rules.required,
                        editedConnection.protocolDriver === 'MQTT-SPARKPLUG-B'
                          ? rules.endpointMQTT
                          : editedConnection.protocolDriver === 'OPC-UA'
                          ? rules.endpointOPC
                          : rules.endpointOPCDA,
                      ]"
                    ></v-text-field>
                  </v-card-title>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn
                      color="orange darken-1"
                      text
                      variant="tonal"
                      @click="dialogAddURL = false"
                    >
                      {{ $t('common.cancel') }}
                    </v-btn>
                    <v-btn
                      color="blue darken-1"
                      text
                      variant="tonal"
                      @click="addNewURL"
                    >
                      {{ $t('common.ok') }}
                    </v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>
            </v-list-item>

            <v-list-item
              v-if="
                ['MQTT-SPARKPLUG-B', 'IEC61850', 'OPC-DA'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :model-value="isActive"
                        :label="$t('admin.protocolConnections.username')"
                        hide-details="auto"
                        v-model="editedConnection.username"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.usernameTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.usernameHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['MQTT-SPARKPLUG-B', 'IEC61850', 'OPC-DA'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :model-value="isActive"
                        :label="$t('admin.protocolConnections.password')"
                        hide-details="auto"
                        v-model="editedConnection.password"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.passwordTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.passwordHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="['OPC-UA'].includes(editedConnection.protocolDriver)"
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="string"
                        :model-value="isActive"
                        :label="$t('admin.protocolConnections.configFileName')"
                        hide-details="auto"
                        v-model="editedConnection.configFileName"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.configFileNameTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.configFileNameHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              class="ma-0"
              v-if="
                [
                  'OPC-UA',
                  'MQTT-SPARKPLUG-B',
                  'TELEGRAF-LISTENER',
                  'IEC61850',
                  'PLC4X',
                  'OPC-DA',
                  'ICCP',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-switch
                class="ma-0"
                v-model="editedConnection.autoCreateTags"
                inset
                color="primary"
                :label="`${$t('admin.protocolConnections.autoCreateTags')}${
                  editedConnection.autoCreateTags
                    ? $t('admin.protocolConnections.autoCreateTagsTrue')
                    : $t('admin.protocolConnections.autoCreateTagsFalse')
                }`"
              ></v-switch>
            </v-list-item>

            <v-list-item
              v-if="
                (['OPC-UA'].includes(editedConnection.protocolDriver) &&
                  editedConnection.autoCreateTags) ||
                ['OPC-DA', 'OPC-DA_SERVER', 'ICCP', 'ICCP_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        :model-value="isActive"
                        :label="
                          $t('admin.protocolConnections.publishingInterval')
                        "
                        hide-details="auto"
                        v-model="
                          editedConnection.autoCreateTagPublishingInterval
                        "
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t('admin.protocolConnections.publishingIntervalTitle')
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{
                        $t('admin.protocolConnections.publishingIntervalHint')
                      }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                (['OPC-UA'].includes(editedConnection.protocolDriver) &&
                  editedConnection.autoCreateTags) ||
                ['OPC-DA'].includes(editedConnection.protocolDriver)
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        :model-value="isActive"
                        :label="
                          $t('admin.protocolConnections.samplingInterval')
                        "
                        hide-details="auto"
                        v-model="editedConnection.autoCreateTagSamplingInterval"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t('admin.protocolConnections.samplingIntervalTitle')
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.samplingIntervalHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                (['OPC-UA'].includes(editedConnection.protocolDriver) &&
                  editedConnection.autoCreateTags) ||
                ['OPC-DA'].includes(editedConnection.protocolDriver)
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        :model-value="isActive"
                        :label="$t('admin.protocolConnections.serverQueueSize')"
                        hide-details="auto"
                        v-model="editedConnection.autoCreateTagQueueSize"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.serverQueueSizeTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.serverQueueSizeHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="['OPC-DA'].includes(editedConnection.protocolDriver)"
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        :model-value="isActive"
                        :label="$t('admin.protocolConnections.deadBand')"
                        hide-details="auto"
                        v-model="editedConnection.deadBand"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.deadBandTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.deadBandHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                 'OPC-UA', 
                 'OPC-DA', 
                 'OPC-DA_SERVER', 
                 'ICCP', 
                 'ICCP_SERVER'
                ].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        :model-value="isActive"
                        :label="$t('admin.protocolConnections.hoursShift')"
                        hide-details="auto"
                        v-model="editedConnection.hoursShift"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.hoursShiftTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.hoursShiftHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'OPC-UA',
                  'OPC-UA_SERVER',
                  'OPC-DA',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        :model-value="isActive"
                        :label="$t('admin.protocolConnections.timeoutMs')"
                        hide-details="auto"
                        v-model="editedConnection.timeoutMs"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.timeoutMsTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.timeoutMsHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-104',
                  'IEC61850',
                  'DNP3',
                  'I104M',
                  'PI_DATA_ARCHIVE_INJECTOR',
                  'PI_DATA_ARCHIVE_CLIENT',
                  'PLC4X',
                  'OPC-UA',
                  'OPC-DA',
                  'ICCP',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :model-value="isActive"
                        :label="$t('admin.protocolConnections.giInterval')"
                        hide-details="auto"
                        v-model="editedConnection.giInterval"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.giIntervalTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.giIntervalHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-101', 'IEC60870-5-104'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template #default="{ isActive }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :model-value="isActive"
                        :label="$t('admin.protocolConnections.testCmdInterval')"
                        hide-details="auto"
                        v-model="editedConnection.testCommandInterval"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.testCmdIntervalTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.testCmdIntervalHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-101', 'IEC60870-5-104', 'PLCTAG'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template #default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :input-value="active"
                        :label="
                          $t('admin.protocolConnections.timeSyncInterval')
                        "
                        hide-details="auto"
                        v-model="editedConnection.timeSyncInterval"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t('admin.protocolConnections.timeSyncIntervalTitle')
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.timeSyncIntervalHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.k')"
                        hide-details="auto"
                        v-model="editedConnection.k"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.kTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.kHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.w')"
                        hide-details="auto"
                        v-model="editedConnection.w"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.wTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.wHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.t0')"
                        hide-details="auto"
                        v-model="editedConnection.t0"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.t0Title') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.t0Hint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.t1')"
                        hide-details="auto"
                        v-model="editedConnection.t1"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.t1Title') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.t1Hint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.t2')"
                        hide-details="auto"
                        v-model="editedConnection.t2"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.t2Title') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.t2Hint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.t3')"
                        hide-details="auto"
                        v-model="editedConnection.t3"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.t3Title') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.t3Hint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-104',
                  'IEC60870-5-101_SERVER',
                  'IEC60870-5-104_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-select
                        :items="itemsSizeOfCOT"
                        :input-value="active"
                        hide-details="auto"
                        v-model="editedConnection.sizeOfCOT"
                        :label="$t('admin.protocolConnections.sizeOfCot')"
                      ></v-select>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.sizeOfCotTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.sizeOfCotHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-104',
                  'IEC60870-5-101_SERVER',
                  'IEC60870-5-104_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-select
                        :items="itemsSizeOfCA"
                        :input-value="active"
                        hide-details="auto"
                        v-model="editedConnection.sizeOfCA"
                        :label="$t('admin.protocolConnections.sizeOfCa')"
                      ></v-select>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.sizeOfCaTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.sizeOfCaHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-104',
                  'IEC60870-5-101_SERVER',
                  'IEC60870-5-104_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-select
                        :items="itemsSizeOfIOA"
                        :input-value="active"
                        hide-details="auto"
                        v-model="editedConnection.sizeOfIOA"
                        label="Size of IOA"
                      ></v-select>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.sizeOfIoaTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.sizeOfIoaHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-104_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template>
                <v-switch
                  v-model="editedConnection.serverModeMultiActive"
                  inset
                  color="primary"
                  :label="`${$t('admin.protocolConnections.modeMultiActive')}${
                    editedConnection.serverModeMultiActive
                      ? $t('admin.protocolConnections.modeMultiActiveTrue')
                      : $t('admin.protocolConnections.modeMultiActiveFalse')
                  }`"
                ></v-switch>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-104_SERVER', 'IEC61850_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="
                          $t('admin.protocolConnections.maxClientConnections')
                        "
                        hide-details="auto"
                        v-model="editedConnection.maxClientConnections"
                      >
                      </v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t(
                          'admin.protocolConnections.maxClientConnectionsTitle'
                        )
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{
                        $t('admin.protocolConnections.maxClientConnectionsHint')
                      }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-104_SERVER',
                  'IEC60870-5-101_SERVER',
                  'IEC61850_SERVER',
                  'PI_DATA_ARCHIVE_INJECTOR',
                  'PI_DATA_ARCHIVE_CLIENT',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.maxQueueSize')"
                        hide-details="auto"
                        v-model="editedConnection.maxQueueSize"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.maxQueueSizeTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.maxQueueSizeHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['DNP3', 'IEC61850'].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.class0Scan')"
                        hide-details="auto"
                        v-model="editedConnection.class0ScanInterval"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.class0ScanTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.class0ScanHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="['DNP3'].includes(editedConnection.protocolDriver)"
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.class1Scan')"
                        hide-details="auto"
                        v-model="editedConnection.class1ScanInterval"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.class1ScanTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.class1ScanHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="['DNP3'].includes(editedConnection.protocolDriver)"
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.class2Scan')"
                        hide-details="auto"
                        v-model="editedConnection.class2ScanInterval"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.class2ScanTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.class2ScanHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="['DNP3'].includes(editedConnection.protocolDriver)"
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.class3Scan')"
                        hide-details="auto"
                        v-model="editedConnection.class3ScanInterval"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.class3ScanTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.class3ScanHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="['DNP3'].includes(editedConnection.protocolDriver)"
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.timeSyncMode')"
                        hide-details="auto"
                        v-model="editedConnection.timeSyncMode"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.timeSyncModeTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.timeSyncModeHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="['DNP3'].includes(editedConnection.protocolDriver)"
            >
              <v-switch
                v-model="editedConnection.enableUnsolicited"
                inset
                color="primary"
                :label="`${$t('admin.protocolConnections.enableUnsolicited')}${
                  editedConnection.enableUnsolicited
                    ? $t('admin.protocolConnections.enableUnsolicitedTrue')
                    : $t('admin.protocolConnections.enableUnsolicitedFalse')
                }`"
                class="mb-n6"
              ></v-switch>
            </v-list-item>

            <v-list-item
              v-if="['DNP3'].includes(editedConnection.protocolDriver)"
            >
              <v-row>
                <v-col>
                  <v-select
                    v-model="editedConnection.rangeScans"
                    :items="editedConnection.rangeScans"
                    :item-title="
                      (item) =>
                        `G:${item.group} V:${item.variation} A:${item.startAddress}-${item.stopAddress} P:${item.period}s`
                    "
                    :item-value="(item) => item"
                    chips
                    small-chips
                    closable-chips
                    :label="$t('admin.protocolConnections.rangeScans')"
                    multiple
                  ></v-select>
                </v-col>
                <v-col>
                  <v-btn
                    color="blue darken-1"
                    text
                    variant="tonal"
                    @click="dialogAddRangeScan = true"
                  >
                    <v-icon dark> mdi-plus </v-icon>
                    {{ $t('admin.protocolConnections.rangeScanAddNew') }}
                  </v-btn>
                </v-col>
              </v-row>

              <v-dialog
                v-model="dialogAddRangeScan"
                max-width="400"
                class="pa-8"
              >
                <v-card>
                  <v-card-title class="headline">
                    {{ $t('admin.protocolConnections.rangeScanAddNew') }}
                  </v-card-title>

                  <v-card-title class="headline">
                    <v-text-field
                      :label="$t('admin.protocolConnections.rangeScanGroup')"
                      autofocus
                      type="number"
                      min="1"
                      v-model="newRangeScan.group"
                    ></v-text-field>

                    <v-text-field
                      :label="
                        $t('admin.protocolConnections.rangeScanVariation')
                      "
                      type="number"
                      min="0"
                      v-model="newRangeScan.variation"
                    ></v-text-field>

                    <v-text-field
                      :label="$t('admin.protocolConnections.rangeScanStart')"
                      type="number"
                      min="0"
                      v-model="newRangeScan.startAddress"
                    ></v-text-field>

                    <v-text-field
                      :label="$t('admin.protocolConnections.rangeScanStop')"
                      type="number"
                      min="0"
                      v-model="newRangeScan.stopAddress"
                    ></v-text-field>

                    <v-text-field
                      :label="$t('admin.protocolConnections.rangeScanPeriod')"
                      type="number"
                      min="1"
                      v-model="newRangeScan.period"
                    ></v-text-field>
                  </v-card-title>

                  <v-card-actions>
                    <v-spacer></v-spacer>
                    <v-btn
                      color="orange darken-1"
                      text
                      variant="tonal"
                      @click="dialogAddRangeScan = false"
                    >
                      {{ $t('common.cancel') }}
                    </v-btn>
                    <v-btn
                      color="blue darken-1"
                      text
                      variant="tonal"
                      @click="addNewRangeScan"
                    >
                      {{ $t('common.ok') }}
                    </v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.timeoutAck')"
                        hide-details="auto"
                        v-model="editedConnection.timeoutForACK"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.timeoutAckTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.timeoutAckHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="1"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.timeoutRepeat')"
                        hide-details="auto"
                        v-model="editedConnection.timeoutRepeat"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.timeoutRepeatTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.timeoutRepeatHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-select
                        :items="sizeOfLinkAddressItems"
                        :input-value="active"
                        hide-details="auto"
                        v-model="editedConnection.sizeOfLinkAddress"
                        :label="
                          $t('admin.protocolConnections.sizeOfLinkAddress')
                        "
                      ></v-select>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t('admin.protocolConnections.sizeOfLinkAddressTitle')
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{
                        $t('admin.protocolConnections.sizeOfLinkAddressHint')
                      }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <v-switch
                v-model="editedConnection.useSingleCharACK"
                inset
                color="primary"
                :label="`${$t('admin.protocolConnections.singleCharAck')}${
                  editedConnection.useSingleCharACK
                    ? $t('admin.protocolConnections.singleCharAckTrue')
                    : $t('admin.protocolConnections.singleCharAckFalse')
                }`"
              ></v-switch>
            </v-list-item>

            <v-list-item
              class="ma-0"
              v-if="
                [
                  'OPC-UA',
                  'MQTT-SPARKPLUG-B',
                  'OPC-UA_SERVER',
                  'IEC61850',
                  'OPC-DA',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-switch
                class="ma-0"
                v-model="editedConnection.useSecurity"
                inset
                color="primary"
                :label="`${$t('admin.protocolConnections.useSecurity')}${
                  editedConnection.useSecurity
                    ? $t('admin.protocolConnections.useSecurityTrue')
                    : $t('admin.protocolConnections.useSecurityFalse')
                }`"
              ></v-switch>
            </v-list-item>

            <v-list-item
              class="ma-0"
              v-if="
                ['DNP3', 'DNP3_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <v-select
                :items="connectionModeDnp3Items"
                :label="$t('admin.protocolConnections.connectionMode')"
                v-model="editedConnection.connectionMode"
                outlined
              ></v-select>
            </v-list-item>
          </v-list>
        </v-card>

        <v-card
          class="mt-6"
          tile
          variant="outlined"
          v-if="
            [
              'IEC60870-5-104',
              'IEC60870-5-104_SERVER',
              'IEC61850',
              'IEC61850_SERVER',
              'I104M',
              'MODBUS',
              'PLCTAG',
              'TELEGRAF-LISTENER',
              'OPC-UA_SERVER',
              'ICCP',
              'ICCP_SERVER',
            ].includes(editedConnection.protocolDriver) ||
            (['DNP3', 'DNP3_SERVER'].includes(
              editedConnection.protocolDriver
            ) &&
              editedConnection.connectionMode !== 'Serial')
          "
        >
          <v-card-title>
            <span class="text-h5">
              {{ $t('admin.protocolConnections.tcpParameters') }}
            </span>
          </v-card-title>
          <v-list flat dense shaped>
            <v-list-item
              v-if="
                [
                  'IEC60870-5-104_SERVER',
                  'IEC61850_SERVER',
                  'I104M',
                  'TELEGRAF-LISTENER',
                  'OPC-UA_SERVER',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver) ||
                (['DNP3', 'DNP3_SERVER'].includes(
                  editedConnection.protocolDriver
                ) &&
                  (editedConnection.connectionMode.endsWith('Passive') ||
                    editedConnection.connectionMode === 'UDP'))
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :rules="[rules.required, rules.ipPort]"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.bindIpPort')"
                        hide-details="auto"
                        v-model="editedConnection.ipAddressLocalBind"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.bindIpPortTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.bindIpPortHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-104_SERVER',
                  'IEC60870-5-104',
                  'IEC61850',
                  'IEC61850_SERVER',
                  'DNP3_SERVER',
                  'I104M',
                  'PLCTAG',
                  'MODBUS',
                  'TELEGRAF-LISTENER',
                  'OPC-UA_SERVER',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver) ||
                (['DNP3'].includes(editedConnection.protocolDriver) &&
                  (editedConnection.connectionMode.endsWith('Active') ||
                    editedConnection.connectionMode === 'UDP'))
              "
            >
              <v-row>
                <v-col>
                  <v-select
                    v-model="editedConnection.ipAddresses"
                    :items="editedConnection.ipAddresses"
                    chips
                    small-chips
                    closable-chips
                    :label="$t('admin.protocolConnections.remoteIpAddresses')"
                    multiple
                  ></v-select>
                </v-col>
                <v-col>
                  <v-btn
                    color="blue darken-1"
                    text
                    variant="tonal"
                    @click="dialogAddIP = true"
                  >
                    <v-icon dark> mdi-plus </v-icon>
                    {{ $t('admin.protocolConnections.remoteIpAddressAdd') }}
                  </v-btn>
                </v-col>
              </v-row>

              <v-dialog v-model="dialogAddIP" max-width="400" class="pa-8">
                <v-card>
                  <v-card-title class="headline">
                    {{ $t('admin.protocolConnections.remoteIpAddressAdd') }}
                  </v-card-title>

                  <v-card-title class="headline">
                    <v-text-field
                      autofocus
                      label="New IP"
                      v-model="newIP"
                      :rules="[rules.required, rules.ipPort]"
                    ></v-text-field>
                  </v-card-title>

                  <v-card-actions>
                    <v-spacer></v-spacer>
                    <v-btn
                      color="orange darken-1"
                      text
                      variant="tonal"
                      @click="dialogAddIP = false"
                    >
                      {{ $t('common.cancel') }}
                    </v-btn>

                    <v-btn
                      color="blue darken-1"
                      text
                      variant="tonal"
                      @click="addNewIP"
                    >
                      {{ $t('common.ok') }}
                    </v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>
            </v-list-item>
          </v-list>
        </v-card>

        <v-card
          class="mt-6"
          tile
          variant="outlined"
          v-if="
            ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
              editedConnection.protocolDriver
            ) ||
            ([
              'MQTT-SPARKPLUG-B',
              'OPC-UA_SERVER',
              'IEC61850',
              'IEC61850_SERVER',
              'OPC-DA',
              'ICCP',
              'ICCP_SERVER',
            ].includes(editedConnection.protocolDriver) &&
              editedConnection.useSecurity) ||
            (['DNP3', 'DNP3_SERVER'].includes(
              editedConnection.protocolDriver
            ) &&
              editedConnection.connectionMode.startsWith('TLS'))
          "
        >
          <v-card-title>
            <span class="text-h5">
              {{ $t('admin.protocolConnections.tlsParameters') }}
            </span>
          </v-card-title>

          <v-list flat dense shaped>
            <v-list-item
              v-if="
                [
                  'IEC60870-5-104',
                  'IEC60870-5-104_SERVER',
                  'IEC61850',
                  'IEC61850_SERVER',
                  'DNP3',
                  'DNP3_SERVER',
                  'MQTT-SPARKPLUG-B',
                  'OPC-UA_SERVER',
                  'OPC-DA',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="
                          $t(
                            'admin.protocolConnections.localCertificateFilePath'
                          )
                        "
                        hide-details="auto"
                        v-model="editedConnection.localCertFilePath"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t(
                          'admin.protocolConnections.localCertificateFilePathTitle'
                        )
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{
                        $t(
                          'admin.protocolConnections.localCertificateFilePathHint'
                        )
                      }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['MQTT-SPARKPLUG-B'].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.pfxFilePath')"
                        hide-details="auto"
                        v-model="editedConnection.pfxFilePath"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.pfxFilePathTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.pfxFilePathHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'MQTT-SPARKPLUG-B',
                  'IEC60870-5-104_SERVER',
                  'IEC60870-5-104',
                  'IEC61850',
                  'IEC61850_SERVER',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.passphrase')"
                        hide-details="auto"
                        v-model="editedConnection.passphrase"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.passphraseTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.passphraseHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'DNP3',
                  'MQTT-SPARKPLUG-B',
                  'OPC-UA_SERVER',
                  'IEC61850',
                  'IEC61850_SERVER',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="
                          $t('admin.protocolConnections.privateCertificateFile')
                        "
                        hide-details="auto"
                        v-model="editedConnection.privateKeyFilePath"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t(
                          'admin.protocolConnections.privateCertificateFileTitle'
                        )
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{
                        $t(
                          'admin.protocolConnections.privateCertificateFileHint'
                        )
                      }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'DNP3', 
                  'DNP3_SERVER', 
                  'OPC-DA', 
                  'OPC-DA_SERVER'
                ].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="
                          $t(
                            'admin.protocolConnections.peerCertificateFilePath'
                          )
                        "
                        hide-details="auto"
                        v-model="editedConnection.peerCertFilePath"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t(
                          'admin.protocolConnections.peerCertificateFilePathTitle'
                        )
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{
                        $t(
                          'admin.protocolConnections.peerCertificateFilePathHint'
                        )
                      }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-104_SERVER',
                  'IEC60870-5-104',
                  'IEC61850',
                  'IEC61850_SERVER',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-row>
                <v-col>
                  <v-select
                    v-model="editedConnection.peerCertFilesPaths"
                    :items="editedConnection.peerCertFilesPaths"
                    chips
                    small-chips
                    closable-chips
                    :label="
                      $t('admin.protocolConnections.peerCertificateFilesPaths')
                    "
                    multiple
                  ></v-select>
                </v-col>
                <v-col>
                  <v-btn
                    color="blue darken-1"
                    text
                    variant="tonal"
                    @click="dialogAddCertPath = true"
                  >
                    <v-icon dark> mdi-plus </v-icon>
                    {{
                      $t(
                        'admin.protocolConnections.peerCertificateFilesPathsAdd'
                      )
                    }}
                  </v-btn>
                </v-col>
              </v-row>
              <v-dialog
                v-model="dialogAddCertPath"
                max-width="400"
                class="pa-8"
              >
                <v-card>
                  <v-card-title class="headline">
                    {{
                      $t(
                        'admin.protocolConnections.peerCertificateFilesPathsAdd'
                      )
                    }}
                  </v-card-title>

                  <v-card-title class="headline">
                    <v-text-field
                      label="New Path"
                      v-model="newCertPath"
                    ></v-text-field>
                  </v-card-title>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn
                      color="orange darken-1"
                      text
                      variant="tonal"
                      @click="dialogAddCertPath = false"
                    >
                      {{ $t('common.cancel') }}
                    </v-btn>

                    <v-btn
                      color="blue darken-1"
                      text
                      variant="tonal"
                      @click="addNewCertPath"
                    >
                      {{ $t('common.ok') }}
                    </v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-104',
                  'IEC60870-5-104_SERVER',
                  'MQTT-SPARKPLUG-B',
                  'IEC61850',
                  'IEC61850_SERVER',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="
                          $t(
                            'admin.protocolConnections.rootCertificateFilePath'
                          )
                        "
                        hide-details="auto"
                        v-model="editedConnection.rootCertFilePath"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t(
                          'admin.protocolConnections.rootCertificateFilePathTitle'
                        )
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{
                        $t(
                          'admin.protocolConnections.rootCertificateFilePathHint'
                        )
                      }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'DNP3', 
                  'DNP3_SERVER',
                  'MQTT-SPARKPLUG-B'
                ].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="
                          $t('admin.protocolConnections.opensslCypherList')
                        "
                        hide-details="auto"
                        v-model="editedConnection.cipherList"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t('admin.protocolConnections.opensslCypherListTitle')
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{
                        $t('admin.protocolConnections.opensslCypherListHint')
                      }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'DNP3',
                  'DNP3_SERVER',
                  'MQTT-SPARKPLUG-B',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-switch
                v-model="editedConnection.allowTLSv10"
                inset
                color="primary"
                :label="`${$t('admin.protocolConnections.allowTls10')}${
                  editedConnection.allowTLSv10
                    ? $t('admin.protocolConnections.allowTls10True')
                    : $t('admin.protocolConnections.allowTls10False')
                }`"
                class="mb-n6"
              ></v-switch>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'DNP3',
                  'DNP3_SERVER',
                  'MQTT-SPARKPLUG-B',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-switch
                v-model="editedConnection.allowTLSv11"
                inset
                color="primary"
                :label="`${$t('admin.protocolConnections.allowTls11')}${
                  editedConnection.allowTLSv11
                    ? $t('admin.protocolConnections.allowTls11True')
                    : $t('admin.protocolConnections.allowTls11False')
                }`"
                class="mb-n6"
              ></v-switch>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'DNP3',
                  'DNP3_SERVER',
                  'MQTT-SPARKPLUG-B',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-switch
                v-model="editedConnection.allowTLSv12"
                inset
                color="primary"
                :label="`${$t('admin.protocolConnections.allowTls12')}${
                  editedConnection.allowTLSv12
                    ? $t('admin.protocolConnections.allowTls12True')
                    : $t('admin.protocolConnections.allowTls12False')
                }`"
                class="mb-n6"
              ></v-switch>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'DNP3',
                  'DNP3_SERVER',
                  'MQTT-SPARKPLUG-B',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-switch
                v-model="editedConnection.allowTLSv13"
                inset
                color="primary"
                :label="`${$t('admin.protocolConnections.allowTls13')}${
                  editedConnection.allowTLSv13
                    ? $t('admin.protocolConnections.allowTls13True')
                    : $t('admin.protocolConnections.allowTls13False')
                }`"
                class="mb-n6"
              ></v-switch>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-104',
                  'IEC60870-5-104_SERVER',
                  'MQTT-SPARKPLUG-B',
                  'IEC61850',
                  'IEC61850_SERVER',
                  'DNP3',
                  'DNP3_SERVER',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-switch
                v-model="editedConnection.allowOnlySpecificCertificates"
                inset
                color="primary"
                :label="`${$t('admin.protocolConnections.allowSpecificCerts')}${
                  editedConnection.allowOnlySpecificCertificates
                    ? $t('admin.protocolConnections.allowSpecificCertsTrue')
                    : $t('admin.protocolConnections.allowSpecificCertsFalse')
                }`"
                class="mb-n6"
              ></v-switch>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-104',
                  'IEC60870-5-104_SERVER',
                  'MQTT-SPARKPLUG-B',
                  'IEC61850',
                  'IEC61850_SERVER',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-switch
                v-model="editedConnection.chainValidation"
                inset
                color="primary"
                :label="`${$t(
                  'admin.protocolConnections.certChainValidation'
                )}${
                  editedConnection.chainValidation
                    ? $t('common.true')
                    : $t('common.false')
                }`"
                class="mb-n6"
              ></v-switch>
            </v-list-item>
          </v-list>
        </v-card>

        <v-card
          class="mt-6"
          tile
          variant="outlined"
          v-if="
            [
              'MQTT-SPARKPLUG-B',
              'OPC-UA_SERVER',
              'IEC61850',
              'IEC61850_SERVER',
              'PI_DATA_ARCHIVE_INJECTOR',
              'PI_DATA_ARCHIVE_CLIENT',
              'PLC4X',
              'OPC-DA',
              'OPC-DA_SERVER',
              'ICCP',
              'ICCP_SERVER',
            ].includes(editedConnection.protocolDriver)
          "
        >
          <v-card-title>
            <span class="text-h5">
              {{ $t('admin.protocolConnections.pubSubCard') }}
            </span>
          </v-card-title>

          <v-list flat dense shaped>
            <v-list-item
              v-if="
                [
                  'ICCP', 
                  'ICCP_SERVER'
                ].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.domain')"
                        hide-details="auto"
                        v-model="editedConnection.domain"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.domainTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.domainHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'MQTT-SPARKPLUG-B',
                  'OPC-UA',
                  'OPC-UA_SERVER',
                  'IEC61850',
                  'IEC61850_SERVER',
                  'PI_DATA_ARCHIVE_INJECTOR',
                  'PI_DATA_ARCHIVE_CLIENT',
                  'PLC4X',
                  'OPC-DA',
                  'OPC-DA_SERVER',
                  'ICCP',
                  'ICCP_SERVER',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <v-row>
                <v-col>
                  <v-select
                    v-model="editedConnection.topics"
                    :items="editedConnection.topics"
                    chips
                    small-chips
                    closable-chips
                    :label="$t('admin.protocolConnections.topics')"
                    multiple
                  ></v-select>
                </v-col>
                <v-col>
                  <v-btn
                    color="blue darken-1"
                    text
                    variant="tonal"
                    @click="dialogAddTopic = true"
                  >
                    <v-icon dark> mdi-plus </v-icon>
                    {{ $t('admin.protocolConnections.topicsAddNew') }}
                  </v-btn>
                </v-col>
              </v-row>

              <v-dialog v-model="dialogAddTopic" max-width="450" class="pa-8">
                <v-card>
                  <v-card-title class="headline">
                    {{ $t('admin.protocolConnections.topicsAddNew') }}
                  </v-card-title>

                  <v-card-title class="headline">
                    <v-text-field
                      autofocus
                      :label="$t('admin.protocolConnections.topicsNew')"
                      v-model="newTopic"
                      :rules="[rules.required, rules.topic]"
                    ></v-text-field>
                  </v-card-title>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn
                      color="orange darken-1"
                      text
                      variant="tonal"
                      @click="dialogAddTopic = false"
                    >
                      {{ $t('common.cancel') }}
                    </v-btn>

                    <v-btn
                      color="blue darken-1"
                      text
                      variant="tonal"
                      @click="addNewTopic"
                    >
                      {{ $t('common.ok') }}
                    </v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>
            </v-list-item>

            <v-list-item
              v-if="
                ['ICCP', 'ICCP_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        :rules="[rules.required]"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.aeQualifier')"
                        hide-details="auto"
                        v-model="editedConnection.aeQualifier"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.aeQualifierTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.aeQualifierHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['ICCP', 'ICCP_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.localAppTitle')"
                        hide-details="auto"
                        v-model="editedConnection.localAppTitle"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.localAppTitleTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.localAppTitleHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['ICCP', 'ICCP_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :rules="[rules.required, rules.isoSelectors]"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.localSelectors')"
                        hide-details="auto"
                        v-model="editedConnection.localSelectors"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.localSelectorsTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.localSelectorsHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['ICCP', 'ICCP_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.remoteAppTitle')"
                        hide-details="auto"
                        v-model="editedConnection.remoteAppTitle"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.remoteAppTitleTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.remoteAppTitleHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="['ICCP'].includes(editedConnection.protocolDriver)"
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :rules="[rules.required, rules.isoSelectors]"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.remoteSelectors')"
                        hide-details="auto"
                        v-model="editedConnection.remoteSelectors"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.remoteSelectorsTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.remoteSelectorsHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['MQTT-SPARKPLUG-B'].includes(editedConnection.protocolDriver)
              "
            >
              <v-row>
                <v-col>
                  <v-select
                    v-model="editedConnection.topicsAsFiles"
                    :items="editedConnection.topicsAsFiles"
                    chips
                    small-chips
                    closable-chips
                    :label="$t('admin.protocolConnections.topicsAsFiles')"
                    multiple
                  ></v-select>
                </v-col>
                <v-col>
                  <v-btn
                    color="blue"
                    text
                    variant="tonal"
                    @click="dialogAddTopicAsFile = true"
                  >
                    <v-icon dark> mdi-plus </v-icon>
                    {{ $t('admin.protocolConnections.topicsAddNew') }}
                  </v-btn>
                </v-col>
              </v-row>

              <v-dialog
                v-model="dialogAddTopicAsFile"
                max-width="450"
                class="pa-8"
              >
                <v-card>
                  <v-card-title class="headline">
                    {{ $t('admin.protocolConnections.topicsAsFilesAddNew') }}
                  </v-card-title>

                  <v-card-title class="headline">
                    <v-text-field
                      autofocus
                      :label="$t('admin.protocolConnections.topicsNew')"
                      v-model="newTopicAsFile"
                      :rules="[rules.required, rules.topic]"
                    ></v-text-field>
                  </v-card-title>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn
                      color="orange darken-1"
                      text
                      variant="tonal"
                      @click="dialogAddTopicAsFile = false"
                    >
                      {{ $t('common.cancel') }}
                    </v-btn>

                    <v-btn
                      color="blue darken-1"
                      text
                      @click="addNewTopicAsFile()"
                    >
                      {{ $t('common.ok') }}
                    </v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>
            </v-list-item>

            <v-list-item
              v-if="
                ['MQTT-SPARKPLUG-B'].includes(editedConnection.protocolDriver)
              "
              :label="$t('admin.protocolConnections.topicsScripted')"
            >
              <v-row fill-height>
                <v-col fill-height>
                  <v-card dense tile>
                    <v-card-text> Scripted Topics </v-card-text>
                    <template
                      v-for="(item, index) in editedConnection.topicsScripted"
                    >
                      <v-container :key="item.dummy" v-if="true" fluid>
                        <v-card dense>
                          <v-card-text dense>
                            <v-text-field
                              style="font-size: 1em"
                              dense
                              :label="$t('admin.protocolConnections.topic')"
                              v-model="item.topic"
                              :rules="[rules.required, rules.topic]"
                            ></v-text-field>

                            <v-textarea
                              row-height="20"
                              auto-grow
                              style="font-size: 0.9em; font-family: monospace"
                              class="ma-0"
                              outlined
                              rows="4"
                              dense
                              :label="
                                $t('admin.protocolConnections.topicScript')
                              "
                              v-model="item.script"
                              :rules="[rules.required]"
                            ></v-textarea>

                            <v-btn
                              class="ma-0"
                              dark
                              x-small
                              color="red"
                              text
                              variant="tonal"
                              @click="deleteTopicScripted(index)"
                            >
                              <v-icon dark> mdi-minus </v-icon>
                              {{
                                $t('admin.protocolConnections.topicDelete') +
                                ' - ' +
                                item.topic
                              }}
                            </v-btn>
                          </v-card-text>
                        </v-card>
                      </v-container>
                    </template>
                    <v-card-text>
                      <v-btn
                        class="ma-0"
                        dark
                        x-small
                        color="blue"
                        text
                        variant="tonal"
                        @click="addNewTopicScripted"
                      >
                        <v-icon dark> mdi-plus </v-icon>
                        {{ $t('admin.protocolConnections.topicsScriptedNew') }}
                      </v-btn>
                    </v-card-text>
                  </v-card>
                </v-col>
              </v-row>
            </v-list-item>

            <v-list-item
              class="mt-4"
              v-if="
                [
                  'MQTT-SPARKPLUG-B',
                  'PI_DATA_ARCHIVE_INJECTOR',
                  'PI_DATA_ARCHIVE_CLIENT',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.clientId')"
                        hide-details="auto"
                        v-model="editedConnection.clientId"
                        :rules="[rules.subtopic]"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.clientIdTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.clientIdHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['MQTT-SPARKPLUG-B', 'OPC-UA_SERVER'].includes(
                  editedConnection.protocolDriver
                )
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.groupId')"
                        hide-details="auto"
                        v-model="editedConnection.groupId"
                        :rules="[rules.subtopic]"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.groupIdTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.groupIdHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['MQTT-SPARKPLUG-B'].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.edgeNodeId')"
                        hide-details="auto"
                        v-model="editedConnection.edgeNodeId"
                        :rules="[rules.subtopic]"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.edgeNodeIdTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.edgeNodeIdHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['MQTT-SPARKPLUG-B'].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.deviceId')"
                        hide-details="auto"
                        v-model="editedConnection.deviceId"
                        :rules="[rules.subtopic]"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.deviceIdTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.deviceIdHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'MQTT-SPARKPLUG-B',
                  'PI_DATA_ARCHIVE_INJECTOR',
                  'PI_DATA_ARCHIVE_CLIENT',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.scadaHostId')"
                        hide-details="auto"
                        v-model="editedConnection.scadaHostId"
                        :rules="[rules.subtopic]"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.scadaHostIdTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.scadaHostIdHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                ['MQTT-SPARKPLUG-B'].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="
                          $t('admin.protocolConnections.publishTopicRoot')
                        "
                        hide-details="auto"
                        v-model="editedConnection.publishTopicRoot"
                        :rules="[rules.subtopic]"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{
                        $t('admin.protocolConnections.publishTopicRootTitle')
                      }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.publishTopicRootHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>
          </v-list>
        </v-card>

        <v-card
          class="mt-6"
          tile
          variant="outlined"
          v-if="
            ['IEC60870-5-101', 'IEC60870-5-101_SERVER', 'MODBUS'].includes(
              editedConnection.protocolDriver
            ) ||
            (['DNP3', 'DNP3_SERVER'].includes(
              editedConnection.protocolDriver
            ) &&
              editedConnection.connectionMode === 'Serial')
          "
        >
          <v-card-title>
            <span class="text-h5">
              {{ $t('admin.protocolConnections.serialParameters') }}
            </span>
          </v-card-title>

          <v-list flat dense shaped>
            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-101_SERVER',
                  'DNP3',
                  'DNP3_SERVER',
                  'MODBUS',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="text"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.commPortName')"
                        hide-details="auto"
                        v-model="editedConnection.portName"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.commPortNameTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.commPortNameHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-101_SERVER',
                  'DNP3',
                  'DNP3_SERVER',
                  'MODBUS',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="150"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.baudRate')"
                        hide-details="auto"
                        v-model="editedConnection.baudRate"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.baudRateTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.baudRateHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-101_SERVER',
                  'DNP3',
                  'DNP3_SERVER',
                  'MODBUS',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-select
                        :items="parityItems"
                        :input-value="active"
                        hide-details="auto"
                        v-model="editedConnection.parity"
                        :label="$t('admin.protocolConnections.parity')"
                      ></v-select>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.parityTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.parityHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-101_SERVER',
                  'DNP3',
                  'DNP3_SERVER',
                  'MODBUS',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-select
                        :items="stopBitsItems"
                        :input-value="active"
                        hide-details="auto"
                        v-model="editedConnection.stopBits"
                        :label="$t('admin.protocolConnections.stopBits')"
                      ></v-select>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.stopBitsTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.stopBitsHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-101_SERVER',
                  'DNP3',
                  'DNP3_SERVER',
                  'MODBUS',
                ].includes(editedConnection.protocolDriver)
              "
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-select
                        :items="handshakeItems"
                        :input-value="active"
                        hide-details="auto"
                        v-model="editedConnection.handshake"
                        :label="$t('admin.protocolConnections.handshake')"
                      ></v-select>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.handshakeTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.handshakeHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>

            <v-list-item
              v-if="['DNP3'].includes(editedConnection.protocolDriver)"
            >
              <template v-slot:default="{ active }">
                <v-row>
                  <v-col>
                    <v-list-item-action>
                      <v-text-field
                        type="number"
                        min="0"
                        :input-value="active"
                        :label="$t('admin.protocolConnections.asyncOpenDelay')"
                        hide-details="auto"
                        v-model="editedConnection.asyncOpenDelay"
                      ></v-text-field>
                    </v-list-item-action>
                  </v-col>
                  <v-col>
                    <v-list-item-title>
                      {{ $t('admin.protocolConnections.asyncOpenDelayTitle') }}
                    </v-list-item-title>
                    <v-list-item-subtitle>
                      {{ $t('admin.protocolConnections.asyncOpenDelayHint') }}
                    </v-list-item-subtitle>
                  </v-col>
                </v-row>
              </template>
            </v-list-item>
          </v-list>
        </v-card>
      </v-card-text>

      <v-card-actions>
        <v-chip v-if="error" color="red darken-1">{{
          $t('common.error')
        }}</v-chip>
        <v-spacer></v-spacer>
        <v-btn
          color="orange darken-1"
          variant="tonal"
          text
          @click="dialogEditConnection = false"
        >
          {{ $t('common.cancel') }}</v-btn
        >
        <v-btn
          color="blue darken-1"
          variant="tonal"
          text
          @click="updateOrCreateProtocolConnection"
        >
          {{ $t('common.save') }}</v-btn
        >
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="dialogDelConn" max-width="400">
    <v-card>
      <v-card-title class="text-h5">
        {{ $t('admin.protocolConnections.deleteProtocolConnection') }}
      </v-card-title>

      <v-card-text>
        {{ $t('admin.protocolConnections.deleteProtocolConnectionConfirm') }}
      </v-card-text>

      <v-card-actions>
        <v-switch
          v-model="deleteTags"
          inset
          color="red"
          :label="`${
            $t('admin.protocolConnections.deleteConnectionEraseTags') + ' '
          }${deleteTags ? $t('common.true') : $t('common.false')}`"
          class="mb-0"
        ></v-switch>

        <v-spacer></v-spacer>

        <v-btn
          color="orange darken-1"
          text
          variant="tonal"
          @click="dialogDelConn = false"
        >
          {{ $t('common.cancel') }}
        </v-btn>

        <v-btn
          color="red darken-1"
          text
          variant="tonal"
          @click="deleteProtocolConnection()"
        >
          {{ $t('common.delete') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
  import { ref, computed, onMounted, onUnmounted } from 'vue'
  import { useI18n } from 'vue-i18n'

  const { t } = useI18n()
  const headers = computed(() => [
    { title: '#', key: 'id', align: 'end' },
    { title: t('admin.protocolConnections.headers.name'), key: 'name' },
    {
      title: t('admin.protocolConnections.headers.protocolConnectionNumber'),
      align: 'end',
      key: 'protocolConnectionNumber',
    },
    {
      title: t(
        'admin.protocolConnections.headers.protocolDriverAndInstanceNumber'
      ),
      align: 'start',
      key: 'protocolDriver',
    },
    { title: t('admin.protocolConnections.headers.enabled'), key: 'enabled' },
    { title: t('admin.protocolConnections.headers.stats'), key: 'stats' },
    {
      title: t('admin.protocolConnections.headers.actions'),
      key: 'actions',
      sortable: false,
    },
  ])

  const dialogEditConnection = ref(false)
  const editedConnection = ref({})
  const error = ref(false)
  const itemsSizeOfCOT = ref([1, 2])
  const itemsSizeOfCA = ref([1, 2])
  const itemsSizeOfIOA = ref([1, 2, 3])
  const dialogAddIP = ref(false)
  const dialogAddCertPath = ref(false)
  const dialogAddURL = ref(false)
  const dialogAddTopic = ref(false)
  const dialogAddTopicAsFile = ref(false)
  const dialogAddRangeScan = ref(false)
  const dialogDelConn = ref(false)
  const deleteTags = ref(false)
  const newRangeScan = ref({
    group: 1,
    variation: 0,
    startAddress: 0,
    stopAddress: 0,
    period: 300,
  })
  const newIP = ref('')
  const newCertPath = ref('')
  const newURL = ref('')
  const newTopic = ref('')
  const newTopicAsFile = ref('')
  const protocolConnections = ref([])
  const driverInstancesByType = ref({})

  const rules = {
    required: (value) =>
      !!value || t('admin.protocolConnections.rulesRequired'),
    isoSelectors: (value) => {
      const pattern = /^\d+(?:\s+\d+){7}$/
      return (
        pattern.test(value) ||
        t('admin.protocolConnections.rulesInvalidIsoSelector')
      )
    },
    ip: (value) => {
      const pattern = /\b((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4}\b/
      return (
        pattern.test(value) || t('admin.protocolConnections.rulesInvalidIP')
      )
    },
    ipPort: (value) => {
      const pattern =
        /\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)(?::\d{0,4})?\b/
      return (
        pattern.test(value) || t('admin.protocolConnections.rulesInvalidIpPort')
      )
    },
    endpointOPC: (value) => {
      let pattern =
        /^opc\.tcp:\/\/[a-zA-Z0-9-_]+[:./\\]+([a-zA-Z0-9 -_./:=&"'?%+@#$!])+$/
      return (
        pattern.test(value) ||
        t('admin.protocolConnections.rulesInvalidEndpoint')
      )
    },
    endpointOPCDA: (value) => {
      let pattern =
        /^opcda:\/\/[a-zA-Z0-9-_]+[:./\\]+([a-zA-Z0-9 -_./:=&"'?%+@#$!])+$/
      return (
        pattern.test(value) ||
        t('admin.protocolConnections.rulesInvalidEndpoint')
      )
    },
    endpointMQTT: (value) => {
      let pattern =
        /^mqtt:\/\/[a-zA-Z0-9-_]+[:./\\]+([a-zA-Z0-9 -_./:=&"'?%+@#$!])+$/
      return (
        pattern.test(value) ||
        t('admin.protocolConnections.rulesInvalidEndpoint')
      )
    },
    subtopic: (value) => {
      return (
        !(
          editedConnection.value.protocolDriver === 'MQTT-SPARKPLUG-B' &&
          (value.includes('#') || value.includes('/') || value.includes('+'))
        ) || t('admin.protocolConnections.rulesInvalidTopic')
      )
    },
    topic: () => {
      return true || t('admin.protocolConnections.rulesInvalidTopic')
    },
    topicScripted: (value) => {
      return (
        rules.topic(value.topic) ||
        t('admin.protocolConnections.rulesInvalidTopic')
      )
    },
  }

  const connectionModeDnp3Items = [
    'TCP Active',
    'TCP Passive',
    'TLS Active',
    'TLS Passive',
    'UDP',
    'Serial',
  ]

  const driverNameItems = [
    'IEC60870-5-104',
    'IEC60870-5-104_SERVER',
    'IEC60870-5-101',
    'IEC60870-5-101_SERVER',
    'IEC61850',
    'IEC61850_SERVER',
    'DNP3',
    'MQTT-SPARKPLUG-B',
    'OPC-UA',
    'OPC-UA_SERVER',
    'OPC-DA',
    'OPC-DA_SERVER',
    'PLCTAG',
    'PLC4X',
    'TELEGRAF-LISTENER',
    'I104M',
    'ICCP',
    'ICCP_SERVER',
    'PI_DATA_ARCHIVE_INJECTOR',
    'PI_DATA_ARCHIVE_CLIENT',
  ]

  const parityItems = ['None', 'Even', 'Odd', 'Mark', 'Space']
  const stopBitsItems = ['One', 'One5', 'Two']
  const handshakeItems = ['None', 'Rts', 'Xon', 'RtsXon']
  const sizeOfLinkAddressItems = [0, 1, 2]

  // Lifecycle hooks
  onMounted(async () => {
    await fetchProtocolConnections()
    document.documentElement.style.overflowY = 'scroll'
  })

  onUnmounted(async () => {
    document.documentElement.style.overflowY = 'auto'
  })

  const updateProtocolConnection = async () => {
    var connDup = Object.assign({}, editedConnection.value)
    delete connDup['id']

    try {
      const response = await fetch('/Invoke/auth/updateProtocolConnection', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(connDup),
      })
      const json = await response.json()
      if (json.error) {
        console.warn(json)
        error.value = true
        return
      }
      dialogEditConnection.value = false
    } catch (err) {
      console.warn(err)
    }
    fetchProtocolConnections()
  }

  const createProtocolConnection = async () => {
    try {
      const response = await fetch('/Invoke/auth/createProtocolConnection', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({}),
      })
      const json = await response.json()
      if (json.error || !json._id) {
        console.warn(json)
        error.value = true
        return
      }
      editedConnection.value._id = json._id
      await updateProtocolConnection()
      dialogEditConnection.value = false
    } catch (err) {
      console.warn(err)
    }
    fetchProtocolConnections()
  }

  const addNewRangeScan = async () => {
    dialogAddRangeScan.value = false
    editedConnection.value.rangeScans.push(
      Object.assign({}, newRangeScan.value)
    )
  }

  const addNewIP = async () => {
    dialogAddIP.value = false
    if (rules.ipPort(newIP.value) !== true) return
    if (
      newIP.value != '' &&
      !editedConnection.value.ipAddresses.includes(newIP.value)
    ) {
      editedConnection.value.ipAddresses.push(newIP.value)
    }
    newIP.value = ''
  }

  const addNewCertPath = async () => {
    dialogAddCertPath.value = false
    if (
      newCertPath.value != '' &&
      !editedConnection.value.peerCertFilesPaths.includes(newCertPath.value)
    ) {
      editedConnection.value.peerCertFilesPaths.push(newCertPath.value)
      newCertPath.value = ''
    }
  }

  const addNewURL = async () => {
    dialogAddURL.value = false
    if (
      editedConnection.value.protocolDriver === 'OPC-UA' &&
      rules.endpointOPC(newURL.value) !== true
    )
      return
    if (
      editedConnection.value.protocolDriver === 'MQTT-SPARKPLUG-B' &&
      rules.endpointMQTT(newURL.value) !== true
    )
      return
    if (
      newURL.value != '' &&
      !editedConnection.value.endpointURLs.includes(newURL.value)
    ) {
      editedConnection.value.endpointURLs.push(newURL.value)
      newURL.value = ''
    }
  }

  const addNewTopic = async () => {
    dialogAddTopic.value = false
    if (rules.topic(newTopic.value) !== true) return
    if (
      newTopic.value != '' &&
      !editedConnection.value.topics.includes(newTopic.value)
    ) {
      editedConnection.value.topics.push(newTopic.value)
      newTopic.value = ''
    }
  }

  const addNewTopicAsFile = async () => {
    dialogAddTopicAsFile.value = false
    if (rules.topic(newTopicAsFile.value) !== true) return
    if (
      newTopicAsFile.value != '' &&
      !editedConnection.value.topicsAsFiles.includes(newTopicAsFile.value)
    ) {
      editedConnection.value.topicsAsFiles.push(newTopicAsFile.value)
      newTopicAsFile.value = ''
    }
  }

  const deleteTopicScripted = async (index) => {
    editedConnection.value.topicsScripted.splice(index, 1)
  }

  const addNewTopicScripted = async () => {
    editedConnection.value.topicsScripted.push({
      topic: '',
      script: `// extract values from array of values [ 1.22, 2.34, 3.45 ]
  shared.dataArray = [] // here put the array of returned objects
  // shared.payload contains the message payload as a buffer
  vals = JSON.parse(shared.payload.toString()) 
  cnt = 1
  vals.forEach(elem => {
    // returned objects must contain id and value at least
    shared.dataArray.push({
      id: 'scrVal' + cnt,
      value: elem,
      qualityOk: true,
      timestamp: new Date().getTime()
    })
    cnt++
  })
  
  `,
    })
  }

  const deleteProtocolConnection = async () => {
    dialogDelConn.value = false
    try {
      const response = await fetch('/Invoke/auth/deleteProtocolConnection', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          protocolConnectionNumber:
            editedConnection.value.protocolConnectionNumber,
          protocolDriver: editedConnection.value.protocolDriver,
          protocolDriverInstanceNumber:
            editedConnection.value.protocolDriverInstanceNumber,
          deleteTags: deleteTags.value,
          _id: editedConnection.value._id,
        }),
      })
      const json = await response.json()
      if (json.error) console.warn(json)
      fetchProtocolConnections() // refreshes connections
    } catch (err) {
      console.warn(err)
    }
  }

  const fetchProtocolConnections = async () => {
    try {
      const response = await fetch('/Invoke/auth/listProtocolConnections')
      const json = await response.json()
      for (let i = 0; i < json.length; i++) {
        json[i].id = i + 1
      }
      protocolConnections.value = json
    } catch (err) {
      console.warn(err)
    }
  }

  const openAddProtocolConnectionDialog = async () => {
    await fetchProtocolDriverInstancesByType()
    try {
      const response = await fetch('/Invoke/auth/getProtocolConnectionModel')
      const json = await response.json()
      if (json.error || !json.protocolConnection) {
        console.warn(json)
        error.value = true
        return
      }
      if ('_id' in json.protocolConnection) delete json.protocolConnection._id
      editedConnection.value = Object.assign({}, json.protocolConnection)
      dialogEditConnection.value = true
    } catch (err) {
      console.warn(err)
      error.value = true
    }
  }

  const openEditProtocolConnectionDialog = async (item) => {
    await fetchProtocolDriverInstancesByType()
    editedConnection.value =  Object.assign({}, item)
    dialogEditConnection.value = true
  }

  const openDeleteProtocolConnectionDialog = (item) => {
    deleteTags.value = false
    editedConnection.value = item
    dialogDelConn.value = true
  }

  const updateOrCreateProtocolConnection = async () => {
    if (editedConnection.value._id) {
      await updateProtocolConnection()
    } else {
      await createProtocolConnection()
    }
  }

  async function fetchProtocolDriverInstancesByType() {
    try {
      const res = await fetch('/Invoke/auth/listProtocolDriverInstances')
      const json = await res.json()
      if (json.error) {
        console.warn(json)
        return
      }
      const instances = json
      instances.forEach((instance) => {
        driverInstancesByType.value[instance.protocolDriver] = []
      })
      instances.forEach((instance) => {
        driverInstancesByType.value[instance.protocolDriver].push(
          instance.protocolDriverInstanceNumber
        )
      })
    } catch (err) {
      console.warn(err)
    }
  }
</script>

<style>
  .v-chip {
    max-width: 300px;
  }

  .v-chip__content {
    padding: 0px 4px;
    display: inline-block !important;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .v-textarea textarea {
    line-height: 1.1em !important;
  }
</style>
