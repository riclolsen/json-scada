<template>
  <v-card>
    <v-tabs v-model="tab" background-color="primary accent-8" dark icons-and-text>
      <v-tabs-slider></v-tabs-slider>

      <v-tab href="#tab-1">
        USERS
        <v-icon>mdi-account-multiple</v-icon>
      </v-tab>

      <v-tab href="#tab-2">
        ROLES
        <v-icon>mdi-security</v-icon>
      </v-tab>

      <v-tab href="#tab-3">
        DRIVER INSTANCES
        <v-icon>mdi-cogs</v-icon>
      </v-tab>
      <v-tab href="#tab-4">
        CONNECTIONS
        <v-icon>mdi-file-tree</v-icon>
      </v-tab>
      <v-tab href="#tab-5">
        TAGS
        <v-icon>mdi-tag-multiple</v-icon>
      </v-tab>

      <v-tab-item :value="'tab-' + 1">
        <Users />
      </v-tab-item>

      <v-tab-item :value="'tab-' + 2">
        <Roles />
      </v-tab-item>

      <v-tab-item :value="'tab-' + 3">
        <ProtocolDriverInstances />
      </v-tab-item>

      <v-tab-item :value="'tab-' + 4">
        <ProtocolConnections />
      </v-tab-item>

      <v-tab-item :value="'tab-' + 5">
        <Tags />
      </v-tab-item>

    </v-tabs>

    <!--
    <v-tabs-items v-model="tab">
      <v-tab-item
        v-for="i in 4"
        :key="i"
        :value="'tab-' + i"
      >
        <v-card flat>
          <v-card-text>{{ text }}</v-card-text>
        </v-card>
      </v-tab-item>
    </v-tabs-items>
-->
  </v-card>
</template>


<script>
import Users from "@/components/Users";
import Roles from "@/components/Roles";
import ProtocolDriverInstances from "@/components/ProtocolDriverInstances";
import ProtocolConnections from "@/components/ProtocolConnections";
import Tags from "@/components/Tags";

export default {
  name: "TabbedOptions",
  data() {
    return {
      tab: null,
      text:
        "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.",
    };
  },
  components: {
    Users,
    Roles,
    ProtocolDriverInstances,
    ProtocolConnections,
    Tags
  },
};
</script>