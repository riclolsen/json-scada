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
        <v-toolbar flat class="d-print-none">
          <v-toolbar-title>{{
            $t("src\\components\\tags.tags")
          }}</v-toolbar-title>
          <v-divider class="mx-4" inset vertical></v-divider>
          <v-text-field
            dense
            @change="fetchTags"
            v-model="searchTag"
            append-icon="mdi-magnify"
            :label="$t('src\\components\\tags.searchTag')"
            hide-details
          ></v-text-field>
          <v-spacer></v-spacer>

          <v-text-field
            dense
            @change="fetchTags"
            v-model="searchDescription"
            append-icon="mdi-magnify"
            :label="$t('src\\components\\tags.searchDescription')"
            hide-details
          ></v-text-field>
          <v-spacer></v-spacer>
          <v-divider class="mx-4" inset vertical></v-divider>
          <v-dialog v-model="dialogDelete" max-width="500px">
            <v-card>
              <v-card-title class="headline">{{
                $t("src\\components\\tags.confirmErase")
              }}</v-card-title>
              <v-card-text>
                <v-text-field
                  dense
                  readonly
                  filled
                  v-model="editedItem._id"
                  :label="$t('src\\components\\tags.eraseId')"
                ></v-text-field>
                <v-spacer></v-spacer>
                <v-text-field
                  dense
                  readonly
                  filled
                  v-model="editedItem.tag"
                  :label="$t('src\\components\\tags.eraseName')"
                ></v-text-field>
              </v-card-text>
              <v-card-actions>
                <v-spacer></v-spacer>
                <v-btn color="blue darken-1" text @click="closeDelete">{{
                  $t("src\\components\\tags.eraseCancel")
                }}</v-btn>
                <v-btn color="blue darken-1" text @click="deleteTag">{{
                  $t("src\\components\\tags.eraseExecute")
                }}</v-btn>
                <v-spacer></v-spacer>
              </v-card-actions>
            </v-card>
          </v-dialog>
          <v-dialog v-model="dialog" max-width="500px">
            <template v-slot:activator="{ attrs }">
              <v-btn
                color="primary"
                dark
                class="mb-2 mr-2"
                v-bind="attrs"
                @click="fetchTags()"
              >
                <v-icon dark> mdi-refresh </v-icon>
              </v-btn>
              <v-btn
                color="primary"
                dark
                class="mb-2 mr-2"
                v-bind="attrs"
                @click="newTag()"
              >
                <v-icon dark> mdi-plus </v-icon>
                {{ $t("src\\components\\tags.newTag") }}
              </v-btn>
            </template>
            <v-card>
              <v-card-title>
                <span class="headline">{{
                  $t("src\\components\\tags.editTag")
                }}</span>
              </v-card-title>

              <v-card-text>
                <v-container>
                  <v-row dense>
                    <v-col cols="12" sm="6" md="12">
                      <v-text-field
                        dense
                        v-model="editedItem._id"
                        :label="$t('src\\components\\tags.editId')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.tag"
                        :label="$t('src\\components\\tags.editName')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.description"
                        :label="$t('src\\components\\tags.editDescription')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.group1"
                        :label="$t('src\\components\\tags.editGroup1')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.group2"
                        :label="$t('src\\components\\tags.editGroup2')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-model="editedItem.group3"
                        :label="$t('src\\components\\tags.editGroup3')"
                      ></v-text-field>
                      <v-select
                        :items="[
                          'supervised',
                          'command',
                          'calculated',
                          'manual',
                        ]"
                        :label="$t('src\\components\\tags.editOrigin')"
                        v-model="editedItem.origin"
                        class="ma-0"
                      ></v-select>
                      <v-select
                        :items="['digital', 'analog', 'string']"
                        :label="$t('src\\components\\tags.editType')"
                        v-model="editedItem.type"
                        class="ma-0"
                      ></v-select>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'digital'"
                        v-model="editedItem.stateTextFalse"
                        :label="$t('src\\components\\tags.editStateTextFalse')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'digital'"
                        v-model="editedItem.stateTextTrue"
                        :label="$t('src\\components\\tags.editStateTextTrue')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'digital'"
                        v-model="editedItem.eventTextFalse"
                        :label="$t('src\\components\\tags.editEventTextFalse')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'digital'"
                        v-model="editedItem.eventTextTrue"
                        :label="$t('src\\components\\tags.editEventTextTrue')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="editedItem.type == 'analog'"
                        v-model="editedItem.unit"
                        :label="$t('src\\components\\tags.editUnit')"
                      ></v-text-field>

                      <v-text-field
                        dense
                        type="number"
                        v-if="['supervised'].includes(editedItem.origin)"
                        v-model="editedItem.commandOfSupervised"
                        :label="
                          $t('src\\components\\tags.editCommandOfSupervised')
                        "
                      ></v-text-field>
                      <v-text-field
                        dense
                        type="number"
                        v-if="['command'].includes(editedItem.origin)"
                        v-model="editedItem.supervisedOfCommand"
                        :label="
                          $t('src\\components\\tags.editSupervisedOfCommand')
                        "
                      ></v-text-field>
                      <v-text-field
                        dense
                        type="number"
                        v-if="['supervised'].includes(editedItem.origin)"
                        v-model="editedItem.invalidDetectTimeout"
                        :label="
                          $t('src\\components\\tags.editInvalidDetectTimeout')
                        "
                      ></v-text-field>

                      <v-text-field
                        dense
                        type="number"
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.protocolSourceConnectionNumber"
                        :label="
                          $t(
                            'src\\components\\tags.editProtocolSourceConnectionNumber'
                          )
                        "
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.protocolSourceASDU"
                        :label="
                          $t('src\\components\\tags.editProtocolSourceASDU')
                        "
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.protocolSourceCommonAddress"
                        :label="
                          $t(
                            'src\\components\\tags.editProtocolSourceCommonAddress'
                          )
                        "
                      ></v-text-field>
                      <v-text-field
                        dense
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.protocolSourceObjectAddress"
                        :label="
                          $t(
                            'src\\components\\tags.editProtocolSourceObjectAddress'
                          )
                        "
                      ></v-text-field>

                      <v-text-field
                        dense
                        v-if="['command'].includes(editedItem.origin)"
                        v-model="editedItem.protocolSourceCommandDuration"
                        :label="
                          $t(
                            'src\\components\\tags.editProtocolSourceCommandDuration'
                          )
                        "
                      ></v-text-field>

                      <v-text-field
                        dense
                        type="number"
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.kconv1"
                        :label="$t('src\\components\\tags.editKconv1')"
                      ></v-text-field>
                      <v-text-field
                        dense
                        type="number"
                        v-if="
                          ['supervised', 'command'].includes(editedItem.origin)
                        "
                        v-model="editedItem.kconv2"
                        :label="$t('src\\components\\tags.editKconv2')"
                      ></v-text-field>

                      <v-switch
                        dense
                        v-if="['command'].includes(editedItem.origin)"
                        v-model="editedItem.protocolSourceCommandUseSBO"
                        inset
                        color="primary"
                        :label="`${$t(
                          'src\\components\\tags.editProtocolSourceCommandUseSBO'
                        )}${
                          editedItem.protocolSourceCommandUseSBO
                            ? $t(
                                'src\\components\\tags.editProtocolSourceCommandUseSBOTrue'
                              )
                            : $t(
                                'src\\components\\tags.editProtocolSourceCommandUseSBOFalse'
                              )
                        }`"
                        class="mt-0"
                      ></v-switch>

                      <v-switch
                        dense
                        v-if="['supervised'].includes(editedItem.origin)"
                        v-model="editedItem.isEvent"
                        inset
                        color="primary"
                        :label="`${$t('src\\components\\tags.editIsEvent')}${
                          editedItem.isEvent
                            ? $t('src\\components\\tags.editIsEventTrue')
                            : $t('src\\components\\tags.editIsEventFalse')
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
                  {{ $t("src\\components\\tags.editCancel") }}
                </v-btn>
                <v-btn color="blue darken-1" text @click="save">
                  {{ $t("src\\components\\tags.editExecute") }}
                </v-btn>
              </v-card-actions>
            </v-card>
          </v-dialog>
        </v-toolbar>
      </template>

      <template v-slot:item.Actions="{ item }">
        <v-icon small class="mr-2" @click="editTag(item)"> mdi-pencil </v-icon>
        <v-icon small @click="deleteTagOpenDialog(item)"> mdi-delete </v-icon>
      </template>
    </v-data-table>
  </div>
</template>
<script>
import i18n from "../i18n.js";

export default {
  name: "Tags",

  data: () => ({
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
        text: i18n.t("src\\components\\tags.colTag"),
        align: "start",
        sortable: true,
        value: "tag",
      },
      {
        text: i18n.t("src\\components\\tags.colId"),
        sortable: true,
        value: "_id",
      },
      {
        text: i18n.t("src\\components\\tags.colGroup1"),
        sortable: true,
        value: "group1",
      },
      {
        text: i18n.t("src\\components\\tags.colDescription"),
        sortable: true,
        value: "description",
      },
      { text: i18n.t("src\\components\\tags.colValue"), value: "value" },
      {
        text: i18n.t("src\\components\\tags.colActions"),
        value: "Actions",
        sortable: false,
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
    async newTag() {
      await fetch("/Invoke/auth/createTag", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ tag: "a_new_tag" }),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          else {
            // success
          }
        })
        .catch((err) => console.warn(err));

      // sort by _id descending to be shown at the top
      this.options.sortBy = ["_id"];
      this.options.sortDesc = [true];
      await this.fetchTags(); // refreshes tags
      //this.editedItem = Object.assign({}, newTagItem);
      //this.dialog = true;
    },
    editTag(item) {
      this.editedIndex = this.tags.indexOf(item);
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
          .catch((err) => {
            console.warn(err);
            this.close();
          });
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