<template>
  <v-card>
    <v-row class="pa-4" justify="space-between">
      <v-col>
        <v-treeview
          style="max-height: 500px; min-width: 250px"
          class="overflow-y-auto overflow-x-hidden"
          :active.sync="active"
          :items="items"
          :load-children="fetchProtocolConnections"
          :open.sync="open"
          activatable
          color="primary"
          open-on-click
          transition
          open-all
        >
          <template v-slot:prepend="{ item }">
            <v-icon v-if="!item.children"> mdi-swap-horizontal </v-icon>
          </template>
        </v-treeview>
        <v-btn
          class="mt-6"
          dark
          x-small
          color="blue"
          @click="createProtocolConnection($event)"
        >
          <v-icon dark> mdi-plus </v-icon>
          {{ $t("src\\components\\connections.newConnection") }}
        </v-btn>
      </v-col>

      <v-divider vertical></v-divider>

      <v-col class="d-flex text-left">
        <v-scroll-y-transition mode="out-in">
          <div
            v-if="!selected"
            class="title grey--text text--lighten-1 font-weight-light"
            style="align-self: center"
          >
            {{ $t("src\\components\\connections.selectConnection") }}
          </div>
          <v-card
            v-else
            :key="selected.id"
            class="pt-6 mx-auto"
            flat
            max-width="600"
          >
            <v-row class="pb-8 mx-auto" justify="space-between">
              <v-text-field
                prepend-inner-icon="mdi-swap-horizontal"
                type="text"
                outlined
                clearable
                :input-value="active"
                :label="$t('src\\components\\connections.connectionName')"
                hide-details="auto"
                v-model="selected.name"
                @change="updateProtocolConnection"
              ></v-text-field>

              <v-tooltip bottom>
                <template v-slot:activator="{ on, attrs }">
                  <v-btn
                    v-bind="attrs"
                    v-on="on"
                    class="mx-2"
                    fab
                    dark
                    x-small
                    color="red"
                    @click="dialogDelConn = true"
                  >
                    <v-icon dark> mdi-minus </v-icon>
                  </v-btn>
                </template>
                <span>{{
                  $t("src\\components\\connections.deleteConnection")
                }}</span>
              </v-tooltip>

              <v-dialog v-model="dialogDelConn" max-width="400">
                <v-card>
                  <v-card-title class="headline">
                    {{ $t("src\\components\\connections.deleteConnection") }}
                  </v-card-title>

                  <v-card-text>
                    {{
                      $t("src\\components\\connections.deleteConnectionConfirm")
                    }}
                  </v-card-text>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn
                      color="green darken-1"
                      text
                      @click="dialogDelConn = false"
                    >
                      {{
                        $t(
                          "src\\components\\connections.deleteConnectionCancel"
                        )
                      }}
                    </v-btn>

                    <v-btn
                      color="red darken-1"
                      text
                      @click="
                        dialogDelConn = false;
                        deleteProtocolConnection($event);
                      "
                    >
                      {{
                        $t(
                          "src\\components\\connections.deleteConnectionExecute"
                        )
                      }}
                    </v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>
            </v-row>

            <v-text-field
              prepend-inner-icon="mdi-swap-horizontal"
              type="number"
              outlined
              min="1"
              clearable
              :input-value="active"
              :label="$t('src\\components\\connections.connectionNumber')"
              hide-details="auto"
              v-model="selected.protocolConnectionNumber"
              @change="updateProtocolConnection"
              class="pb-6 mx-auto"
            ></v-text-field>

            <v-text-field
              type="text"
              outlined
              clearable
              :input-value="active"
              :label="$t('src\\components\\connections.description')"
              hide-details="auto"
              v-model="selected.description"
              @change="updateProtocolConnection"
            ></v-text-field>

            <v-switch
              v-model="selected.enabled"
              inset
              color="primary"
              :label="`${$t('src\\components\\connections.enabled')}${
                selected.enabled ? $t('src\\components\\connections.enabledTrue') : $t('src\\components\\connections.enabledFalse')
              }`"
              @change="updateProtocolConnection"
              class="mb-0"
            ></v-switch>

            <v-switch
              v-if="!['TELEGRAF-LISTENER'].includes(selected.protocolDriver)"
              v-model="selected.commandsEnabled"
              inset
              color="primary"
              :label="`${$t('src\\components\\connections.cmdEnabled')}${
                selected.commandsEnabled
                  ? $t('src\\components\\connections.cmdEnabledTrue')
                  : $t('src\\components\\connections.cmdEnabledFalse')
              }`"
              @change="updateProtocolConnection"
              class="mt-0"
            ></v-switch>

            <v-select
              prepend-inner-icon="mdi-cogs"
              :items="driverNameItems"
              :label="$t('src\\components\\connections.protocolDriver')"
              v-model="selected.protocolDriver"
              outlined
              @change="updateProtocolConnection"
            ></v-select>

            <v-text-field
              class="pb-8"
              prepend-inner-icon="mdi-play-circle"
              type="number"
              outlined
              min="1"
              clearable
              :input-value="active"
              :label="$t('src\\components\\connections.instanceNumber')"
              hide-details="auto"
              v-model="selected.protocolDriverInstanceNumber"
              @change="updateProtocolConnection"
            ></v-text-field>

            <v-card class="mx-auto" tile>
              <v-list flat dense shaped subheader>
                <v-subheader>
                  {{
                    $t(
                      "src\\components\\connections.protocolConnectionParameters"
                    )
                  }}
                </v-subheader>

                <v-list-item-group multiple active-class="">
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
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.localLinkAddress')"
                          hide-details="auto"
                          v-model="selected.localLinkAddress"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t(
                              "src\\components\\connections.localLinkAddressTitle"
                            )
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t(
                              "src\\components\\connections.localLinkAddressHint"
                            )
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
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
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.remoteLinkAddress')"
                          hide-details="auto"
                          v-model="selected.remoteLinkAddress"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t(
                              "src\\components\\connections.remoteLinkAddressTitle"
                            )
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t(
                              "src\\components\\connections.remoteLinkAddressHint"
                            )
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['OPC-UA'].includes(selected.protocolDriver)"
                  >
                    <v-autocomplete
                      v-model="selected.endpointURLs"
                      :items="selected.endpointURLs"
                      chips
                      small-chips
                      deletable-chips
                      :label="$t('src\\components\\connections.remoteEndpointsUrls')"
                      multiple
                      @change="updateProtocolConnection"
                    ></v-autocomplete>

                    <v-tooltip bottom>
                      <template v-slot:activator="{ on, attrs }">
                        <v-btn
                          v-bind="attrs"
                          v-on="on"
                          class="mx-2"
                          fab
                          dark
                          x-small
                          color="blue"
                          @click="dialogAddURL = true"
                        >
                          <v-icon dark> mdi-plus </v-icon>
                        </v-btn>
                      </template>
                      <span>{{
                        $t("src\\components\\connections.remoteEndpointsAddNew")
                      }}</span>
                    </v-tooltip>
                    <v-dialog
                      v-model="dialogAddURL"
                      max-width="450"
                      class="pa-8"
                    >
                      <v-card>
                        <v-card-title class="headline">
                          {{
                            $t(
                              "src\\components\\connections.remoteEndpointsAddNew"
                            )
                          }}
                        </v-card-title>

                        <v-card-title class="headline">
                          <v-text-field
                            :label="$t('src\\components\\connections.remoteEndpointsNewUrl')"
                            v-model="newURL"
                            :rules="[rules.required, rules.opcUrl]"
                          ></v-text-field>
                        </v-card-title>

                        <v-card-actions>
                          <v-spacer></v-spacer>

                          <v-btn
                            color="green darken-1"
                            text
                            @click="dialogAddURL = false"
                          >
                            {{
                              $t(
                                "src\\components\\connections.remoteEndpointsNewUrlCancel"
                              )
                            }}
                          </v-btn>

                          <v-btn
                            color="blue darken-1"
                            text
                            @click="
                              dialogAddURL = false;
                              addNewURL($event);
                            "
                          >
                            {{
                              $t(
                                "src\\components\\connections.remoteEndpointsNewUrlExecute"
                              )
                            }}
                          </v-btn>
                        </v-card-actions>
                      </v-card>
                    </v-dialog>
                  </v-list-item>

                  <v-list-item
                    v-if="['OPC-UA'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="string"
                          :input-value="active"
                          :label="$t('src\\components\\connections.configFileName')"
                          hide-details="auto"
                          v-model="selected.configFileName"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t(
                              "src\\components\\connections.configFileNameTitle"
                            )
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t(
                              "src\\components\\connections.configFileNameHint"
                            )
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    class="ma-0"
                    v-if="['OPC-UA'].includes(selected.protocolDriver)"
                  >
                    <v-switch
                      class="ma-0"
                      v-model="selected.useSecurity"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.useSecurity')}${
                        selected.useSecurity
                          ? $t('src\\components\\connections.useSecurityTrue')
                          : $t('src\\components\\connections.useSecurityFalse')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>

                  <v-list-item
                    class="ma-0"
                    v-if="
                      ['OPC-UA', 'TELEGRAF-LISTENER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <v-switch
                      class="ma-0"
                      v-model="selected.autoCreateTags"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.autoCreateTags')}${
                        selected.autoCreateTags
                          ? $t('src\\components\\connections.autoCreateTagsTrue')
                          : $t('src\\components\\connections.autoCreateTagsFalse')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['OPC-UA'].includes(selected.protocolDriver) &&
                      selected.autoCreateTags
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="$t('src\\components\\connections.publishingInterval')"
                          hide-details="auto"
                          v-model="selected.autoCreateTagPublishingInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t(
                              "src\\components\\connections.publishingIntervalTitle"
                            )
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t(
                              "src\\components\\connections.publishingIntervalHint"
                            )
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['OPC-UA'].includes(selected.protocolDriver) &&
                      selected.autoCreateTags
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="$t('src\\components\\connections.samplingInterval')"
                          hide-details="auto"
                          v-model="selected.autoCreateTagSamplingInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t(
                              "src\\components\\connections.samplingIntervalTitle"
                            )
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t(
                              "src\\components\\connections.samplingIntervalHint"
                            )
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['OPC-UA'].includes(selected.protocolDriver) &&
                      selected.autoCreateTags
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="$t('src\\components\\connections.serverQueueSize')"
                          hide-details="auto"
                          v-model="selected.autoCreateTagQueueSize"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t(
                              "src\\components\\connections.serverQueueSizeTitle"
                            )
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t(
                              "src\\components\\connections.serverQueueSizeHint"
                            )
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['OPC-UA'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="$t('src\\components\\connections.timeoutKeepalive')"
                          hide-details="auto"
                          v-model="selected.timeoutMs"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t(
                              "src\\components\\connections.timeoutKeepaliveTitle"
                            )
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t(
                              "src\\components\\connections.timeoutKeepaliveHint"
                            )
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-101',
                        'IEC60870-5-104',
                        'DNP3',
                        'I104M',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.giInterval')"
                          hide-details="auto"
                          v-model="selected.giInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t("src\\components\\connections.giIntervalTitle")
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t("src\\components\\connections.giIntervalHint")
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101', 'IEC60870-5-104'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.testCmdInterval')"
                          hide-details="auto"
                          v-model="selected.testCommandInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t(
                              "src\\components\\connections.testCmdIntervalTitle"
                            )
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t(
                              "src\\components\\connections.testCmdIntervalHint"
                            )
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101', 'IEC60870-5-104', 'PLCTAG'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.timeSyncInterval')"
                          hide-details="auto"
                          v-model="selected.timeSyncInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t(
                              "src\\components\\connections.timeSyncIntervalTitle"
                            )
                          }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{
                            $t(
                              "src\\components\\connections.timeSyncIntervalHint"
                            )
                          }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.k')"
                          hide-details="auto"
                          v-model="selected.k"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{ $t("src\\components\\connections.kTitle") }}
                        </v-list-item-title>
                        <v-list-item-subtitle>
                          {{ $t("src\\components\\connections.kHint") }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.w')"
                          hide-details="auto"
                          v-model="selected.w"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{ $t("src\\components\\connections.wTitle") }}
                        </v-list-item-title>
                        <v-list-item-subtitle>{{
                          $t("src\\components\\connections.wHint")
                        }}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.t0')"
                          hide-details="auto"
                          v-model="selected.t0"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t("src\\components\\connections.t0Title")
                          }}</v-list-item-title
                        >
                        <v-list-item-subtitle>{{
                          $t("src\\components\\connections.t0Hint")
                        }}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.t1')"
                          hide-details="auto"
                          v-model="selected.t1"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{
                          $t("src\\components\\connections.t1Title")
                        }}</v-list-item-title>
                        <v-list-item-subtitle>{{
                          $t("src\\components\\connections.t1Hint")
                        }}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.t2')"
                          hide-details="auto"
                          v-model="selected.t2"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{
                          $t("src\\components\\connections.t2Title")
                        }}</v-list-item-title>
                        <v-list-item-subtitle>{{
                          $t("src\\components\\connections.t2Hint")
                        }}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.t3')"
                          hide-details="auto"
                          v-model="selected.t3"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{
                          $t("src\\components\\connections.t3Title")
                        }}</v-list-item-title>
                        <v-list-item-subtitle>{{
                          $t("src\\components\\connections.t3Hint")
                        }}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-101',
                        'IEC60870-5-104',
                        'IEC60870-5-101_SERVER',
                        'IEC60870-5-104_SERVER',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-select
                          :items="itemsSizeOfCOT"
                          :input-value="active"
                          hide-details="auto"
                          v-model="selected.sizeOfCOT"
                          :label="$t('src\\components\\connections.sizeOfCot')"
                        ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{
                            $t("src\\components\\connections.sizeOfCotTitle")
                          }}</v-list-item-title
                        >
                        <v-list-item-subtitle>
                          {{ $t("src\\components\\connections.sizeOfCotHint") }}
                        </v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-101',
                        'IEC60870-5-104',
                        'IEC60870-5-101_SERVER',
                        'IEC60870-5-104_SERVER',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-select
                          :items="itemsSizeOfCA"
                          :input-value="active"
                          hide-details="auto"
                          v-model="selected.sizeOfCA"
                          :label="$t('src\\components\\connections.sizeOfCa')"
                        ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{
                          $t("src\\components\\connections.sizeOfCaTitle")
                        }}</v-list-item-title>
                        <v-list-item-subtitle>{{
                          $t("src\\components\\connections.sizeOfCaHint")
                        }}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-101',
                        'IEC60870-5-104',
                        'IEC60870-5-101_SERVER',
                        'IEC60870-5-104_SERVER',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-select
                          :items="itemsSizeOfIOA"
                          :input-value="active"
                          hide-details="auto"
                          v-model="selected.sizeOfIOA"
                          label="Size of IOA"
                        ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{
                          $t("src\\components\\connections.sizeOfIoaTitle")
                        }}</v-list-item-title>
                        <v-list-item-subtitle>{{
                          $t("src\\components\\connections.sizeOfIoaHint")
                        }}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <v-switch
                      v-model="selected.serverModeMultiActive"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.modeMultiActive')}${
                        selected.serverModeMultiActive
                          ? $t('src\\components\\connections.modeMultiActiveTrue')
                          : $t('src\\components\\connections.modeMultiActiveFalse')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.maxClientConnections')"
                          hide-details="auto"
                          v-model="selected.maxClientConnections"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.maxClientConnectionsTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.maxClientConnectionsHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-104_SERVER',
                        'IEC60870-5-101_SERVER',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.maxQueueSize')"
                          hide-details="auto"
                          v-model="selected.maxQueueSize"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.maxQueueSizeTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.maxQueueSizeHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.class0Scan')"
                          hide-details="auto"
                          v-model="selected.class0ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.class0ScanTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.class0ScanHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.class1Scan')"
                          hide-details="auto"
                          v-model="selected.class1ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.class1ScanTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.class1ScanHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.class2Scan')"
                          hide-details="auto"
                          v-model="selected.class2ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.class2ScanTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.class2ScanHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.class3Scan')"
                          hide-details="auto"
                          v-model="selected.class3ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.class3ScanTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.class3ScanHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.timeSyncMode')"
                          hide-details="auto"
                          v-model="selected.timeSyncMode"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.timeSyncModeTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.timeSyncModeHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <v-switch
                      v-model="selected.enableUnsolicited"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.enableUnsolicited')}${
                        selected.enableUnsolicited
                          ? $t('src\\components\\connections.enableUnsolicitedTrue')
                          : $t('src\\components\\connections.enableUnsolicitedFalse')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <v-autocomplete
                      v-model="selected.rangeScansStr"
                      :items="selected.rangeScansStr"
                      chips
                      small-chips
                      deletable-chips
                      :label="$t('src\\components\\connections.rangeScans')"
                      multiple
                      @change="updateProtocolConnection"
                    ></v-autocomplete>

                    <v-tooltip bottom>
                      <template v-slot:activator="{ on, attrs }">
                        <v-btn
                          v-bind="attrs"
                          v-on="on"
                          class="mx-2"
                          fab
                          dark
                          x-small
                          color="blue"
                          @click="dialogAddRangeScan = true"
                        >
                          <v-icon dark> mdi-plus </v-icon>
                        </v-btn>
                      </template>
                      <span>{{$t("src\\components\\connections.rangeScanAddNew")}}</span>
                    </v-tooltip>
                    <v-dialog
                      v-model="dialogAddRangeScan"
                      max-width="400"
                      class="pa-8"
                    >
                      <v-card>
                        <v-card-title class="headline">
                          {{$t("src\\components\\connections.rangeScanAddNew")}}
                        </v-card-title>

                        <v-card-title class="headline">
                          <v-text-field
                            :label="$t('src\\components\\connections.rangeScanGroup')"
                            type="number"
                            min="1"
                            v-model="newRangeScan.group"
                          ></v-text-field>

                          <v-text-field
                            :label="$t('src\\components\\connections.rangeScanVariation')"
                            type="number"
                            min="0"
                            v-model="newRangeScan.variation"
                          ></v-text-field>

                          <v-text-field
                            :label="$t('src\\components\\connections.rangeScanStart')"
                            type="number"
                            min="0"
                            v-model="newRangeScan.startAddress"
                          ></v-text-field>

                          <v-text-field
                            :label="$t('src\\components\\connections.rangeScanStop')"
                            type="number"
                            min="0"
                            v-model="newRangeScan.stopAddress"
                          ></v-text-field>

                          <v-text-field
                            :label="$t('src\\components\\connections.rangeScanPeriod')"
                            type="number"
                            min="1"
                            v-model="newRangeScan.period"
                          ></v-text-field>
                        </v-card-title>

                        <v-card-actions>
                          <v-spacer></v-spacer>

                          <v-btn
                            color="green darken-1"
                            text
                            @click="dialogAddRangeScan = false"
                          >
                          {{$t("src\\components\\connections.rangeScanAddCancel")}}
                          </v-btn>

                          <v-btn
                            color="blue darken-1"
                            text
                            @click="
                              dialogAddIP = false;
                              addNewRangeScan($event);
                            "
                          >
                          {{$t("src\\components\\connections.rangeScanAddExecute")}}
                          </v-btn>
                        </v-card-actions>
                      </v-card>
                    </v-dialog>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.timeoutAck')"
                          hide-details="auto"
                          v-model="selected.timeoutForACK"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.timeoutAckTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.timeoutAckHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="1"
                          :input-value="active"
                          :label="$t('src\\components\\connections.timeoutRepeat')"
                          hide-details="auto"
                          v-model="selected.timeoutRepeat"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.timeoutRepeatTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.timeoutRepeatHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-select
                          :items="sizeOfLinkAddressItems"
                          :input-value="active"
                          hide-details="auto"
                          v-model="selected.sizeOfLinkAddress"
                          :label="$t('src\\components\\connections.sizeOfLinkAddress')"
                        ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.sizeOfLinkAddressTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.sizeOfLinkAddressHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <v-switch
                      v-model="selected.useSingleCharACK"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.singleCharAck')}${
                        selected.useSingleCharACK
                          ? $t('src\\components\\connections.singleCharAckTrue')
                          : $t('src\\components\\connections.singleCharAckFalse')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>
                </v-list-item-group>
              </v-list>
            </v-card>

            <v-card
              class="mt-6"
              tile
              v-if="
                [
                  'IEC60870-5-104',
                  'IEC60870-5-104_SERVER',
                  'DNP3',
                  'DNP3_SERVER',
                  'I104M',
                  'MODBUS',
                  'PLCTAG',
                  'TELEGRAF-LISTENER',
                ].includes(selected.protocolDriver)
              "
            >
              <v-list flat dense shaped subheader>
                <v-subheader>{{$t("src\\components\\connections.tcpParameters")}}</v-subheader>
                <v-list-item-group>
                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-104_SERVER',
                        'DNP3',
                        'DNP3_SERVER',
                        'I104M',
                        'TELEGRAF-LISTENER',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :rules="[rules.required, rules.ipPort]"
                          :input-value="active"
                          :label="$t('src\\components\\connections.bindIpPort')"
                          hide-details="auto"
                          v-model="selected.ipAddressLocalBind"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>
                        {{$t("src\\components\\connections.bindIpPortTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.bindIpPortHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-104_SERVER',
                        'IEC60870-5-104',
                        'DNP3_SERVER',
                        'DNP3',
                        'I104M',
                        'PLCTAG',
                        'MODBUS',
                        'TELEGRAF-LISTENER',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <v-autocomplete
                      v-model="selected.ipAddresses"
                      :items="selected.ipAddresses"
                      chips
                      small-chips
                      deletable-chips
                      :label="$t('src\\components\\connections.remoteIpAddresses')"
                      multiple
                      @change="updateProtocolConnection"
                    ></v-autocomplete>

                    <v-tooltip bottom>
                      <template v-slot:activator="{ on, attrs }">
                        <v-btn
                          v-bind="attrs"
                          v-on="on"
                          class="mx-2"
                          fab
                          dark
                          x-small
                          color="blue"
                          @click="dialogAddIP = true"
                        >
                          <v-icon dark> mdi-plus </v-icon>
                        </v-btn>
                      </template>
                      <span>{{$t("src\\components\\connections.remoteIpAddressAdd")}}</span>
                    </v-tooltip>
                    <v-dialog
                      v-model="dialogAddIP"
                      max-width="400"
                      class="pa-8"
                    >
                      <v-card>
                        <v-card-title class="headline">
                          {{$t("src\\components\\connections.remoteIpAddressAdd")}}
                        </v-card-title>

                        <v-card-title class="headline">
                          <v-text-field
                            label="New IP"
                            v-model="newIP"
                            :rules="[rules.required, rules.ip]"
                          ></v-text-field>
                        </v-card-title>

                        <v-card-actions>
                          <v-spacer></v-spacer>

                          <v-btn
                            color="green darken-1"
                            text
                            @click="dialogAddIP = false"
                          >
                          {{$t("src\\components\\connections.remoteIpAddressAddCancel")}}
                          </v-btn>

                          <v-btn
                            color="blue darken-1"
                            text
                            @click="
                              dialogAddIP = false;
                              addNewIP($event);
                            "
                          >
                          {{$t("src\\components\\connections.remoteIpAddressAddExecute")}}
                          </v-btn>
                        </v-card-actions>
                      </v-card>
                    </v-dialog>
                  </v-list-item>
                </v-list-item-group>
              </v-list>
            </v-card>

            <v-card
              class="mt-6"
              tile
              v-if="
                ['IEC60870-5-104', 'DNP3'].includes(selected.protocolDriver)
              "
            >
              <v-list flat dense shaped subheader>
                <v-subheader
                  >TLS Parameters (leave blank for unencrypted
                  connections)</v-subheader
                >
                <v-list-item-group>
                  <v-list-item
                    v-if="
                      ['IEC60870-5-104', 'DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="$t('src\\components\\connections.localCertificateFile')"
                          hide-details="auto"
                          v-model="selected.localCertFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{$t("src\\components\\connections.localCertificateFileTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.localCertificateFileHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104', 'DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="$t('src\\components\\connections.peerCertificateFile')"
                          hide-details="auto"
                          v-model="selected.peerCertFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.peerCertificateFileTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.peerCertificateFileHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['IEC60870-5-104'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="$t('src\\components\\connections.rootCertificateFile')"
                          hide-details="auto"
                          v-model="selected.rootCertFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.rootCertificateFileTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.rootCertificateFileHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="$t('src\\components\\connections.privateCertificateFile')"
                          hide-details="auto"
                          v-model="selected.privateKeyFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.privateCertificateFileTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.privateCertificateFileHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="$t('src\\components\\connections.opensslCypherList')"
                          hide-details="auto"
                          v-model="selected.cipherList"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                        {{$t("src\\components\\connections.opensslCypherListTitle")}}
                        </v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.opensslCypherListHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <v-switch
                      v-model="selected.allowTLSv10"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.allowTls10')}${
                        selected.allowTLSv10
                          ? $t('src\\components\\connections.allowTls10True')
                          : $t('src\\components\\connections.allowTls10False')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <v-switch
                      v-model="selected.allowTLSv11"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.allowTls11')}${
                        selected.allowTLSv10
                          ? $t('src\\components\\connections.allowTls11True')
                          : $t('src\\components\\connections.allowTls11False')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <v-switch
                      v-model="selected.allowTLSv12"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.allowTls12')}${
                        selected.allowTLSv10
                          ? $t('src\\components\\connections.allowTls12True')
                          : $t('src\\components\\connections.allowTls12False')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <v-switch
                      v-model="selected.allowTLSv13"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.allowTls13')}${
                        selected.allowTLSv10
                          ? $t('src\\components\\connections.allowTls13True')
                          : $t('src\\components\\connections.allowTls13False')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="['IEC60870-5-104'].includes(selected.protocolDriver)"
                  >
                    <v-switch
                      v-model="selected.allowOnlySpecificCertificates"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.allowSpecificCerts')}${
                        selected.allowOnlySpecificCertificates
                          ? $t('src\\components\\connections.allowSpecificCertsTrue')
                          : $t('src\\components\\connections.allowSpecificCertsFalse')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="['IEC60870-5-104'].includes(selected.protocolDriver)"
                  >
                    <v-switch
                      v-model="selected.chainValidation"
                      inset
                      color="primary"
                      :label="`${$t('src\\components\\connections.certChainValidation')}${
                        selected.chainValidation
                          ? $t('src\\components\\connections.certChainValidationTrue')
                          : $t('src\\components\\connections.certChainValidationFalse')
                      }`"
                      @change="updateProtocolConnection"
                    ></v-switch>
                  </v-list-item>
                </v-list-item-group>
              </v-list>
            </v-card>

            <v-card
              class="mt-6"
              tile
              v-if="
                [
                  'IEC60870-5-101',
                  'IEC60870-5-101_SERVER',
                  'DNP3',
                  'DNP3_SERVER',
                  'MODBUS',
                ].includes(selected.protocolDriver)
              "
            >
              <v-list flat dense shaped subheader>
                <v-subheader
                  >Serial Parameters (leave blank for network
                  connections)</v-subheader
                >
                <v-list-item-group>
                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-101',
                        'IEC60870-5-101_SERVER',
                        'DNP3',
                        'DNP3_SERVER',
                        'MODBUS',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="$t('src\\components\\connections.commPortName')"
                          hide-details="auto"
                          v-model="selected.portName"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>
                          {{$t("src\\components\\connections.commPortNameTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.commPortNameHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
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
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="150"
                          :input-value="active"
                          :label="$t('src\\components\\connections.baudRate')"
                          hide-details="auto"
                          v-model="selected.baudRate"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.baudRateTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.baudRateHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
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
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-select
                          :items="parityItems"
                          :input-value="active"
                          hide-details="auto"
                          v-model="selected.parity"
                          :label="$t('src\\components\\connections.parity')"
                        ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.parityTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.parityHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
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
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-select
                          :items="stopBitsItems"
                          :input-value="active"
                          hide-details="auto"
                          v-model="selected.stopBits"
                          :label="$t('src\\components\\connections.stopBits')"
                        ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.stopBitsTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.stopBitsHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
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
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-select
                          :items="handshakeItems"
                          :input-value="active"
                          hide-details="auto"
                          v-model="selected.handshake"
                          :label="$t('src\\components\\connections.handshake')"
                        ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.handshakeTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.handshakeHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="['DNP3'].includes(selected.protocolDriver)"
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          :label="$t('src\\components\\connections.asyncOpenDelay')"
                          hide-details="auto"
                          v-model="selected.asyncOpenDelay"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\connections.asyncOpenDelayTitle")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\connections.asyncOpenDelayHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>
                </v-list-item-group>
              </v-list>
            </v-card>
          </v-card>
        </v-scroll-y-transition>
      </v-col>
    </v-row>
  </v-card>
</template>

<script>

export default {
  name: "ProtocolConnections",

  data: () => ({
    itemsSizeOfCOT: [1, 2],
    itemsSizeOfCA: [1, 2],
    itemsSizeOfIOA: [1, 2, 3],
    dialogAddIP: false,
    dialogAddURL: false,
    dialogAddRangeScan: false,
    dialogDelConn: false,
    newRangeScan: {
      group: 1,
      variation: 0,
      startAddress: 0,
      stopAddress: 0,
      period: 300,
    },
    newIP: "",
    newURL: "",
    active: [],
    open: [],
    rules: {
      required: (value) => !!value || "Required.",
      counter: (value) => value.length <= 20 || "Max 20 characters",
      ip: (value) => {
        const pattern = /\b((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4}\b/;
        return pattern.test(value) || "Invalid IP Address.";
      },
      ipPort: (value) => {
        const pattern = /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]):[0-9]+$/;
        return pattern.test(value) || "Invalid IP Address:Port.";
      },
      opcUrl: (value) => {
        const pattern = /^opc\.tcp:\/\/[a-zA-Z0-9-_]+[:./\\]+([a-zA-Z0-9 -_./:=&"'?%+@#$!])+$/;
        return pattern.test(value) || "Invalid OPC-UA URL.";
      },
      email: (value) => {
        const pattern = /^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
        return pattern.test(value) || "Invalid e-mail.";
      },
    },
    driverNameItems: [
      "IEC60870-5-104",
      "IEC60870-5-104_SERVER",
      "IEC60870-5-101",
      "IEC60870-5-101_SERVER",
      "DNP3",
      "OPC-UA",
      "PLCTAG",
      "I104M",
      "TELEGRAF-LISTENER",
    ],
    parityItems: ["None", "Even", "Odd", "Mark", "Space"],
    stopBitsItems: ["One", "One5", "Two"],
    handshakeItems: ["None", "Rts", "Xon", "RtsXon"],
    sizeOfLinkAddressItems: [0, 1, 2],
    timeSyncModeItems: [0, 1, 2],
    protocolConnections: [],
  }),

  computed: {
    items() {
      return [
        {
          name: "Protocol Connections",
          children: this.protocolConnections,
        },
      ];
    },
    selected() {
      if (!this.active.length) return undefined;

      const id = this.active[0];

      return this.protocolConnections.find((elem) => elem.id === id);
    },
  },

  watch: {
    // selected: "randomAvatar",
  },

  methods: {
    async updateProtocolConnection() {
      var connDup = Object.assign({}, this.selected);
      delete connDup["id"];

      if ("rangeScans" in connDup) {
        connDup.rangeScans = [];
        for (let i = 0; i < connDup.rangeScansStr.length; i++)
          connDup.rangeScans.push(JSON.parse(connDup.rangeScansStr[i]));
      }
      delete connDup["rangeScansStr"];
      return await fetch("/Invoke/auth/updateProtocolConnection", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify(connDup),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchProtocolConnections(); // refreshes connections
        })
        .catch((err) => console.warn(err));
    },
    async createProtocolConnection() {
      return await fetch("/Invoke/auth/createProtocolConnection", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({}),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchProtocolConnections(); // refreshes connections
        })
        .catch((err) => console.warn(err));
    },
    async addNewRangeScan() {
      this.selected.rangeScansStr.push(JSON.stringify(this.newRangeScan));
      this.updateProtocolConnection();
    },
    async addNewIP() {
      if (this.rules.ip(this.newIP) !== true) return;
      if (this.newIP != "" && !this.selected.ipAddresses.includes(this.newIP)) {
        this.selected.ipAddresses.push(this.newIP);
        this.updateProtocolConnection();
        this.newIP = "";
      }
    },
    async addNewURL() {
      if (this.rules.opcUrl(this.newURL) !== true) return;
      if (
        this.newURL != "" &&
        !this.selected.endpointURLs.includes(this.newURL)
      ) {
        this.selected.endpointURLs.push(this.newURL);
        this.updateProtocolConnection();
        this.newURL = "";
      }
    },
    async deleteProtocolConnection() {
      return await fetch("/Invoke/auth/deleteProtocolConnection", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          protocolConnectionNumber: this.selected.protocolConnectionNumber,
          protocolDriver: this.selected.protocolDriver,
          protocolDriverInstanceNumber: this.selected
            .protocolDriverInstanceNumber,
          _id: this.selected._id,
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchProtocolConnections(); // refreshes connections
        })
        .catch((err) => console.warn(err));
    },
    async fetchProtocolConnections() {
      return await fetch("/Invoke/auth/listProtocolConnections")
        .then((res) => res.json())
        .then((json) => {
          for (let i = 0; i < json.length; i++) {
            json[i].id = i + 1;

            json[i].rangeScansStr = [];
            if ("rangeScans" in json[i])
              for (let j = 0; j < json[i].rangeScans.length; j++)
                json[i].rangeScansStr.push(
                  JSON.stringify(json[i].rangeScans[j])
                );
          }
          this.protocolConnections.length = 0;
          this.protocolConnections.push(...json);
        })
        .catch((err) => console.warn(err));
    },
  },
};
</script>