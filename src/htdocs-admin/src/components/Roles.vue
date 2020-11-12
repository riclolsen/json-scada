<template>
  <v-card>
    <v-row class="pa-4" justify="space-between">
      <v-col cols="5">
        <v-treeview
          :active.sync="active"
          :items="items"
          :load-children="fetchRoles"
          :open.sync="open"
          activatable
          color="primary"
          open-on-click
          transition
        >
          <template v-slot:prepend="{ item }">
            <v-icon v-if="!item.children"> mdi-security </v-icon>
          </template>
        </v-treeview>
          <v-btn
            class="mt-6"
            dark
            x-small
            color="blue"
            @click="createRole($event)"
          >
            <v-icon dark> mdi-plus </v-icon>
            New Role
          </v-btn>
      </v-col>

      <v-divider vertical></v-divider>
      <v-col class="d-flex text-left">
        <v-scroll-y-transition mode="out-in">
          <div
            v-if="!selected"
            class="title grey--text text--lighten-1 font-weight-light text-center"
            style="align-self: center"
          >
            Select a Role
          </div>
          <v-card v-else :key="selected.id" class="pt-6 mx-auto" flat max-width="600">

              <v-row class="pb-8 mx-auto" justify="space-between">  
              <v-text-field
                prepend-inner-icon="mdi-security"
                type="text"
                :disabled="selected.name==='admin' ? true : false"
                outlined
                clearable
                :input-value="active"
                label="Role name"
                hide-details="auto"
                v-model="selected.name"
                @change="roleChange"
              ></v-text-field>
              <v-tooltip bottom>
                <template v-slot:activator="{ on, attrs }">
                  <v-btn
                    v-if="!(selected.name==='admin')"
                    v-bind="attrs"
                    v-on="on"
                    class="mx-2"
                    fab
                    dark
                    x-small
                    color="red"
                    @click="deleteRole($event)"
                  >
                    <v-icon dark> mdi-minus </v-icon>
                  </v-btn>
                </template>
                <span>Delete role!</span>
              </v-tooltip>
              </v-row>

            <v-autocomplete
              v-model="selected.group1List"
              :items="group1ListAll"
              outlined
              dense
              chips
              small-chips
              label="Can View - Group1 List"
              multiple
              @change="roleChange"
            ></v-autocomplete>

            <v-autocomplete
              v-model="selected.group1CommandList"
              :items="group1ListAll"
              outlined
              dense
              chips
              small-chips
              label="Can Command - Group1 List"
              multiple
              @change="roleChange"
            ></v-autocomplete>

            <v-autocomplete
              v-model="selected.displayList"
              :items="displayListAll"
              outlined
              dense
              chips
              small-chips
              label="Display List"
              multiple
              @change="roleChange"
            ></v-autocomplete>

            <v-card class="mx-auto" tile>
              <v-list flat dense shaped subheader>
                <v-subheader>Rights</v-subheader>

                <v-list-item-group multiple active-class="">
                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :disabled="selected.name==='admin' ? true : false"
                          :input-value="active"
                          v-model="selected.isAdmin"
                          @change="roleChange"
                        >
                        </v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Is Admin</v-list-item-title>
                        <v-list-item-subtitle
                          >Add/remove/edit users and
                          roles.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :disabled="selected.name==='admin' ? true : false"
                          :input-value="active"
                          v-model="selected.changePassword"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Change Password</v-list-item-title>
                        <v-list-item-subtitle
                          >Can change its own password.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :input-value="active"
                          v-model="selected.sendCommands"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Send Commands</v-list-item-title>
                        <v-list-item-subtitle
                          >Can send commands (controls).</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :input-value="active"
                          v-model="selected.enterAnnotations"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Enter Annotations</v-list-item-title>
                        <v-list-item-subtitle
                          >Can create/edit blocking
                          annotations.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :input-value="active"
                          v-model="selected.enterNotes"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Enter Notes</v-list-item-title>
                        <v-list-item-subtitle
                          >Can edit/create documental
                          notes.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :input-value="active"
                          v-model="selected.enterManuals"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Enter Manuals</v-list-item-title>
                        <v-list-item-subtitle
                          >Can change state of manual
                          points.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :input-value="active"
                          v-model="selected.enterLimits"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Enter Limits</v-list-item-title>
                        <v-list-item-subtitle
                          >Can change limits for analog
                          points.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :input-value="active"
                          v-model="selected.substituteValues"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Substitute Values</v-list-item-title>
                        <v-list-item-subtitle
                          >Can replace (impose) supervised
                          values.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :input-value="active"
                          v-model="selected.ackEvents"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Ack Events</v-list-item-title>
                        <v-list-item-subtitle
                          >Can acknowledge events.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :input-value="active"
                          v-model="selected.ackAlarms"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Ack Alarms</v-list-item-title>
                        <v-list-item-subtitle
                          >Can acknowledge alarms.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :input-value="active"
                          v-model="selected.disableAlarms"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Disable Alarms</v-list-item-title>
                        <v-list-item-subtitle
                          >Can disable/enable alarms.</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          label="Number of days..."
                          hide-details="auto"
                          v-model="selected.maxSessionDays"
                          @change="roleChange"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>Max Session Days</v-list-item-title>
                        <v-list-item-subtitle
                          >Maximum days of session period.</v-list-item-subtitle
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
  name: "Roles",

  data: () => ({
    active: [],
    open: [],
    roles: [],
    settings: [],
    displayListAll: [],
    group1ListAll: [],
    group1ListCommandAll: [],
  }),

  computed: {
    items() {
      return [
        {
          name: "Roles",
          children: this.roles,
          roles: this.roles,
        },
      ];
    },
    selected() {
      if (!this.active.length) return undefined;

      const id = this.active[0];

      return this.roles.find((roles) => roles.id === id);
    },
  },

  watch: {
    // selected: "randomAvatar",
  },

  methods: {
    async addRole(evt, roleName) {
      console.log(roleName);

      return await fetch("/Invoke/auth/userAddRole", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username: this.selected.username,
          role: roleName,
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
    },
    async removeRole(evt, roleName) {
      console.log(roleName);

      return await fetch("/Invoke/auth/userRemoveRole", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username: this.selected.username,
          role: roleName,
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
    },
    async fetchRoles() {
      return await fetch("/Invoke/auth/listRoles")
        .then((res) => res.json())
        .then((json) => {
          for (let i = 0; i < json.length; i++) {
            json[i].id = i + 1;
          }
          this.roles.length = 0;
          this.roles.push(...json);
          this.fetchGroup1List();
          this.fetchDisplayList();
        })
        .catch((err) => console.warn(err));
    },
    async fetchGroup1List() {
      return await fetch("/Invoke/auth/listGroup1")
        .then((res) => res.json())
        .then((json) => {
          this.group1ListAll.length = 0;
          this.group1ListAll.push(...json);
        })
        .catch((err) => console.warn(err));
    },
    async fetchDisplayList() {
      return await fetch("/Invoke/auth/listDisplays")
        .then((res) => res.json())
        .then((json) => {
          this.displayListAll.length = 0;
          this.displayListAll.push(...json);
        })
        .catch((err) => console.warn(err));
    },
    async roleChange() {
      var roleDup = Object.assign({}, this.selected);
      delete roleDup["id"];
      return await fetch("/Invoke/auth/updateRole", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify(roleDup),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchRoles(); // refreshes roles
        })
        .catch((err) => console.warn(err));
    },
    async deleteRole() {
      return await fetch("/Invoke/auth/deleteRole", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          name: this.selected.name,
          _id: this.selected._id,
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchRoles(); // refreshes roles
        })
        .catch((err) => console.warn(err));
    },
    async createRole() {
      return await fetch("/Invoke/auth/createRole", {
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
          this.fetchRoles(); // refreshes roles          
        })
        .catch((err) => console.warn(err));
    },
  },
};
</script>