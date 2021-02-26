<template>
<div>
  <v-container class="grey lighten-5">
    <v-row no-gutters>
      <v-col
        cols="12"
        sm="2"
      >
        <v-card
          class="pa-2"
          outlined
          tile
        >
      <v-spacer></v-spacer>
      <v-text-field
        dense
        @change="fetchTags"
        v-model="searchTag"
        append-icon="mdi-magnify"
        label="Search Tag"
        hide-details
      ></v-text-field>
        </v-card>
      </v-col>
      <v-col
        cols="12"
        sm="2"
      >
        <v-card
          class="pa-2"
          outlined
          tile
        >
      <v-spacer></v-spacer>
      <v-text-field
        dense
        @change="fetchTags"
        v-model="searchDescription"
        append-icon="mdi-magnify"
        label="Description"
        hide-details
      ></v-text-field>
        </v-card>
      </v-col>
    </v-row>
  </v-container>

   <v-data-table
      :headers="headers"
      :items="tags"
      :options.sync="options"
      :server-items-length="totalTags"
      :loading="loading"
      :search="search"
      multi-sort
      class="elevation-1"
      dense
      :items-per-page="15"
    >
    </v-data-table>
    </div>
</template>
<script>
// const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

export default {
  name: "Tags",

  data: () => ({
    dialog: false,
    active: [],
    open: [],
    tags: [],
    totalTags: 0,
    searchTag: "",
    searchDescription: "",
    loading: true,
    options: {},
    headers: [
      {
        text: "Tag",
        align: "start",
        sortable: true,
        value: "tag",
      },
      { text: "Id", sortable: true, value: "_id" },
      { text: "Group1", sortable: true, value: "group1" },
      { text: "Description", sortable: true, value: "description" },
      { text: "Value", value: "value" },
    ],
  }),

  computed: {
    items() {
      return [
        {
          name: "Tags",
          children: this.tags,
        },
      ];
    },
    selected() {
      if (!this.active.length) return undefined;

      // const id = this.active[0];

      return this.tags.find((tag) => tag.tag === tag);
    },
  },

  watch: {
    options: {
      handler() {
        this.fetchTags();
      },
      deep: true,
    },
  },

  mounted() {
    this.fetchTags();
  },

  methods: {
    async fetchTags() {
      const { sortBy, sortDesc, page, itemsPerPage } = this.options;
      let filter = {}

      if (this.searchTag.trim()!="")
        filter.tag =  {'$regex': this.searchTag, '$options': 'i'}
      if (this.searchDescription.trim()!="")
        filter.description =  {'$regex': this.searchDescription, '$options': 'i'}

      this.loading = true;
      return await fetch("/Invoke/auth/listTags", {
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
          filter: filter
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          //for (let i = 0; i < json.length; i++) {
          //  json[i].id = (page-1)*itemsPerPage + i + 1;
          // }
          this.tags.length = 0;
          this.tags.push(...json.tags);
          this.totalTags = json.countTotal;
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