<template>
  <v-data-table-server v-model:items-per-page="itemsPerPage" v-model:expanded="expanded" show-expand :headers="headers"
    :items="userActions" v-model:options="options" :items-length="totalUserActions" :loading="loading" multi-sort
    class="mi-dataTable elevation-1" density="compact" item-value="timeTag">
    <template v-slot:top>
      <v-toolbar flat class="d-print-none">
        <v-divider class="mx-4" inset vertical></v-divider>
        <v-text-field density="compact" append-icon="mdi-magnify" v-on:keyup.enter="fetchUserActions()"
          v-model="searchUsername" :label="t('admin.userActions.searchUsername')" hide-details></v-text-field>
        <v-spacer></v-spacer>

        <v-text-field density="compact"  append-icon="mdi-magnify" v-on:keyup.enter="fetchUserActions()"
          v-model="searchAction" :label="t('admin.userActions.searchAction')" hide-details></v-text-field>
        <v-spacer></v-spacer>

        <v-menu v-model="menu" :close-on-content-click="false" transition="scale-transition" offset-y min-width="auto">
          <template #activator="{ props }">
            <v-text-field class="mt-4 mx-auto" v-model="dateRangeText" :label="t('admin.userActions.searchDateRange')"
              prepend-icon="mdi-calendar" readonly v-bind="props"></v-text-field>
          </template>

          <v-date-picker v-model="dates" :locale="locale" no-title scrollable show-adjacent-months multiple="range"
            @update:model-value="fetchUserActions">
          </v-date-picker>
        </v-menu>
        <v-spacer></v-spacer>
        <v-btn color="primary" dark class="mb-2 mr-2" @click="fetchUserActions()">
          <v-icon dark> mdi-refresh </v-icon>
        </v-btn>
      </v-toolbar>
    </template>
    <template v-slot:expanded-row="{ columns, item }">
      <tr>
        <td :colspan="columns.length">
          {{ item.localTime }}
          <pre>{{ JSON.stringify(item.properties, null, 2) }}</pre>
        </td>
      </tr>
    </template>
  </v-data-table-server>
</template>

<script setup>
import { ref, computed, watch, onMounted, toRaw } from 'vue';
import { useI18n } from 'vue-i18n';

const { t, locale } = useI18n();

const menu = ref(false);
const userActions = ref([]);
const searchUsername = ref('');
const searchAction = ref('');
const totalUserActions = ref(0);
const itemsPerPage = ref(10);
const dates = ref([]);
const loading = ref(true);
const options = ref({
  page: 1,
  itemsPerPage: 10,
  sortBy: [],
  sortDesc: [],
});
const expanded = ref([]);

const headers = computed(() => [
  {
    title: t("admin.userActions.headers.time"),
    key: "timeTag",
  },
  {
    title: t("admin.userActions.headers.username"),
    key: "username",
  },
  {
    title: t("admin.userActions.headers.action"),
    key: "action",
  },
  {
    title: t("admin.userActions.headers.tag"),
    key: "tag",
  },
  {
    title: t("admin.userActions.headers.pointKey"),
    key: "pointKey",
  },
  { title: '', key: 'data-table-expand' },
]);

const dateRangeText = computed(() => {
  return dates.value.join(' - ');
});

const items = computed(() => {
  return [
    {
      name: "User Actions",
      children: userActions.value,
    },
  ];
});

watch(options, () => {
  fetchUserActions();
}, { deep: true });

onMounted(() => {
  fetchUserActions();
});

const fetchUserActions = async () => {
  const { sortBy, sortDesc, page, itemsPerPage } = toRaw(options.value);
  let filter = {};

  if (searchUsername.value.trim() !== "")
    filter.username = { $regex: searchUsername.value, $options: "i" };
  if (searchAction.value.trim() !== "")
    filter.action = { $regex: searchAction.value, $options: "i" };

  let dts = dates.value.map(date => new Date(date));

  function addDays(date, days) {
    const result = new Date(date);
    result.setDate(result.getDate() + days);
    return result;
  }

  if (dts.length > 1 && dts[1] < dts[0]) {
    [dts[0], dts[1]] = [dts[1], dts[0]];
  }

  if (dts.length > 0)
    filter.timeTag = { $gte: dts[0], $lt: addDays(dts[0], 1) };
  if (dts.length > 1)
    filter.timeTag = { $gte: dts[0], $lt: addDays(dts[1], 1) };

  loading.value = true;
  try {
    const response = await fetch("/Invoke/auth/listUserActions", {
      method: "post",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        itemsPerPage,
        sortBy,
        sortDesc,
        page,
        filter,
      }),
    });
    const json = await response.json();

    userActions.value = json.userActions.map(action => ({
      ...action,
      timeTag: action.timeTag ? new Date(action.timeTag).toISOString() : '',
      localTime: action.timeTag ? new Date(action.timeTag).toLocaleString() : '',
    }));

    totalUserActions.value = json.countTotal;
    console.log('Total User Actions:', totalUserActions.value); // Debug log
  } catch (err) {
    console.warn(err);
    totalUserActions.value = 0; // Set to 0 if there's an error
  } finally {
    loading.value = false;
  }
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