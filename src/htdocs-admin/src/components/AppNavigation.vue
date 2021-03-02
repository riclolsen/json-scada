<template>
  <span>
       <v-navigation-drawer
      app
      v-model="drawer"
      class="primary lighten-2"
      dark
      disable-resize-watcher
    >
      <v-list>
        <template v-for="(item, index) in items">
          <v-list-item :key="index">
            <v-list-item-content>
              {{ item.title }}
            </v-list-item-content>
          </v-list-item>
          <v-divider :key="`divider-${index}`"></v-divider>
        </template>
      </v-list>
    </v-navigation-drawer>
    <v-app-bar color="primary darken-4" dark dense>
      <v-app-bar-nav-icon
        class="hidden-md-and-up"
        @click="drawer = !drawer"
      ></v-app-bar-nav-icon>
      <v-spacer class="hidden-md-and-up"></v-spacer>
      <img alt="{json-scada}" height="36px" src="../assets/json-scada.svg" />
      <v-spacer></v-spacer>
      <v-toolbar-title>{{ msg.appTitle }}</v-toolbar-title>
      <v-spacer class="hidden-sm-and-down"></v-spacer>
      <v-btn text class="hidden-sm-and-down" @click="logout($event)"
        >{{ msg.logout }} <v-icon>mdi-logout</v-icon>
      </v-btn>
    </v-app-bar>
  </span>
</template>

<script>
import i18n from "@/i18n/i18n-current";

export default {
  name: "AppNavigation",
  data() {
    return {
      msg: { ...i18n },
      items: [{ title: i18n.logout }],
      drawer: false,
    };
  },
  methods: {
    logout() {
      fetch("/Invoke/auth/signout", {
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        method: "POST",
        body: JSON.stringify({}),
      })
        .then((/*response*/) => {
          // console.log(response)
        })
        .catch((error) => {
          console.log("Error on a fetch operation: " + error.message);
        })
        .finally(() => {
          window.location = "/login/login.html";
        });
    },
  },
};
</script>

<style scoped>
</style>