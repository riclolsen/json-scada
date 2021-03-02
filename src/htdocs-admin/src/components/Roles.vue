<template>
  <v-card>
    <v-row class="pa-4" justify="space-between">
      <v-col>
        <v-treeview style="max-height: 500px" class="overflow-y-auto overflow-x-hidden"
          :active.sync="active"
          :items="items"
          :load-children="fetchRoles"
          :open.sync="open"
          activatable
          color="primary"
          open-on-click
          transition
          open-all
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
          {{msg.newRole}}
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
            {{msg.selectRole}}
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
                prepend-inner-icon="mdi-security"
                type="text"
                :disabled="selected.name === 'admin' ? true : false"
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
                    v-if="selected.name !== 'admin'"
                    v-bind="attrs"
                    v-on="on"
                    class="mx-2"
                    fab
                    dark
                    x-small
                    color="red"
                    @click="dialog = true"
                  >
                    <v-icon dark> mdi-minus </v-icon>
                  </v-btn>
                </template>
                <span>{{msg.deleteRole}}</span>
              </v-tooltip>

              <v-dialog v-model="dialog" max-width="290">
                <v-card>
                  <v-card-title class="headline">{{msg.deleteRole}}</v-card-title>

                  <v-card-text>
                    {{msg.confirmDeleteRole}}
                  </v-card-text>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn color="green darken-1" text @click="dialog = false">
                      {{msg.deleteRoleCancel}}
                    </v-btn>

                    <v-btn
                      color="red darken-1"
                      text
                      @click="
                        dialog = false;
                        deleteRole($event);
                      "
                    >
                      {{msg.deleteRoleExecute}}
                    </v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>
            </v-row>

            <v-autocomplete
              v-model="selected.group1List"
              :items="group1ListAll"
              outlined
              chips
              deletable-chips
              small-chips
              :label="msg.roleCanViewGroup1List"
              multiple
              @change="roleChange"
            ></v-autocomplete>

            <v-autocomplete
              v-model="selected.group1CommandList"
              :items="group1ListAll"
              outlined
              chips
              deletable-chips
              small-chips
              :label="msg.roleCanCommandGroup1List"
              multiple
              @change="roleChange"
            ></v-autocomplete>

            <v-autocomplete
              v-model="selected.displayList"
              :items="displayListAll"
              outlined
              chips
              deletable-chips
              small-chips
              :label="msg.roleCanAccessDisplayList"
              multiple
              @change="roleChange"
            ></v-autocomplete>

            <v-card class="mx-auto" tile>
              <v-list flat dense shaped subheader>
                <v-subheader>{{msg.roleRights}}</v-subheader>

                <v-list-item-group multiple active-class="">
                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :disabled="selected.name === 'admin' ? true : false"
                          :input-value="active"
                          v-model="selected.isAdmin"
                          @change="roleChange"
                        >
                        </v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>{{msg.roleIsAdmin}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{msg.roleIsAdminHint}}</v-list-item-subtitle
                        >
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-checkbox
                          :disabled="selected.name === 'admin' ? true : false"
                          :input-value="active"
                          v-model="selected.changePassword"
                          @change="roleChange"
                        ></v-checkbox>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>{{msg.roleChangePassword}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{msg.roleChangePasswordHint}}</v-list-item-subtitle
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
                        <v-list-item-title>{{msg.roleSendCommands}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{msg.roleSendCommandsHint}}</v-list-item-subtitle
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
                        <v-list-item-title>{{msg.roleEnterAnnotations}}</v-list-item-title>
                        <v-list-item-subtitle>{{msg.roleEnterAnnotationsHint}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{msg.roleEnterNotes}}</v-list-item-title>
                        <v-list-item-subtitle>{{msg.roleEnterNotesHint}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{msg.roleEnterManuals}}</v-list-item-title>
                        <v-list-item-subtitle>{{msg.roleEnterManualsHint}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{msg.roleEnterLimits}}</v-list-item-title>
                        <v-list-item-subtitle>{{msg.roleEnterLimitsHint}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{msg.roleSubstituteValues}}</v-list-item-title>
                        <v-list-item-subtitle>{{msg.roleSubstituteValuesHint}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{msg.roleAckEvents}}</v-list-item-title>
                        <v-list-item-subtitle>{{msg.roleAckEventsHint}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{msg.roleAckAlarms}}</v-list-item-title>
                        <v-list-item-subtitle>{{msg.roleAckAlarmsHint}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{msg.roleDisableAlarms}}</v-list-item-title>
                        <v-list-item-subtitle>{{msg.roleDisableAlarmsHint}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="msg.roleNumberOfDaysSession"
                          hide-details="auto"
                          v-model="selected.maxSessionDays"
                          @change="roleChange"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>{{msg.roleMaxSessionDays}}</v-list-item-title>
                        <v-list-item-subtitle>{{msg.roleMaxSessionDaysHint}}</v-list-item-subtitle>
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
  name: "Roles",

  data: () => ({
    msg: { ...i18n },
    dialog: false,
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
          name: this.msg.roles,
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
        body: JSON.stringify({name: this.msg.newRoleName}),
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