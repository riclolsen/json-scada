<template>
  <v-card tile flat class="pa-0">
    <v-tabs
      v-model="tab"
      background-color="primary accent-8"
      dark
      icons-and-text
      height="62px"
    >
      <v-tabs-slider></v-tabs-slider>

      <v-tab href="#tab-1">
        {{ $t('src\\components\\users.users') }}
        <v-icon>mdi-account-multiple</v-icon>
      </v-tab>

      <v-tab href="#tab-2">
        {{ $t('src\\components\\roles.roles') }}
        <v-icon>mdi-security</v-icon>
      </v-tab>

      <v-tab href="#tab-3">
        {{ $t('src\\components\\driverInstances.driverInstances') }}
        <v-icon>mdi-cogs</v-icon>
      </v-tab>

      <v-tab href="#tab-4">
        {{ $t('src\\components\\connections.connections') }}
        <v-icon>mdi-file-tree</v-icon>
      </v-tab>

      <v-tab href="#tab-5">
        {{ $t('src\\components\\tags.tags') }}
        <v-icon>mdi-tag-multiple</v-icon>
      </v-tab>

      <v-tab href="#tab-6">
        {{ $t('src\\components\\useractions.userActions') }}
        <v-icon>mdi-clipboard-account</v-icon>
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

      <v-tab-item :value="'tab-' + 6">
        <UserActions />
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
import UserActions from "@/components/UserActions";

export default {
  name: "TabbedOptions",
  data() {
    return {
      tab: null,
      text:
        "",
    };
  },
  components: {
    Users,
    Roles,
    ProtocolDriverInstances,
    ProtocolConnections,
    Tags,
    UserActions
  },
};
</script>