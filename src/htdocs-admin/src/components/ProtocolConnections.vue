<template>
  <v-card>
    <v-row class="pa-4" justify="space-between">
      <v-col cols="5">
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
          New Connection
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
            Select a Connection
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
                label="Name"
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
                <span>Delete protocol connection!</span>
              </v-tooltip>

              <v-dialog v-model="dialogDelConn" max-width="290">
                <v-card>
                  <v-card-title class="headline">
                    Delete connection!
                  </v-card-title>

                  <v-card-text>
                    Please confirm removal of protocol connection.
                  </v-card-text>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn
                      color="green darken-1"
                      text
                      @click="dialogDelConn = false"
                    >
                      Cancel
                    </v-btn>

                    <v-btn
                      color="red darken-1"
                      text
                      @click="
                        dialogDelConn = false;
                        deleteProtocolConnection($event);
                      "
                    >
                      Delete Connection!
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
              label="Connection Number"
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
              label="Description"
              hide-details="auto"
              v-model="selected.description"
              @change="updateProtocolConnection"
            ></v-text-field>

            <v-switch
              v-model="selected.enabled"
              inset
              color="primary"
              :label="`Enabled: ${selected.enabled.toString()}`"
              @change="updateProtocolConnection"
            ></v-switch>

            <v-select
              prepend-inner-icon="mdi-cogs"
              :items="driverNameItems"
              label="Protocol driver"
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
              label="Instance Number"
              hide-details="auto"
              v-model="selected.protocolDriverInstanceNumber"
              @change="updateProtocolConnection"
            ></v-text-field>

            <v-card class="mx-auto" tile>
              <v-list flat dense shaped subheader>
                <v-subheader>Protocol Connection Parameters</v-subheader>

                <v-list-item-group multiple active-class="">


                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          label="Local link address"
                          hide-details="auto"
                          v-model="selected.localLinkAddress"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Local Link Address</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Integer number</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          min="0"
                          :input-value="active"
                          label="Remote link address"
                          hide-details="auto"
                          v-model="selected.remoteLinkAddress"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Remote Link Address</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Integer number</v-list-item-subtitle
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
                          label="GI interval"
                          hide-details="auto"
                          v-model="selected.giInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >General interrogation</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Interval in seconds</v-list-item-subtitle
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
                          label="Test command interval"
                          hide-details="auto"
                          v-model="selected.testCommandInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Test command interval</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Interval in seconds</v-list-item-subtitle
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
                          label="Time sync interval"
                          hide-details="auto"
                          v-model="selected.timeSyncInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Time sync interval</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Interval in seconds</v-list-item-subtitle
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
                          label="k"
                          hide-details="auto"
                          v-model="selected.k"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >'k' protocol parameter</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Integer number</v-list-item-subtitle
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
                          label="w"
                          hide-details="auto"
                          v-model="selected.w"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >'w' protocol parameter</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Integer number</v-list-item-subtitle
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
                          label="t0"
                          hide-details="auto"
                          v-model="selected.t0"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >'t0' protocol parameter</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In seconds</v-list-item-subtitle
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
                          label="t1"
                          hide-details="auto"
                          v-model="selected.t1"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >'t1' protocol parameter</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In seconds</v-list-item-subtitle
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
                          label="t2"
                          hide-details="auto"
                          v-model="selected.t2"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >'t2' protocol parameter</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In seconds</v-list-item-subtitle
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
                          label="t3"
                          hide-details="auto"
                          v-model="selected.t3"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >'t3' protocol parameter</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In seconds</v-list-item-subtitle
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
                        label="Size of COT"
                      ></v-select>
                      </v-list-item-action>
                                            <v-list-item-content>
                        <v-list-item-title
                          >Size Of COT</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >(Cause of Transmission)</v-list-item-subtitle
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
                        label="Size of CA"
                      ></v-select>
                      </v-list-item-action>
                                            <v-list-item-content>
                        <v-list-item-title
                          >Size Of CA</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >(Common Address)</v-list-item-subtitle
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
                          >Size Of IOA</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >(Information Object Address)</v-list-item-subtitle
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
                        :label="`One data buffer per client (serverModeMultiActive)`"
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
                          label="Max client connections"
                          hide-details="auto"
                          v-model="selected.maxClientConnections"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Max number of client connections</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Integer number</v-list-item-subtitle
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
                          label="Max queue size"
                          hide-details="auto"
                          v-model="selected.maxQueueSize"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Max size of data messages queue</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Integer number</v-list-item-subtitle
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
                          label="Class 0 scan interval"
                          hide-details="auto"
                          v-model="selected.class0ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Class 0 scan interval</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In seconds</v-list-item-subtitle
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
                          label="Class 1 scan interval"
                          hide-details="auto"
                          v-model="selected.class1ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Class 1 scan interval</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In seconds</v-list-item-subtitle
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
                          label="Class 2 scan interval"
                          hide-details="auto"
                          v-model="selected.class2ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Class 2 scan interval</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In seconds</v-list-item-subtitle
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
                          label="Class 3 scan interval"
                          hide-details="auto"
                          v-model="selected.class3ScanInterval"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Class 3 scan interval</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In seconds</v-list-item-subtitle
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
                          label="Time sync mode"
                          hide-details="auto"
                          v-model="selected.timeSyncMode"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Time sync mode</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Integer number: 0, 1 or 2</v-list-item-subtitle
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
                        :label="`Enable unsolicited`"
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
                      label="Range Scans"
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
                      <span>Add new range scan!</span>
                    </v-tooltip>
                    <v-dialog
                      v-model="dialogAddRangeScan"
                      max-width="400"
                      class="pa-8"
                    >
                      <v-card>
                        <v-card-title class="headline">
                          Add a new range scan!
                        </v-card-title>

                        <v-card-title class="headline">
                          <v-text-field
                            label="Group"
                            type="number"
                            min=1
                            v-model="newRangeScan.group"
                          ></v-text-field>

                          <v-text-field
                            label="Variation"
                            type="number"
                            min=0
                            v-model="newRangeScan.variation"
                          ></v-text-field>

                          <v-text-field
                            label="Start Address"
                            type="number"
                            min=0
                            v-model="newRangeScan.startAddress"
                          ></v-text-field>

                          <v-text-field
                            label="Stop Address"
                            type="number"
                            min=0
                            v-model="newRangeScan.stopAddress"
                          ></v-text-field>

                          <v-text-field
                            label="Period (seconds)"
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
                            Cancel
                          </v-btn>

                          <v-btn
                            color="blue darken-1"
                            text
                            @click="
                              dialogAddIP = false;
                              addNewRangeScan($event);
                            "
                          >
                            Add Range Scan!
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
                          label="Timeout for ACK"
                          hide-details="auto"
                          v-model="selected.timeoutForACK"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Timeout for ack</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In milliseconds</v-list-item-subtitle
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
                          label="Timeout for repeat"
                          hide-details="auto"
                          v-model="selected.timeoutRepeat"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Timeout for repeat</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In milliseconds</v-list-item-subtitle
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
                        label="Size of link address"
                      ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Size of Link Address</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >0, 1, 2</v-list-item-subtitle
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
                        :label="`Use single char ACK`"
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
                <v-subheader>TCP Parameters (leave blank for serial connections)</v-subheader>
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
                          label="Bind IP address and port"
                          hide-details="auto"
                          v-model="selected.ipAddressLocalBind"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title
                          >Bind IP Address and Port</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >Local bind for listening</v-list-item-subtitle
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
                      label="Remote IP addresses"
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
                      <span>Add new IP Address!</span>
                    </v-tooltip>
                    <v-dialog
                      v-model="dialogAddIP"
                      max-width="290"
                      class="pa-8"
                    >
                      <v-card>
                        <v-card-title class="headline">
                          Add a new IP Address!
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
                            Cancel
                          </v-btn>

                          <v-btn
                            color="blue darken-1"
                            text
                            @click="
                              dialogAddIP = false;
                              addNewIP($event);
                            "
                          >
                            Add IP!
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
                          label="Local certificate file path"
                          hide-details="auto"
                          v-model="selected.localCertFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Local certificate file path [TLS]</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. 'C:\json-scada\conf\localCert.pfx'</v-list-item-subtitle
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
                          label="Peer certificate file path"
                          hide-details="auto"
                          v-model="selected.peerCertFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Peer certificate file path [TLS]</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. 'C:\json-scada\conf\peerCert.cer'</v-list-item-subtitle
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
                          label="Root certificate file path"
                          hide-details="auto"
                          v-model="selected.rootCertFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Root certificate file path [TLS]</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. 'C:\json-scada\conf\rootCert.cer'</v-list-item-subtitle
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
                          label="Private key file path"
                          hide-details="auto"
                          v-model="selected.privateKeyFilePath"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Private key file path [TLS]</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. 'C:\json-scada\conf\deviceKey.pem'</v-list-item-subtitle
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
                          label="Openssl Cypher list"
                          hide-details="auto"
                          v-model="selected.cipherList"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Openssl format cipher list [TLS]</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. 'AES128, AES256, AES, DES'</v-list-item-subtitle
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
                        :label="`Allow TLS version 1.0`"
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
                        :label="`Allow TLS version 1.1`"
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
                        :label="`Allow TLS version 1.2`"
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
                        :label="`Allow TLS version 1.3`"
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
                        :label="`Allow only specific certificates [TLS]`"
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
                        :label="`Certificate chain validation [TLS]`"
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
                          label="Comm port name"
                          hide-details="auto"
                          v-model="selected.portName"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Serial port name or IP:address</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. "COM3", "/dev/ttyS0", "192.168.0.1:2410"</v-list-item-subtitle
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
                          label="Baud rate"
                          hide-details="auto"
                          v-model="selected.baudRate"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Serial baud rate (bps)</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. "9600", "19200"</v-list-item-subtitle
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
                        label="Parity"
                      ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Serial parity</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. None, Even, Odd, ...</v-list-item-subtitle
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
                        label="Stop bits"
                      ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Serial stop bits</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. One, One5, Two</v-list-item-subtitle
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
                        label="Handshake"
                      ></v-select>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Type of serial handshake</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >e.g. None, Xon, Rts, ...</v-list-item-subtitle
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
                          label="Async open delay"
                          hide-details="auto"
                          v-model="selected.asyncOpenDelay"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>
                      <v-list-item-content>
                        <v-list-item-title
                          >Async open delay (serial)</v-list-item-title
                        >
                        <v-list-item-subtitle
                          >In milliseconds</v-list-item-subtitle
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
// const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

export default {
  name: "ProtocolConnections",

  data: () => ({
    itemsSizeOfCOT: [1, 2],
    itemsSizeOfCA: [1, 2],
    itemsSizeOfIOA: [1, 2, 3],
    dialogAddIP: false,
    dialogAddRangeScan: false,
    dialogDelConn: false,
    newRangeScan: {group: 1, variation: 0, startAddress: 0, stopAddress: 0, period: 300},
    newIP: "",
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
          this.fetchProtocolConnections(); // refreshes driver instances
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
          this.fetchProtocolConnections(); // refreshes roles
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
          this.fetchProtocolConnections(); // refreshes users
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