<template>
  <v-card>
    <v-row class="pa-4" justify="space-between">
      <v-col>
        <v-treeview
          style="max-height: 500px;min-width: 250px"
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
          {{msg.connNewConnection}}
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
            {{msg.connSelectConnection}}
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
                :label="msg.connConnectionName"
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
                <span>{{msg.connDeleteConnection}}</span>
              </v-tooltip>

              <v-dialog v-model="dialogDelConn" max-width="400">
                <v-card>
                  <v-card-title class="headline">
                    {{msg.connDeleteConnection}}
                  </v-card-title>

                  <v-card-text>
                    {{msg.connDeleteConnectionConfirm}}
                  </v-card-text>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn
                      color="green darken-1"
                      text
                      @click="dialogDelConn = false"
                    >
                      {{msg.connDeleteConnectionCancel}}
                    </v-btn>

                    <v-btn
                      color="red darken-1"
                      text
                      @click="
                        dialogDelConn = false;
                        deleteProtocolConnection($event);
                      "
                    >
                      {{msg.connDeleteConnectionExecute}}
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
              :label="msg.connConnectionNumber"
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
              :label="msg.connDescription"
              hide-details="auto"
              v-model="selected.description"
              @change="updateProtocolConnection"
            ></v-text-field>

            <v-switch
              v-model="selected.enabled"
              inset
              color="primary"
              :label="`${msg.connEnabled}${selected.enabled?msg.connEnabledTrue:msg.connEnabledFalse}`"
              @change="updateProtocolConnection"
              class="mb-0"
            ></v-switch>

            <v-switch
              v-model="selected.commandsEnabled"
              inset
              color="primary"
              :label="`${msg.connCmdEnabled}${selected.commandsEnabled?msg.connCmdEnabledTrue:msg.connCmdEnabledFalse}`"
              @change="updateProtocolConnection"
              class="mt-0"
            ></v-switch>

            <v-select
              prepend-inner-icon="mdi-cogs"
              :items="driverNameItems"
              :label="msg.connProtocolDriver"
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
              :label="msg.connInstanceNumber"
              hide-details="auto"
              v-model="selected.protocolDriverInstanceNumber"
              @change="updateProtocolConnection"
            ></v-text-field>

            <v-card class="mx-auto" tile>
              <v-list flat dense shaped subheader>
                <v-subheader>{{msg.connProtocolConnectionParameters}}</v-subheader>

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
                          :label="msg.connLocalLinkAddress"
                          hide-details="auto"
                          v-model="selected.localLinkAddress"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connLocalLinkAddressTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connLocalLinkAddressHint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connRemoteLinkAddress"
                          hide-details="auto"
                          v-model="selected.remoteLinkAddress"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connRemoteLinkAddressTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connRemoteLinkAddressHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>


                  <v-list-item
                    v-if="
                      [
                        'OPC-UA'
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <v-autocomplete
                      v-model="selected.endpointURLs"
                      :items="selected.endpointURLs"
                      chips
                      small-chips
                      deletable-chips
                      :label="msg.connRemoteEndpointsUrls"
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
                      <span>{{msg.connRemoteEndpointsAddNew}}</span>
                    </v-tooltip>
                    <v-dialog
                      v-model="dialogAddURL"
                      max-width="400"
                      class="pa-8"
                    >
                      <v-card>
                        <v-card-title class="headline">
                          {{msg.connRemoteEndpointsAddNew}}
                        </v-card-title>

                        <v-card-title class="headline">
                          <v-text-field
                            :label="msg.connRemoteEndpointsNewUrl"
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
                            {{msg.connRemoteEndpointsNewUrlCancel}}
                          </v-btn>

                          <v-btn
                            color="blue darken-1"
                            text
                            @click="
                              dialogAddURL = false;
                              addNewURL($event);
                            "
                          >
                            {{msg.connRemoteEndpointsNewUrlExecute}}
                          </v-btn>
                        </v-card-actions>
                      </v-card>
                    </v-dialog>
                  </v-list-item>                  

                  <v-list-item                    
                    v-if="
                      [
                        'OPC-UA'
                      ].includes(selected.protocolDriver) 
                    ">
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="string"
                          :input-value="active"
                          :label="msg.connConfigFileName"
                          hide-details="auto"
                          v-model="selected.configFileName"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>{{msg.connConfigFileNameTitle}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{msg.connConfigFileNameHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item class="ma-0"
                    v-if="
                      [
                        'OPC-UA'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch class="ma-0"
                        v-model="selected.useSecurity"
                        inset
                        color="primary"
                        :label="`${msg.connUseSecurity}${selected.useSecurity?msg.connUseSecurityTrue:msg.connUseSecurityFalse}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>

                  <v-list-item class="ma-0"
                    v-if="
                      [
                        'OPC-UA'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch class="ma-0"
                        v-model="selected.autoCreateTags"
                        inset
                        color="primary"
                        :label="`${msg.connAutoCreateTags}${selected.autoCreateTags?msg.connAutoCreateTagsTrue:msg.connAutoCreateTagsFalse}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>

                    <v-list-item                    
                    v-if="
                      [
                        'OPC-UA'
                      ].includes(selected.protocolDriver) 
                      && selected.autoCreateTags
                    ">
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="msg.connPublishingInterval"
                          hide-details="auto"
                          v-model="selected.autoCreateTagPublishingInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>{{msg.connPublishingIntervalTitle}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{msg.connPublishingIntervalHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                    </v-list-item>

                    <v-list-item                    
                    v-if="
                      [
                        'OPC-UA'
                      ].includes(selected.protocolDriver)
                      && selected.autoCreateTags
                    ">
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="msg.connSamplingInterval"
                          hide-details="auto"
                          v-model="selected.autoCreateTagSamplingInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{msg.connSamplingIntervalTitle}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{msg.connSamplingIntervalHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                    <v-list-item                    
                    v-if="
                      [
                        'OPC-UA'
                      ].includes(selected.protocolDriver)
                      && selected.autoCreateTags
                    ">
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="msg.connServerQueueSize"
                          hide-details="auto"
                          v-model="selected.autoCreateTagQueueSize"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{msg.connServerQueueSizeTitle}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{msg.connServerQueueSizeHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                    <v-list-item                    
                    v-if="
                      [
                        'OPC-UA'
                      ].includes(selected.protocolDriver)
                    ">
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="msg.connTimeoutKeepalive"
                          hide-details="auto"
                          v-model="selected.timeoutMs"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title>{{msg.connTimeoutKeepaliveTitle}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{msg.connTimeoutKeepaliveHint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connGiInterval"
                          hide-details="auto"
                          v-model="selected.giInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                        >{{msg.connGiIntervalTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                        >{{msg.connGiIntervalHint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connTestCmdInterval"
                          hide-details="auto"
                          v-model="selected.testCommandInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connTestCmdIntervalTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connTestCmdIntervalHint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connTimeSyncInterval"
                          hide-details="auto"
                          v-model="selected.timeSyncInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connTimeSyncIntervalTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connTimeSyncIntervalHint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connK"
                          hide-details="auto"
                          v-model="selected.k"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connKTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connKHint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connW"
                          hide-details="auto"
                          v-model="selected.w"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connWTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connWHint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connT0"
                          hide-details="auto"
                          v-model="selected.t0"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connT0Title}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connT0Hint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connT1"
                          hide-details="auto"
                          v-model="selected.t1"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connT1Title}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connT1Hint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connT2"
                          hide-details="auto"
                          v-model="selected.t2"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connT2Title}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connT2Hint}}</v-list-item-subtitle
                        >
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
                          :label="msg.connT3"
                          hide-details="auto"
                          v-model="selected.t3"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connT3Title}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connT3Hint}}</v-list-item-subtitle
                        >
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
                        :label="msg.connSizeOfCot"
                      ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connSizeOfCotTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connSizeOfCotHint}}</v-list-item-subtitle
                        >
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
                        :label="msg.connSizeOfCa"
                      ></v-select>
                      </v-list-item-action>
                                            <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connSizeOfCaTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connSizeOfCaHint}}</v-list-item-subtitle
                        >
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
                        <v-list-item-title
                          >{{msg.connSizeOfIoaTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connSizeOfIoaHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>

                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-104_SERVER'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch
                        v-model="selected.serverModeMultiActive"
                        inset
                        color="primary"
                        :label="`${msg.connModeMultiActive}${selected.serverModeMultiActive?msg.connModeMultiActiveTrue:msg.connModeMultiActiveFalse}`"
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
                          :label="msg.connMaxClientConnections"
                          hide-details="auto"
                          v-model="selected.maxClientConnections"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connMaxClientConnectionsTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connMaxClientConnectionsHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104_SERVER','IEC60870-5-101_SERVER'].includes(
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
                          :label="msg.connMaxQueueSize"
                          hide-details="auto"
                          v-model="selected.maxQueueSize"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connMaxQueueSizeTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connMaxQueueSizeHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min=0
                          :input-value="active"
                          :label="msg.connClass0Scan"
                          hide-details="auto"
                          v-model="selected.class0ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connClass0ScanTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connClass0ScanHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min=0
                          :input-value="active"
                          :label="msg.connClass1Scan"
                          hide-details="auto"
                          v-model="selected.class1ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connClass1ScanTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connClass1ScanHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min=0
                          :input-value="active"
                          :label="msg.connClass2Scan"
                          hide-details="auto"
                          v-model="selected.class2ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connClass2ScanTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connClass2ScanHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min=0
                          :input-value="active"
                          :label="msg.connClass3Scan"
                          hide-details="auto"
                          v-model="selected.class3ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connClass3ScanTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connClass3ScanHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min=0
                          :input-value="active"
                          :label="msg.connTimeSyncMode"
                          hide-details="auto"
                          v-model="selected.timeSyncMode"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connTimeSyncModeTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connTimeSyncModeHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'DNP3'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch
                        v-model="selected.enableUnsolicited"
                        inset
                        color="primary"
                        :label="`${msg.connEnableUnsolicited}${selected.enableUnsolicited?msg.connEnableUnsolicitedTrue:msg.connEnableUnsolicitedFalse}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'DNP3',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <v-autocomplete
                      v-model="selected.rangeScansStr"
                      :items="selected.rangeScansStr"
                      chips
                      small-chips
                      deletable-chips
                      :label="msg.connRangeScans"
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
                      <span>{{msg.connRangeScanAddNew}}</span>
                    </v-tooltip>
                    <v-dialog
                      v-model="dialogAddRangeScan"
                      max-width="400"
                      class="pa-8"
                    >
                      <v-card>
                        <v-card-title class="headline">
                          {{msg.connRangeScanAddNew}}
                        </v-card-title>

                        <v-card-title class="headline">
                          <v-text-field
                            :label="msg.connRangeScanGroup"
                            type="number"
                            min=1
                            v-model="newRangeScan.group"
                          ></v-text-field>

                          <v-text-field
                            :label="msg.connRangeScanVariation"
                            type="number"
                            min=0
                            v-model="newRangeScan.variation"
                          ></v-text-field>

                          <v-text-field
                            :label="msg.connRangeScanStart"
                            type="number"
                            min=0
                            v-model="newRangeScan.startAddress"
                          ></v-text-field>

                          <v-text-field
                            :label="msg.connRangeScanStop"
                            type="number"
                            min=0
                            v-model="newRangeScan.stopAddress"
                          ></v-text-field>

                          <v-text-field
                            :label="msg.connRangeScanPeriod"
                            type="number"
                            min=1
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
                            {{msg.connRangeScanAddCancel}}
                          </v-btn>

                          <v-btn
                            color="blue darken-1"
                            text
                            @click="
                              dialogAddIP = false;
                              addNewRangeScan($event);
                            "
                          >
                            {{msg.connRangeScanAddExecute}}
                          </v-btn>
                        </v-card-actions>
                      </v-card>
                    </v-dialog>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101','IEC60870-5-101_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min=1
                          :input-value="active"
                          :label="msg.connTimeoutAck"
                          hide-details="auto"
                          v-model="selected.timeoutForACK"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connTimeoutAckTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connTimeoutAckHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101','IEC60870-5-101_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min=1
                          :input-value="active"
                          :label="msg.connTimeoutRepeat"
                          hide-details="auto"
                          v-model="selected.timeoutRepeat"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connTimeoutRepeatTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connTimeoutRepeatHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101','IEC60870-5-101_SERVER'].includes(
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
                        v-model="selected.sizeOfLinkAddress "
                        :label="msg.connSizeOfLinkAddress"
                      ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connSizeOfLinkAddressTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connSizeOfLinkAddressHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-101',
                        'IEC60870-5-101_SERVER'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch
                        v-model="selected.useSingleCharACK "
                        inset
                        color="primary"
                        :label="`${msg.connSingleCharAck}${selected.useSingleCharACK?msg.connSingleCharAckTrue:msg.connSingleCharAckFalse}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>

                </v-list-item-group>
              </v-list>
            </v-card>

            <v-card class="mt-6" tile
                    v-if="
                      [
                        'IEC60870-5-104',
                        'IEC60870-5-104_SERVER',
                        'DNP3',
                        'DNP3_SERVER',
                        'I104M',
                        'MODBUS',
                        'PLCTAG'
                      ].includes(selected.protocolDriver)
                    "
            >
              <v-list flat dense shaped subheader>
                <v-subheader>{{msg.connTcpParameters}}</v-subheader>
                <v-list-item-group>
                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-104_SERVER',
                        'DNP3',
                        'DNP3_SERVER',
                        'I104M',
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :rules="[rules.required, rules.ipPort]"
                          :input-value="active"
                          :label="msg.connBindIpPort"
                          hide-details="auto"
                          v-model="selected.ipAddressLocalBind"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connBindIpPortTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connBindIpPortHint}}</v-list-item-subtitle
                        >
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
                      ].includes(selected.protocolDriver)
                    "
                  >
                    <v-autocomplete
                      v-model="selected.ipAddresses"
                      :items="selected.ipAddresses"
                      chips
                      small-chips
                      deletable-chips
                      :label="msg.connRemoteIpAddresses"
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
                      <span>{{msg.connRemoteIpAddressAdd}}</span>
                    </v-tooltip>
                    <v-dialog
                      v-model="dialogAddIP"
                      max-width="290"
                      class="pa-8"
                    >
                      <v-card>
                        <v-card-title class="headline">
                          {{msg.connRemoteIpAddressAdd}}
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
                            {{msg.connRemoteIpAddressAddCancel}}
                          </v-btn>

                          <v-btn
                            color="blue darken-1"
                            text
                            @click="
                              dialogAddIP = false;
                              addNewIP($event);
                            "
                          >
                            {{msg.connRemoteIpAddressAddExecute}}
                          </v-btn>
                        </v-card-actions>
                      </v-card>
                    </v-dialog>
                  </v-list-item>

                </v-list-item-group>
              </v-list>
            </v-card>

            <v-card class="mt-6" tile
                                v-if="
                      [
                        'IEC60870-5-104',
                        'DNP3',
                      ].includes(selected.protocolDriver)
                    "
                    >
              <v-list flat dense shaped subheader>
                <v-subheader>TLS Parameters (leave blank for unencrypted connections)</v-subheader>
                <v-list-item-group>
                  <v-list-item
                    v-if="
                      ['IEC60870-5-104','DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="msg.connLocalCertificateFile"
                          hide-details="auto"
                          v-model="selected.localCertFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connLocalCertificateFileTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connLocalCertificateFileHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104','DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="msg.connPeerCertificateFile"
                          hide-details="auto"
                          v-model="selected.peerCertFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connPeerCertificateFileTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connPeerCertificateFileHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-104'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="msg.connRootCertificateFile"
                          hide-details="auto"
                          v-model="selected.rootCertFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connRootCertificateFileTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connRootCertificateFileHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="msg.connPrivateCertificateFile"
                          hide-details="auto"
                          v-model="selected.privateKeyFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connPrivateCertificateFileTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connPrivateCertificateFileHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="msg.connOpensslCypherList"
                          hide-details="auto"
                          v-model="selected.cipherList"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connOpensslCypherListTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connOpensslCypherListHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'DNP3'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch
                        v-model="selected.allowTLSv10"
                        inset
                        color="primary"
                        :label="`${msg.connAllowTls10}${selected.allowTLSv10?msg.connAllowTls10True:msg.connAllowTls10False}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'DNP3'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch
                        v-model="selected.allowTLSv11"
                        inset
                        color="primary"
                        :label="`${msg.connAllowTls11}${selected.allowTLSv11?msg.connAllowTls11True:msg.connAllowTls11False}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'DNP3'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch
                        v-model="selected.allowTLSv12"
                        inset
                        color="primary"
                        :label="`${msg.connAllowTls12}${selected.allowTLSv12?msg.connAllowTls12True:msg.connAllowTls12False}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'DNP3'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch
                        v-model="selected.allowTLSv13"
                        inset
                        color="primary"
                        :label="`${msg.connAllowTls13}${selected.allowTLSv13?msg.connAllowTls13True:msg.connAllowTls13False}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-104'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch
                        v-model="selected.allowOnlySpecificCertificates"
                        inset
                        color="primary"
                        :label="`${msg.connAllowSpecificCerts}${selected.allowOnlySpecificCertificates?msg.connAllowSpecificCertsTrue:msg.connAllowSpecificCertsFalse}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      [
                        'IEC60870-5-104'
                      ].includes(selected.protocolDriver)
                    "
                  >
                      <v-switch
                        v-model="selected.chainValidation"
                        inset
                        color="primary"
                        :label="`${msg.connCertChainValidation}${selected.chainValidation?msg.connCertChainValidationTrue:msg.connCertChainValidationFalse}`"
                        @change="updateProtocolConnection"
                      ></v-switch>
                  </v-list-item>
                </v-list-item-group>
              </v-list>
            </v-card>

            <v-card class="mt-6" tile
                    v-if="
                      [
                        'IEC60870-5-101',
                        'IEC60870-5-101_SERVER',
                        'DNP3',
                        'DNP3_SERVER',
                        'MODBUS'
                      ].includes(selected.protocolDriver)
                    "
            >
              <v-list flat dense shaped subheader>
                <v-subheader>Serial Parameters (leave blank for network connections)</v-subheader>
                <v-list-item-group>
                  <v-list-item
                    v-if="
                      ['IEC60870-5-101','IEC60870-5-101_SERVER', 'DNP3','DNP3_SERVER', 'MODBUS'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :input-value="active"
                          :label="msg.connCommPortName"
                          hide-details="auto"
                          v-model="selected.portName"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connCommPortNameTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connCommPortNameHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101','IEC60870-5-101_SERVER', 'DNP3','DNP3_SERVER', 'MODBUS'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min=150
                          :input-value="active"
                          :label="msg.connBaudRate"
                          hide-details="auto"
                          v-model="selected.baudRate"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connBaudRateTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connBaudRateHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101','IEC60870-5-101_SERVER', 'DNP3','DNP3_SERVER', 'MODBUS'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                      <v-select
                        :items="parityItems"
                        :input-value="active"
                        hide-details="auto"
                        v-model="selected.parity"
                        :label="msg.connParity"
                      ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connParityTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connParityHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101','IEC60870-5-101_SERVER', 'DNP3','DNP3_SERVER', 'MODBUS'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                      <v-select
                        :items="stopBitsItems"
                        :input-value="active"
                        hide-details="auto"
                        v-model="selected.stopBits"
                        :label="msg.connStopBits"
                      ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connStopBitsTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connStopBitsHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['IEC60870-5-101','IEC60870-5-101_SERVER', 'DNP3','DNP3_SERVER', 'MODBUS'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                      <v-select
                        :items="handshakeItems"
                        :input-value="active"
                        hide-details="auto"
                        v-model="selected.handshake"
                        :label="msg.connHandshake"
                      ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connHandshakeTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connHandshakeHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item
                    v-if="
                      ['DNP3'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min=0
                          :input-value="active"
                          :label="msg.connAsyncOpenDelay"
                          hide-details="auto"
                          v-model="selected.asyncOpenDelay"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >{{msg.connAsyncOpenDelayTitle}}</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >{{msg.connAsyncOpenDelayHint}}</v-list-item-subtitle
                        >
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
import i18n from "@/i18n/i18n-current";

export default {
  name: "ProtocolConnections",

  data: () => ({
    msg: { ...i18n },
    itemsSizeOfCOT: [1, 2],
    itemsSizeOfCA: [1, 2],
    itemsSizeOfIOA: [1, 2, 3],
    dialogAddIP: false,
    dialogAddURL: false,
    dialogAddRangeScan: false,
    dialogDelConn: false,
    newRangeScan: {group: 1, variation: 0, startAddress: 0, stopAddress: 0, period: 300},
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
    ],
    parityItems: [
      "None",
      "Even",
      "Odd",
      "Mark",
      "Space",
    ],
    stopBitsItems: [
      "One",
      "One5",
      "Two",
    ],
    handshakeItems: [
      "None",
      "Rts",
      "Xon",
      "RtsXon",
    ],
    sizeOfLinkAddressItems: [
      0,
      1,
      2,
    ],
    timeSyncModeItems: [
      0,
      1,      
      2
    ],
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
      for (let i=0; i<connDup.rangeScansStr.length; i++)
        connDup.rangeScans.push(JSON.parse(connDup.rangeScansStr[i]))
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
      if (this.newURL != "" && !this.selected.endpointURLs.includes(this.newURL)) {
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
            if ('rangeScans' in json[i])
            for (let j=0; j<json[i].rangeScans.length; j++)
              json[i].rangeScansStr.push(JSON.stringify(json[i].rangeScans[j]));
          }
          this.protocolConnections.length = 0;
          this.protocolConnections.push(...json);
        })
        .catch((err) => console.warn(err));
    },
  },
};
</script>