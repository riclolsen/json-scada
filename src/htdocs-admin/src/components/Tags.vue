<template>
  <div>
    <v-data-table
      :headers="headers"
      :items="tags"
      :options.sync="options"
      :server-items-length="totalTags"
      :loading="loading"
      multi-sort
      class="elevation-1"
      dense
      :items-per-page="15"
    >
      <template v-slot:top>
        <v-toolbar flat>
          <v-toolbar-title>Tags</v-toolbar-title>
          <v-divider class="mx-4" inset vertical></v-divider>
          <v-text-field
            dense
            @change="fetchTags"
            v-model="searchTag"
            append-icon="mdi-magnify"
            label="Search Tag"
            hide-details
          ></v-text-field>
          <v-spacer></v-spacer>

          <v-text-field
            dense
            @change="fetchTags"
            v-model="searchDescription"
            append-icon="mdi-magnify"
            label="Description"
            hide-details
          ></v-text-field>
          <v-spacer></v-spacer>
          <v-divider class="mx-4" inset vertical></v-divider>
          <v-dialog v-model="dialogDelete" max-width="500px">
            <v-card>
              <v-card-title class="headline">Confirm tag erase?</v-card-title>
              <v-card-text>
                <v-text-field
                  dense
                  readonly
                  filled
                  v-model="editedItem._id"
                  label="Tag _id"
                ></v-text-field>
                <v-spacer></v-spacer>
                <v-text-field
                  dense
                  readonly
                  filled
                  v-model="editedItem.tag"
                  label="Tag name"
                ></v-text-field>
              </v-card-text>
              <v-card-actions>
                <v-spacer></v-spacer>
                <v-btn color="blue darken-1" text @click="closeDelete"
                  >Cancel</v-btn
                >
                <v-btn color="blue darken-1" text @click="deleteTag">OK</v-btn>
                <v-spacer></v-spacer>
              </v-card-actions>
            </v-card>
          </v-dialog>
          <v-dialog v-model="dialog" max-width="500px">
            <template v-slot:activator="{ on, attrs }">
              <v-btn color="primary" dark class="mb-2" v-bind="attrs" v-on="on" @click="newTag()">
                New Tag
              </v-btn>
            </template>
            <v-card>
              <v-card-title>
                <span class="headline">Edit Tag</span>
              </v-card-title>

              <v-card-text>
                <v-container>
                  <v-row dense>
                    <v-col cols="12" sm="6" md="12">
                      <v-text-field
                        dense
                        v-model="editedItem._id"
                        label="Tag _id"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.tag"
                        label="Tag name"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.description"
                        label="Description"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.group1"
                        label="Group 1 (e.g.Installation/Plant)"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.group2"
                        label="Group 2 (e.g.Area/Bay)"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.group3"
                        label="Group 3 (e.g.Equipment/Device)"
                      ></v-text-field>
                      <v-select
                        :items="[
                          'supervised',
                          'command',
                          'calculated',
                          'manual',
                        ]"
                        label="Origin"
                        v-model="editedItem.origin"
                        class="ma-0"
                      ></v-select>
                      <v-select
                        :items="['digital', 'analog', 'string']"
                        label="Type"
                        v-model="editedItem.type"
                        class="ma-0"
                      ></v-select>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'digital'"
                        v-model="editedItem.stateTextFalse"
                        label="State Text False"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'digital'"
                        v-model="editedItem.stateTextTrue"
                        label="State Text True"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'digital'"
                        v-model="editedItem.eventTextFalse"
                        label="Event Text False"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'digital'"
                        v-model="editedItem.eventTextTrue"
                        label="Event Text True"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'analog'"
                        v-model="editedItem.unit"
                        label="Unit"
                      ></v-text-field>

                      <v-text-field
                        dense
                        type="number"
                        v-if="['supervised'].includes(editedItem.origin)"
                        v-model="editedItem.commandOfSupervised"
                        label="Command point of supervised (use zero for none)"
                      ></v-text-field>
                      <v-text-field
                        dense
                        type="number"
                        v-if="['command'].includes(editedItem.origin)"
                        v-model="editedItem.supervisedOfCommand"
                        label="Supervised point of command (use zero for none)"
                      ></v-text-field>
                      <v-text-field
                        dense
                        type="number"
                        v-if="
                          ['supervised'].includes(editedItem.origin)
                        "
                        v-model="editedItem.invalidDetectTimeout"
                        label="Invalid detect timeout (seconds)"
                      ></v-text-field>

                      <v-text-field
                        dense
                        type="number"
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.protocolSourceConnectionNumber"
                        label="Protocol Source Connection Number"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.protocolSourceASDU"
                        label="protocolSourceASDU"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.protocolSourceCommonAddress"
                        label="protocolSourceCommonAddress"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.protocolSourceObjectAddress"
                        label="protocolSourceObjectAddress"
                      ></v-text-field>

                      <v-text-field
                        dense
                        v-if="['command'].includes(editedItem.origin)"
                        v-model="editedItem.protocolSourceCommandDuration"
                        label="protocolSourceCommandDuration"
                      ></v-text-field>

                      <v-text-field
                        dense
                        type="number"
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.kconv1"
                        label="Convertion factor multiplier (kconv1)"
                      ></v-text-field>
                      <v-text-field
                        dense
                        type="number"
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.kconv2"
                        label="Convertion factor adder (kconv2)"
                      ></v-text-field>

                      <v-switch
                        dense
                        v-if="['command'].includes(editedItem.origin)"
                        v-model="editedItem.protocolSourceCommandUseSBO"
                        inset
                        color="primary"
                        :label="`Use SBO: ${
                          editedItem.protocolSourceCommandUseSBO
                            ? 'true'
                            : 'false'
                        }`"
                        class="mt-0"
                      ></v-switch>

                      <v-switch
                        dense
                        v-if="['supervised'].includes(editedItem.origin)"
                        v-model="editedItem.isEvent"
                        inset
                        color="primary"
                        :label="`Is event: ${
                          editedItem.isEvent ? 'true' : 'false'
                        }`"
                        class="mt-0"
                      ></v-switch>
                    </v-col>
                  </v-row>
                </v-container>
              </v-card-text>

              <v-card-actions>
                <v-spacer></v-spacer>
                <v-btn color="blue darken-1" text @click="close">
                  Cancel
                </v-btn>
                <v-btn color="blue darken-1" text @click="save"> Save </v-btn>
              </v-card-actions>
            </v-card>
          </v-dialog>
        </v-toolbar>
      </template>

      <template>
        <v-icon small class="mr-2" @click="editTag(item)"> mdi-pencil </v-icon>
        <v-icon small @click="deleteTagOpenDialog(item)"> mdi-delete </v-icon>
      </template>
    </v-data-table>
  </div>
</template>
<script>
import i18n from "@/i18n/i18n-current";

export default {
  name: "Tags",

  data: () => ({
    msg: { ...i18n },
    dialog: false,
    dialogDelete: false,
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
      { text: "Actions", value: "Actions", sortable: false },
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
    newTag() {
      this.editedItem = Object.assign({}, {
        _id: 0.0,
        tag: "a_new_tag"
      });
      this.dialog = true;
    },
    editTag(item) {      
      this.editedIndex = this.tags.indexOf(item);
      console.log(this.editedIndex);
      this.editedItem = Object.assign({}, item);
      this.dialog = true;
    },
    async deleteTag() {
      this.dialogDelete = false;
      return await fetch("/Invoke/auth/deleteTag", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          tag: this.editedItem.tag,
          _id: this.editedItem._id,
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchTags(); // refreshes tags
        })
        .catch((err) => console.warn(err));
    },
    deleteTagOpenDialog(item) {
      this.editedIndex = this.tags.indexOf(item);
      this.editedItem = Object.assign({}, item);
      this.dialogDelete = true;      
    },
    close() {
      this.dialog = false;
      this.$nextTick(() => {
        this.editedItem = Object.assign({}, this.defaultItem);
        this.editedIndex = -1;
      });
    },
    closeDelete() {
      this.dialogDelete = false;
      this.$nextTick(() => {
        this.editedItem = Object.assign({}, this.defaultItem);
        this.editedIndex = -1;
      });
    },
    async save() {
      if (this.editedIndex > -1) {
        // Object.assign(this.tags[this.editedIndex], this.editedItem);
        await fetch("/Invoke/auth/updateTag", {
          method: "post",
          headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
          },
          body: JSON.stringify(this.editedItem),
        })
          .then((res) => res.json())
          .then((json) => {
            if (json.error) console.log(json);
            this.close();
            this.fetchTags(); // refreshes tags
          })
          .catch((err) => {console.warn(err);this.close();});
      } else {
        //  this.tags.push(this.editedItem);
      }
      this.close();
    },
    async fetchTags() {
      const { sortBy, sortDesc, page, itemsPerPage } = this.options;
      let filter = {};

      if (this.searchTag.trim() != "")
        filter.tag = { $regex: this.searchTag, $options: "i" };
      if (this.searchDescription.trim() != "")
        filter.description = { $regex: this.searchDescription, $options: "i" };

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
          filter: filter,
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