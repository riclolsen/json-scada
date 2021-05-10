<template>
  <span>
    <v-app-bar color="primary darken-4" dark dense class="d-print-none">
      <img alt="{json-scada}" height="36px" src="../assets/json-scada.svg" />
      <v-spacer></v-spacer>
      <v-toolbar-title>{{ $t('src\\components\\appnavigation.appTitle') }}</v-toolbar-title>
      <v-spacer ></v-spacer>
      <v-btn text @click="logout($event)"
        >{{ $t('src\\components\\appnavigation.logout') }} <v-icon>mdi-logout</v-icon>
      </v-btn>      
      <img alt="" height="16px" src="../assets/admin.png" />
    </v-app-bar>

    <LanguageSwitcher />

  </span>
</template>

<script>
import i18n from "../i18n.js";
import LanguageSwitcher from "@/components/LanguageSwitcher.vue";

export default {
  name: "AppNavigation",
    components: {
    LanguageSwitcher
  },
  data() {
    return {
      items: [{ title: i18n.t('src\\components\\appnavigation.appTitle') }],
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