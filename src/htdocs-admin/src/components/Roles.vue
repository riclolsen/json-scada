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
          {{$t("src\\components\\roles.newRole")}}
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
            {{$t("src\\components\\roles.selectRole")}}
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
                :label="$t('src\\components\\roles.roleName')"
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
                <span>{{$t("src\\components\\roles.deleteRole")}}</span>
              </v-tooltip>

              <v-dialog v-model="dialog" max-width="290">
                <v-card>
                  <v-card-title class="headline">{{$t("src\\components\\roles.deleteRole")}}</v-card-title>

                  <v-card-text>
                    {{$t("src\\components\\roles.confirmDeleteRole")}}
                  </v-card-text>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn color="green darken-1" text @click="dialog = false">
                      {{$t("src\\components\\roles.deleteRoleCancel")}}
                    </v-btn>

                    <v-btn
                      color="red darken-1"
                      text
                      @click="
                        dialog = false;
                        deleteRole($event);
                      "
                    >
                      {{$t("src\\components\\roles.deleteRoleExecute")}}
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
              :label="$t('src\\components\\roles.canViewGroup1List')"
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
              :label="$t('src\\components\\roles.canCommandGroup1List')"
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
              :label="$t('src\\components\\roles.canAccessDisplayList')"
              multiple
              @change="roleChange"
            ></v-autocomplete>

            <v-card class="mx-auto" tile>
              <v-list flat dense shaped subheader>
                <v-subheader>{{$t("src\\components\\roles.rights")}}</v-subheader>

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
                        <v-list-item-title>{{$t("src\\components\\roles.isAdmin")}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{$t("src\\components\\roles.isAdminHint")}}</v-list-item-subtitle
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
                        <v-list-item-title>{{$t("src\\components\\roles.changePassword")}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{$t("src\\components\\roles.changePasswordHint")}}</v-list-item-subtitle
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
                        <v-list-item-title>{{$t("src\\components\\roles.sendCommands")}}</v-list-item-title>
                        <v-list-item-subtitle
                          >{{$t("src\\components\\roles.sendCommandsHint")}}</v-list-item-subtitle
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
                        <v-list-item-title>{{$t("src\\components\\roles.enterAnnotations")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\roles.enterAnnotationsHint")}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{$t("src\\components\\roles.enterNotes")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\roles.enterNotesHint")}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{$t("src\\components\\roles.enterManuals")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\roles.enterManualsHint")}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{$t("src\\components\\roles.enterLimits")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\roles.enterLimitsHint")}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{$t("src\\components\\roles.substituteValues")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\roles.substituteValuesHint")}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{$t("src\\components\\roles.ackEvents")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\roles.ackEventsHint")}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{$t("src\\components\\roles.ackAlarms")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\roles.ackAlarmsHint")}}</v-list-item-subtitle>
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
                        <v-list-item-title>{{$t("src\\components\\roles.disableAlarms")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\roles.disableAlarmsHint")}}</v-list-item-subtitle>
                      </v-list-item-content>
                    </template>
                  </v-list-item>

                  <v-list-item>
                    <template v-slot:default="{ active }">
                      <v-list-item-action>
                        <v-text-field
                          type="number"
                          :input-value="active"
                          :label="$t('src\\components\\roles.numberOfDaysSession')"
                          hide-details="auto"
                          v-model="selected.maxSessionDays"
                          @change="roleChange"
                        ></v-text-field>
                      </v-list-item-action>

                      <v-list-item-content>
                        <v-list-item-title>{{$t("src\\components\\roles.maxSessionDays")}}</v-list-item-title>
                        <v-list-item-subtitle>{{$t("src\\components\\roles.maxSessionDaysHint")}}</v-list-item-subtitle>
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
import i18n from "../i18n.js";

export default {
  name: "Roles",

  data: () => ({
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
          name: i18n.t("src\\components\\roles.roles"),
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
        body: JSON.stringify({name: i18n.t("src\\components\\roles.newRoleName")}),
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