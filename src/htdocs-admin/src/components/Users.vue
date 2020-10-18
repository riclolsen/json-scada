<template>
  <v-card>
    <v-row class="pa-4" justify="space-between">
      <v-col cols="5">
        <v-treeview
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
          </template>
        </v-treeview>
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
            <v-card-text>
              <v-icon x-large color="primary darken-2"> mdi-account </v-icon>
              <h3 class="primary--text headline mb-2">
                {{ selected.name }}
              </h3>
              <div class="primary--text mb-2">
                {{ selected.email }}
              </div>
              <div class="primary--text subheading font-weight-bold">
                {{ selected.username }}
              </div>
            </v-card-text>
            <v-divider></v-divider>

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
                  @click="addRole($event, item.name)"
                >
                  <v-list-item-title>{{ item.name }}</v-list-item-title>
                </v-list-item>
              </v-list>
            </v-menu>
            </v-card-text>
            <v-card class="mx-auto" max-width="400" tile>
              <v-list nav dense>
                <v-list-item-group color="primary">
                  <v-list-item v-for="(item, i) in selected.roles" :key="i">
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
                            v-bind="attrs"
                            v-on="on"
                            class="mx-2"
                            fab
                            dark
                            x-small
                            color="red"
                            @click="removeRole($event, item.name)"
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
    async fetchUsers() {
      return await fetch("/Invoke/auth/listUsers")
        .then((res) => res.json())
        .then((json) => {
          for (let i = 0; i < json.length; i++) {
            json[i].id = i + 1;
            json[i].name = json[i].username;
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
  },
};
</script>