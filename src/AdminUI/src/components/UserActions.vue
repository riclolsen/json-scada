<template>
  <div>
    <v-data-table
      :headers="headers"
      :items="userActions"
      v-model:options="options"
      :items-length="totalUserActions"
      :loading="loading"
      multi-sort
      class="mi-dataTable elevation-1"
      density="compact"
      :items-per-page="15"
    >
      <template #top>
        <v-toolbar flat class="d-print-none">
          <v-toolbar-title>{{
            t("src\\components\\useractions.userActions")
          }}</v-toolbar-title>
          <v-divider class="mx-4" inset vertical></v-divider>
          <v-text-field
            density="compact"
            @update:model-value="fetchUserActions"
            append-icon="mdi-magnify"
            v-model="searchUsername"
            :label="t('src\\components\\useractions.searchUsername')"
            hide-details
          ></v-text-field>
          <v-spacer></v-spacer>

          <v-text-field
            density="compact"
            @update:model-value="fetchUserActions"
            append-icon="mdi-magnify"
            v-model="searchAction"
            :label="t('src\\components\\useractions.searchAction')"
            hide-details
          ></v-text-field>
          <v-spacer></v-spacer>

          <v-menu
            v-model="menu"
            :close-on-content-click="false"
            transition="scale-transition"
            offset-y
            min-width="auto"
          >
            <template #activator="{ props }">
              <v-text-field
                class="mt-4 mx-auto"
                v-model="dateRangeText"
                :label="t('src\\components\\useractions.searchDateRange')"
                prepend-icon="mdi-calendar"
                readonly
                v-bind="props"
              ></v-text-field>
            </template>

            <v-date-picker
              v-model="dates"
              :locale="locale"
              no-title
              scrollable
              show-adjacent-months
              range
              @update:model-value="fetchUserActions"
            >
            </v-date-picker>
          </v-menu>
          <v-spacer></v-spacer>
          <v-btn
            color="primary"
            dark
            class="mb-2 mr-2"
            @click="fetchUserActions()"
          >
            <v-icon dark> mdi-refresh </v-icon>
          </v-btn>
        </v-toolbar>
      </template>
    </v-data-table>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';

const { t, locale } = useI18n();

const menu = ref(false);
const userActions = ref([]);
const searchUsername = ref('');
const searchAction = ref('');
const totalUserActions = ref(0);
const dates = ref([]);
const loading = ref(true);
const options = ref({});

const headers = [
  {
    title: t("src\\components\\useractions.colUsername"),
    key: "username",
    width: "8%",
  },
  {
    title: t("src\\components\\useractions.colAction"),
    key: "action",
    width: "8%",
  },
  {
    title: t("src\\components\\useractions.colTag"),
    key: "tag",
    width: "8%",
  },
  {
    title: t("src\\components\\useractions.colPointKey"),
    key: "pointKey",
    width: "8%",
  },
  {
    title: t("src\\components\\useractions.colProperties"),
    key: "propertiesString",
    width: "35%",
  },
  {
    title: t("src\\components\\useractions.colTime"),
    key: "timeTag",
    width: "15%",
  },
];

const dateRangeText = computed(() => {
  return dates.value.join(' - ');
});

const fetchUserActions = () => {
  // Implement your fetch logic here
};

</script>
  <style scoped>
  table {
    table-layout: fixed;
  }
  td {
    vertical-align: top;
  }
  </style>
  <script>
  import i18n from "../i18n.js";
  
  export default {
    name: "UserActions",
  
    data: () => ({
      menu: false,
      attrs: {},
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
        //{
        //  text: i18n.t("src\\components\\useractions.colId"),
        //  sortable: true,
        //  value: "_id",
        //  width: "15%",
        //  hidden: true
        //},
        {
          text: i18n.t("src\\components\\useractions.colUsername"),
          sortable: true,
          value: "username",
          width: "8%",
        },
        {
          text: i18n.t("src\\components\\useractions.colAction"),
          sortable: true,
          value: "action",
          width: "8%",
        },
        {
          text: i18n.t("src\\components\\useractions.colTag"),
          sortable: true,
          value: "tag",
          width: "8%",
        },
        {
          text: i18n.t("src\\components\\useractions.colPointKey"),
          sortable: true,
          value: "pointKey",
          width: "8%",
        },
        {
          text: i18n.t("src\\components\\useractions.colProperties"),
          sortable: false,
          value: "propertiesString",
          width: "35%",
        },
        {
          text: i18n.t("src\\components\\useractions.colTime"),
          sortable: true,
          value: "timeTag",
          width: "15%",
        },
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