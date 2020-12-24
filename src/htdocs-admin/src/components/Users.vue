<template>
  <v-card>
    <v-row class="pa-4" justify="space-between">
      <v-col cols="5">
        <v-treeview style="max-height: 500px" class="overflow-y-auto overflow-x-hidden"
          :active.sync="active"
          :items="items"
          :load-children="fetchUsers"
          :open.sync="open"
          activatable
          color="primary"
          open-on-click
          transition
        >
          <template v-slot:prepend="{ item }">
            <v-icon v-if="!item.children"> mdi-account </v-icon>
            {{ item.username }}
          </template>
        </v-treeview>
            <v-btn              
              class="mt-6"
              dark
              x-small
              color="blue"
              @click="createUser($event)"
            >
              <v-icon dark> mdi-plus </v-icon>
              New User
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
            Select a User
          </div>
          <v-card
            v-else
            :key="selected.id"
            class="pt-6 mx-auto"
            flat
            max-width="400"
          >
            <v-row class="pb-8 mx-auto" justify="space-between">
              <v-text-field
                prepend-inner-icon="mdi-account"
                :disabled="selected.username==='admin' ? true : false"
                type="text"
                outlined
                clearable
                :input-value="active"
                label="User name"
                hide-details="auto"
                v-model="selected.username"
                @change="updateUser"
              ></v-text-field>
              
              <v-tooltip bottom>
                <template v-slot:activator="{ on, attrs }">
                  <v-btn
                    v-if="selected.username !== 'admin'"
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
                <span>Delete user!</span>
              </v-tooltip>

              <v-dialog v-model="dialog" max-width="290">
                <v-card>
                  <v-card-title class="headline"> Delete User! </v-card-title>

                  <v-card-text>
                    Please confirm removal of user.
                  </v-card-text>

                  <v-card-actions>
                    <v-spacer></v-spacer>

                    <v-btn color="green darken-1" text @click="dialog = false">
                      Cancel
                    </v-btn>

                    <v-btn
                      color="red darken-1"
                      text
                      @click="
                        dialog = false;
                        deleteUser($event);
                      "
                    >
                      Delete User!
                    </v-btn>
                  </v-card-actions>
                </v-card>
              </v-dialog>



            </v-row>

            <v-text-field
              class="pb-8"
              prepend-inner-icon="mdi-email"
              type="text"
              outlined
              clearable
              :input-value="active"
              label="email"
              hide-details="auto"
              v-model="selected.email"
              @change="updateUser"
            ></v-text-field>

            <v-text-field
              prepend-inner-icon="mdi-account-key"
              type="password"
              outlined
              clearable
              :input-value="active"
              label="password"
              hide-details="auto"
              v-model="selected.password"
              @change="updateUser"
            ></v-text-field>

            <v-card class="my-4" tile>
              <v-card-text>
                <v-icon x-large color="primary darken-2">mdi-security</v-icon>
                <h3 class="headline mb-2">ROLES</h3>
                <v-menu :load-children="fetchRoles">
                  <template v-slot:activator="{ on: menu, attrs }">
                    <v-tooltip bottom>
                      <template v-slot:activator="{ on: tooltip }">
                        <v-btn
                          color="primary"
                          fab
                          dark
                          x-small
                          v-bind="attrs"
                          v-on="{ ...tooltip, ...menu }"
                          @click="fetchRoles()"
                        >
                          <v-icon dark> mdi-plus </v-icon>
                        </v-btn>
                      </template>
                      <span>Add Role</span>
                    </v-tooltip>
                  </template>
                  <v-list>
                    <v-list-item
                      v-for="(item, index) in roles"
                      :key="index"
                      @click="addRoleToUser($event, item.name)"
                    >
                      <v-list-item-title>{{ item.name }}</v-list-item-title>
                    </v-list-item>
                  </v-list>
                </v-menu>
              </v-card-text>
              <v-card class="mx-auto" tile>
                <v-list nav dense>
                  <v-list-item-group color="primary">
                    <v-list-item v-for="(item, i) in selected.roles" :key="i">
                      <v-list-item-icon>
                        <v-icon>mdi-security</v-icon>
                      </v-list-item-icon>
                      <v-list-item-content>
                        <v-list-item-title
                          class="primary--text subheading font-weight-bold"
                          v-text="item.name"
                        ></v-list-item-title>
                      </v-list-item-content>
                      <v-list-item-action>
                        <v-tooltip bottom>
                          <template v-slot:activator="{ on, attrs }">
                            <v-btn
                              :disabled="(selected.username==='admin' && item.name==='admin')"
                              v-bind="attrs"
                              v-on="on"
                              class="mx-2"
                              fab
                              dark
                              x-small
                              color="red"
                              @click="removeRoleFromUser($event, item.name)"
                            >
                              <v-icon dark> mdi-minus </v-icon>
                            </v-btn>
                          </template>
                          <span>Remove role!</span>
                        </v-tooltip>
                      </v-list-item-action>
                    </v-list-item>
                  </v-list-item-group>
                </v-list>
              </v-card>
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
  name: "Users",

  data: () => ({
    dialog: false,
    active: [],
    open: [],
    users: [],
    roles: [],
  }),

  computed: {
    items() {
      return [
        {
          name: "Users",
          children: this.users,
          roles: this.roles,
        },
      ];
    },
    selected() {
      if (!this.active.length) return undefined;

      const id = this.active[0];

      return this.users.find((user) => user.id === id);
    },
  },

  watch: {
    // selected: "randomAvatar",
  },

  methods: {
    async addRoleToUser(evt, roleName) {
      if (this.selected.roles.some(e => e.name === roleName)) 
        return
 
      if (this.selected.roles.includes(roleName))
        return
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
    async removeRoleFromUser(evt, roleName) {
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
    async fetchUsers() {
      return await fetch("/Invoke/auth/listUsers")
        .then((res) => res.json())
        .then((json) => {
          for (let i = 0; i < json.length; i++) {
            json[i].id = i + 1;
            // json[i].name = json[i].username;
          }
          this.users.length = 0;
          this.users.push(...json);
        })
        .catch((err) => console.warn(err));
    },
    async fetchRoles() {
      return await fetch("/Invoke/auth/listRoles")
        .then((res) => res.json())
        .then((json) => {
          this.roles.length = 0;
          this.roles.push(...json);
        })
        .catch((err) => console.warn(err));
    },
    async deleteUser() {
      return await fetch("/Invoke/auth/deleteUser", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username: this.selected.username,
          _id: this.selected._id,
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
    },
    async updateUser() {
      var userDup = Object.assign({}, this.selected);
      delete userDup["id"];
      if ("password" in userDup)
        if (userDup.password === "" || userDup.password === null)
          delete userDup["password"];
      this.selected.password = "";
      return await fetch("/Invoke/auth/updateUser", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify(userDup),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
    },
    async createUser() {
      return await fetch("/Invoke/auth/createUser", {
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
          this.fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
    },
  },
};
</script>