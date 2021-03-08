<template>
  <div>
    <v-data-table
      :headers="headers"
      :items="userActions"
      :options.sync="options"
      :server-items-length="totalUserActions"
      :loading="loading"
      multi-sort
      class="mi-dataTable elevation-1"
      dense
      :items-per-page="15"
    >
      <template v-slot:top>
        <v-toolbar flat>
          <v-toolbar-title>{{ msg.userActions }}</v-toolbar-title>
          <v-divider class="mx-4" inset vertical></v-divider>
          <v-text-field
            dense
            @change="fetchUserActions"
            append-icon="mdi-magnify"
            v-model="searchUsername"
            :label="msg.actionSearchUsername"
            hide-details
          ></v-text-field>
          <v-spacer></v-spacer>

          <v-text-field
            dense
            @change="fetchUserActions"
            append-icon="mdi-magnify"
            v-model="searchAction"
            :label="msg.actionSearchAction"
            hide-details
          ></v-text-field>
          <v-spacer></v-spacer>

          <v-menu
            ref="menu"
            v-model="menu"
            :close-on-content-click="false"
            :return-value.sync="date"
            transition="scale-transition"
            offset-y
            min-width="auto"
          >
            <template v-slot:activator="{ on, attrs }">
              <v-text-field
                class="mt-4 mx-auto"
                v-model="dateRangeText"
                :label="msg.actionSearchDateRange"
                prepend-icon="mdi-calendar"
                readonly
                v-bind="attrs"
                v-on="on"
              ></v-text-field>
            </template>

            <v-date-picker
              v-model="dates"
              no-title
              scrollable
              show-current
              show-adjacent-months
              range
              @change="fetchUserActions"
            >
            </v-date-picker>
          </v-menu>
        </v-toolbar>
      </template>
    </v-data-table>
  </div>
</template>

<style>
table {
  table-layout: fixed;
}
td {
  vertical-align: top;
}
</style>
<script>
import i18n from "@/i18n/i18n-current";

export default {
  name: "UserActions",

  data: () => ({
    msg: { ...i18n },
    menu: false,
    active: [],
    open: [],
    userActions: [],
    searchUsername: "",
    searchTag: "",
    searchAction: "",
    totalUserActions: 0,
    date: false,
    dates: [],
    searchDescription: "",

    loading: true,
    options: {},
    headers: [
      // { text: "_id", sortable: true, value: "_id", width: "15%" },
      { text: "Username", sortable: true, value: "username", width: "8%" },
      { text: "Action", sortable: true, value: "action", width: "8%" },
      { text: "Tag", sortable: true, value: "tag", width: "8%" },
      { text: "Point Key", sortable: true, value: "pointKey", width: "8%" },
      {
        text: "Properties",
        sortable: false,
        value: "propertiesString",
        width: "35%",
      },
      { text: "Time", sortable: true, value: "timeTag", width: "15%" },
    ],
    editedIndex: -1,
    editedItem: {
      tag: "",
      description: "",
      group1: "",
    },
    defaultItem: {
      tag: "",
      description: "",
      group1: "",
    },
  }),

  computed: {
    dateRangeText() {
      if (this.dates.length === 1) this.fetchUserActions();
      return this.dates.join(" --> ");
    },
    items() {
      return [
        {
          name: "User Actions",
          children: this.userActions,
        },
      ];
    },
    selected() {
      if (!this.active.length) return undefined;

      // const id = this.active[0];

      return this.userActions.find((userAction) => userAction.tag === "");
    },
  },

  watch: {
    options: {
      handler() {
        this.fetchUserActions();
      },
      deep: true,
    },
  },

  mounted() {
    this.fetchUserActions();
  },

  methods: {
    async fetchUserActions() {
      const { sortBy, sortDesc, page, itemsPerPage } = this.options;
      let filter = {};

      if (this.searchUsername.trim() != "")
        filter.username = { $regex: this.searchUsername, $options: "i" };
      if (this.searchAction.trim() != "")
        filter.action = { $regex: this.searchAction, $options: "i" };

      let dts = [];
      this.dates.map(function (date) {
        dts.push(new Date(date));
      });

      function addDays(date, days) {
        var result = new Date(date);
        result.setDate(result.getDate() + days);
        return result;
      }

      if (dts.length > 1)
        if (dts[1] < dts[0]) {
          let d = dts[0];
          dts[0] = dts[1];
          dts[1] = d;
        }

      if (dts.length > 0)
        filter.timeTag = { $gte: dts[0], $lt: addDays(dts[0], 1) };
      if (dts.length > 1)
        filter.timeTag = { $gte: dts[0], $lt: addDays(dts[1], 1) };

      this.loading = true;
      return await fetch("/Invoke/auth/listUserActions", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          itemsPerPage: itemsPerPage,
          sortBy: sortBy,
          sortDesc: sortDesc,
          page: page,
          filter: filter,
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          for (let i = 0; i < json.userActions.length; i++) {
            // format object 'properties' as a string for display
            if ("properties" in json.userActions[i])
              json.userActions[i].propertiesString = JSON.stringify(
                json.userActions[i].properties
              );

            // convert dates to local time for display
            if ("timeTag" in json.userActions[i])
              json.userActions[i].timeTag = new Date(
                json.userActions[i].timeTag
              ).toString();
          }
          this.userActions.length = 0;
          this.userActions.push(...json.userActions);
          this.totalUserActions = json.countTotal;
          this.loading = false;
        })
        .catch((err) => {
          console.warn(err);
          this.loading = false;
        });
    },
  },
};
</script>