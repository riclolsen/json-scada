<template>
    <div>
        <v-data-table-server :headers="headers" :items="tags" :items-per-page="itemsPerPage" :page="page"
            :items-length="totalTags" :loading="loading" @update:options="handleOptionsUpdate" multi-sort
            class="elevation-1" density="compact">
            <template v-slot:top>
                <v-toolbar flat class="d-print-none">
                    <v-divider class="mx-4" inset vertical></v-divider>
                    <v-text-field density="compact" @update:model-value="fetchTags" v-model="searchTag"
                        append-icon="mdi-magnify" :label="$t('admin.tags.searchTag')" hide-details></v-text-field>
                    <v-spacer></v-spacer>

                    <v-text-field density="compact" @update:model-value="fetchTags" v-model="searchDescription"
                        append-icon="mdi-magnify" :label="$t('admin.tags.searchDescription')"
                        hide-details></v-text-field>
                    <v-spacer></v-spacer>
                    <v-dialog v-model="dialogDelete" max-width="500px">
                        <v-card>
                            <v-card-title class="headline">{{ $t("admin.tags.confirmErase") }}</v-card-title>
                            <v-card-text>
                                <v-text-field density="compact" readonly variant="outlined" v-model="editedItem._id"
                                    :label="$t('admin.tags.eraseId')"></v-text-field>
                                <v-spacer></v-spacer>
                                <v-text-field density="compact" readonly variant="outlined" v-model="editedItem.tag"
                                    :label="$t('admin.tags.eraseName')"></v-text-field>
                            </v-card-text>
                            <v-card-actions>
                                <v-spacer></v-spacer>
                                <v-btn color="blue-darken-1" variant="text" @click="closeDelete">{{
                                    $t("admin.tags.eraseCancel") }}</v-btn>
                                <v-btn color="blue-darken-1" variant="text" @click="deleteTag">{{
                                    $t("admin.tags.eraseExecute") }}</v-btn>
                                <v-spacer></v-spacer>
                            </v-card-actions>
                        </v-card>
                    </v-dialog>
                    <v-dialog v-model="dialog" max-width="500px">
                        <template v-slot:activator="{ props }">
                            <v-btn color="primary" dark class="mb-2 mr-2" v-bind="props" @click="fetchTags()">
                                <v-icon>mdi-refresh</v-icon>
                            </v-btn>
                            <v-btn color="primary" dark class="mb-2 mr-2" v-bind="props" @click="newTag()">
                                <v-icon>mdi-plus</v-icon>
                                {{ $t("admin.tags.newTag") }}
                            </v-btn>
                        </template>
                        <v-card>
                            <v-card-title>
                                <span class="headline">{{ $t("admin.tags.editTag") }}</span>
                            </v-card-title>

                            <v-card-text>
                                <v-text-field density="compact" v-model="editedItem._id"
                                    :label="$t('admin.tags.editId')"></v-text-field>
                                <v-text-field density="compact" v-model="editedItem.tag"
                                    :label="$t('admin.tags.editName')"></v-text-field>

                                <v-text-field dense v-model="editedItem._id"
                                    :label="$t('admin.tags.editId')"></v-text-field>
                                <v-text-field dense v-model="editedItem.tag"
                                    :label="$t('admin.tags.editName')"></v-text-field>
                                <v-text-field dense v-model="editedItem.description"
                                    :label="$t('admin.tags.editDescription')"></v-text-field>
                                <v-text-field dense v-model="editedItem.group1"
                                    :label="$t('admin.tags.editGroup1')"></v-text-field>
                                <v-text-field dense v-model="editedItem.group2"
                                    :label="$t('admin.tags.editGroup2')"></v-text-field>
                                <v-text-field dense v-model="editedItem.group3"
                                    :label="$t('admin.tags.editGroup3')"></v-text-field>
                                <v-select :items="[
                                    'supervised',
                                    'command',
                                    'calculated',
                                    'manual',
                                ]" :label="$t('admin.tags.editOrigin')"
                                    v-model="editedItem.origin" class="ma-0"></v-select>
                                <v-select :items="['digital', 'analog', 'string']" :label="$t('admin.tags.editType')"
                                    v-model="editedItem.type" class="ma-0"></v-select>
                                <v-text-field dense v-if="editedItem.type == 'digital'"
                                    v-model="editedItem.stateTextFalse"
                                    :label="$t('admin.tags.editStateTextFalse')"></v-text-field>
                                <v-text-field dense v-if="editedItem.type == 'digital'"
                                    v-model="editedItem.stateTextTrue"
                                    :label="$t('admin.tags.editStateTextTrue')"></v-text-field>
                                <v-text-field dense v-if="editedItem.type == 'digital'"
                                    v-model="editedItem.eventTextFalse"
                                    :label="$t('admin.tags.editEventTextFalse')"></v-text-field>
                                <v-text-field dense v-if="editedItem.type == 'digital'"
                                    v-model="editedItem.eventTextTrue"
                                    :label="$t('admin.tags.editEventTextTrue')"></v-text-field>
                                <v-text-field dense v-if="editedItem.type == 'analog'" v-model="editedItem.unit"
                                    :label="$t('admin.tags.editUnit')"></v-text-field>

                                <v-text-field dense type="number" v-if="['supervised'].includes(editedItem.origin)"
                                    v-model="editedItem.commandOfSupervised" :label="$t('admin.tags.editCommandOfSupervised')
                                        "></v-text-field>
                                <v-text-field dense type="number" v-if="['command'].includes(editedItem.origin)"
                                    v-model="editedItem.supervisedOfCommand" :label="$t('admin.tags.editSupervisedOfCommand')
                                        "></v-text-field>
                                <v-text-field dense type="number" v-if="['supervised'].includes(editedItem.origin)"
                                    v-model="editedItem.invalidDetectTimeout" :label="$t('admin.tags.editInvalidDetectTimeout')
                                        "></v-text-field>

                                <v-text-field dense type="number" v-if="
                                    ['supervised', 'command'].includes(editedItem.origin)
                                " v-model="editedItem.protocolSourceConnectionNumber"
                                    :label="$t(
                                        'admin.tags.editProtocolSourceConnectionNumber'
                                    )
                                        "></v-text-field>
                                <v-text-field dense v-if="
                                    ['supervised', 'command'].includes(editedItem.origin)
                                " v-model="editedItem.protocolSourceASDU" :label="$t('admin.tags.editProtocolSourceASDU')
                                                                "></v-text-field>
                                <v-text-field dense v-if="
                                    ['supervised', 'command'].includes(editedItem.origin)
                                " v-model="editedItem.protocolSourceCommonAddress" :label="$t(
                                                                'admin.tags.editProtocolSourceCommonAddress'
                                                            )
                                                                "></v-text-field>
                                <v-text-field dense v-if="
                                    ['supervised', 'command'].includes(editedItem.origin)
                                " v-model="editedItem.protocolSourceObjectAddress" :label="$t(
                                                                'admin.tags.editProtocolSourceObjectAddress'
                                                            )
                                                                "></v-text-field>

                                <v-text-field dense v-if="['command'].includes(editedItem.origin)"
                                    v-model="editedItem.protocolSourceCommandDuration" :label="$t(
                                        'admin.tags.editProtocolSourceCommandDuration'
                                    )
                                        "></v-text-field>

                                <v-text-field dense type="number" v-if="
                                    ['supervised', 'command'].includes(editedItem.origin)
                                " v-model="editedItem.kconv1"
                                    :label="$t('admin.tags.editKconv1')"></v-text-field>
                                <v-text-field dense type="number" v-if="
                                    ['supervised', 'command'].includes(editedItem.origin)
                                " v-model="editedItem.kconv2"
                                    :label="$t('admin.tags.editKconv2')"></v-text-field>

                                <v-switch dense v-if="
                                    ['command'].includes(editedItem.origin) &&
                                    [
                                        '45',
                                        '46',
                                        '47',
                                        '58',
                                        '59',
                                        '60',
                                        '0',
                                        '1',
                                        '2',
                                        '3',
                                        '4'
                                    ].includes('' + editedItem.protocolSourceASDU)
                                " v-model="editedItem.protocolSourceCommandUseSBO" inset
                                    color="primary" :label="`${$t(
                                        'admin.tags.editProtocolSourceCommandUseSBO'
                                    )}${editedItem.protocolSourceCommandUseSBO
                                        ? $t(
                                            'admin.tags.editProtocolSourceCommandUseSBOTrue'
                                        )
                                        : $t(
                                            'admin.tags.editProtocolSourceCommandUseSBOFalse'
                                        )
                                        }`" class="mt-0"></v-switch>

                                <v-switch dense v-if="['supervised'].includes(editedItem.origin)"
                                    v-model="editedItem.isEvent" inset color="primary" :label="`${$t('admin.tags.editIsEvent')}${editedItem.isEvent
                                        ? $t('admin.tags.editIsEventTrue')
                                        : $t('admin.tags.editIsEventFalse')
                                        }`" class="mt-0"></v-switch>

                            </v-card-text>

                            <v-card-actions>
                                <v-spacer></v-spacer>
                                <v-btn color="blue-darken-1" variant="text" @click="close">
                                    {{ $t("admin.tags.editCancel") }}
                                </v-btn>
                                <v-btn color="blue-darken-1" variant="text" @click="save">
                                    {{ $t("admin.tags.editExecute") }}
                                </v-btn>
                            </v-card-actions>
                        </v-card>
                    </v-dialog>
                </v-toolbar>
            </template>

            <template v-slot:[`item.Actions`]="{ item }">
                <v-icon size="small" class="me-2" @click="editTag(item)">mdi-pencil</v-icon>
                <v-icon size="small" @click="deleteTagOpenDialog(item)">mdi-delete</v-icon>
            </template>
        </v-data-table-server>
    </div>
</template>

<script setup>
import { ref, reactive, computed, nextTick, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const dialog = ref(false);
const dialogDelete = ref(false);
const tags = ref([]);
const totalTags = ref(0);
const searchTag = ref('');
const searchDescription = ref('');
const loading = ref(true);
const itemsPerPage = ref(10);
const page = ref(1);
const sortBy = ref([]);
const sortDesc = ref([]);

const editedIndex = ref(-1);
const editedItem = reactive({
    tag: '',
    description: '',
    group1: '',
});

const defaultItem = {
    tag: '',
    description: '',
    group1: '',
};

const headers = computed(() => [
    {
        title: t("admin.tags.headers.tag"),
        align: "start",
        sortable: true,
        key: "tag",
    },
    {
        title: t("admin.tags.headers.id"),
        sortable: true,
        key: "_id",
    },
    {
        title: t("admin.tags.headers.group1"),
        sortable: true,
        key: "group1",
    },
    {
        title: t("admin.tags.headers.description"),
        sortable: true,
        key: "description",
    },
    { title: t("admin.tags.headers.value"), key: "value" },
    { title: t("admin.tags.headers.valueJson"), key: "valueJson" },
    { title: t("admin.tags.headers.valueString"), key: "valueString" },
    {
        title: t("admin.tags.headers.actions"),
        key: "Actions",
        sortable: false,
    },
]);

const handleOptionsUpdate = (newOptions) => {
    page.value = newOptions.page;
    itemsPerPage.value = newOptions.itemsPerPage;
    sortBy.value = newOptions.sortBy;
    sortDesc.value = newOptions.sortDesc;
    fetchTags();
};

const newTag = async () => {
    try {
        const response = await fetch("/Invoke/auth/createTag", {
            method: "post",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ tag: "a_new_tag" }),
        });
        const json = await response.json();
        if (json.error) console.log(json);

        sortBy.value = ["_id"];
        sortDesc.value = [true];
        await fetchTags();
    } catch (err) {
        console.warn(err);
    }
};

const editTag = (item) => {
    editedIndex.value = tags.value.indexOf(item);
    Object.assign(editedItem, item);
    dialog.value = true;
};

const deleteTag = async () => {
    dialogDelete.value = false;
    try {
        const response = await fetch("/Invoke/auth/deleteTag", {
            method: "post",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                tag: editedItem.tag,
                _id: editedItem._id,
            }),
        });
        const json = await response.json();
        if (json.error) console.log(json);
        fetchTags();
    } catch (err) {
        console.warn(err);
    }
};

const deleteTagOpenDialog = (item) => {
    editedIndex.value = tags.value.indexOf(item);
    Object.assign(editedItem, item);
    dialogDelete.value = true;
};

const close = () => {
    dialog.value = false;
    nextTick(() => {
        Object.assign(editedItem, defaultItem);
        editedIndex.value = -1;
    });
};

const closeDelete = () => {
    dialogDelete.value = false;
    nextTick(() => {
        Object.assign(editedItem, defaultItem);
        editedIndex.value = -1;
    });
};

const save = async () => {
    if (editedIndex.value > -1) {
        try {
            const response = await fetch("/Invoke/auth/updateTag", {
                method: "post",
                headers: {
                    Accept: "application/json",
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(editedItem),
            });
            const json = await response.json();
            if (json.error) console.log(json);
            close();
            fetchTags();
        } catch (err) {
            console.warn(err);
            close();
        }
    }
    close();
};

const fetchTags = async () => {
    let filter = {};

    if (searchTag.value.trim() !== "")
        filter.tag = { $regex: searchTag.value, $options: "i" };
    if (searchDescription.value.trim() !== "")
        filter.description = { $regex: searchDescription.value, $options: "i" };

    loading.value = true;
    try {
        const response = await fetch("/Invoke/auth/listTags", {
            method: "post",
            headers: {
                Accept: "application/json",
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                itemsPerPage: itemsPerPage.value,
                sortBy: sortBy.value,
                sortDesc: sortDesc.value,
                page: page.value,
                filter: filter,
            }),
        });
        const json = await response.json();
        tags.value = json.tags;
        totalTags.value = json.countTotal;
    } catch (err) {
        console.warn(err);
    } finally {
        loading.value = false;
    }
};

onMounted(() => {
    fetchTags();
});

defineExpose({ fetchTags });
</script>