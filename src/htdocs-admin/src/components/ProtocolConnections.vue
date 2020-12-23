<template>
  <v-card>
    <v-row class="pa-4" justify="space-between">
      <v-col cols="5">
        <v-treeview
          style="max-height: 500px"
          class="overflow-y-auto overflow-x-hidden"
          :active.sync="active"
          :items="items"
          :load-children="fetchProtocolConnections"
          :open.sync="open"
          activatable
          color="primary"
          open-on-click
          transition
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

      <v-col class="d-flex text-center">
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
            max-width="440"
          >
            <v-row class="pb-8 mx-auto" justify="space-between">
              <v-text-field
                prepend-inner-icon="mdi-play-circle"
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
                    @click="dialogDelInst = true"
                  >
                    <v-icon dark> mdi-minus </v-icon>
                  </v-btn>
                </template>
                <span>Delete protocol connection!</span>
              </v-tooltip>

              <v-dialog v-model="dialogDelInst" max-width="290">
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
                      @click="dialogDelInst = false"
                    >
                      Cancel
                    </v-btn>

                    <v-btn
                      color="red darken-1"
                      text
                      @click="
                        dialogDelInst = false;
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
              prepend-inner-icon="mdi-play-circle"
              type="number"
              outlined
              min="1"
              clearable
              :input-value="active"
              label="Connection Number"
              hide-details="auto"
              v-model="selected.protocolConnectionNumber"
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
                <v-subheader>Connection Parameters</v-subheader>

                <v-list-item-group multiple active-class="">
                  <v-list-item
                    v-if="
                      ['IEC60870-5-104_SERVER', 'DNP3_SERVER'].includes(
                        selected.protocolDriver
                      )
                    "
                  >
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="text"
                          :rules="[rules.required, rules.ip]"
                          :input-value="active"
                          label="Bind IP Address"
                          hide-details="auto"
                          v-model="selected.ipAddressLocalBind"
                          @change="updateProtocolConnection"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Bind IP Address</v-list-item-title>
                        <v-list-item-subtitle
                          >Local bind for listening</v-list-item-subtitle
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
    dialogDelInst: false,
    active: [],
    open: [],
    rules: {
      required: (value) => !!value || "Required.",
      counter: (value) => value.length <= 20 || "Max 20 characters",
      ip: (value) => {
        const pattern = /\b((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4}\b/;
        return pattern.test(value) || "Invalid IP Address.";
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
    async fetchNodes() {
      return await fetch("/Invoke/auth/listNodes")
        .then((res) => res.json())
        .then((json) => {
          this.nodeNames = json;
        })
        .catch((err) => console.warn(err));
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
      this.fetchNodes();
      return await fetch("/Invoke/auth/listProtocolConnections")
        .then((res) => res.json())
        .then((json) => {
          for (let i = 0; i < json.length; i++) {
            json[i].id = i + 1;
          }
          this.protocolConnections.length = 0;
          this.protocolConnections.push(...json);
        })
        .catch((err) => console.warn(err));
    },
  },
};
</script>