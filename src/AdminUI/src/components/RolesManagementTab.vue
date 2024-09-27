<template>

    <v-container fluid class="roles-management-tab">

        <v-btn color="primary" size="small" class="mt-0 me-2" @click="openAddRoleDialog">
            {{ $t('admin.rolesManagement.addRole') }}
            <v-icon>mdi-plus</v-icon>
        </v-btn>

        <v-data-table :headers="headers" :items="roles" :items-per-page="5" class="mt-4 elevation-1"
            :load-children="fetchRoles" :items-per-page-text="$t('common.itemsPerPageText')">
            <template #[`item.actions`]="{ item }">
                <v-icon size="small" class="me-2" @click="openEditRoleDialog(item)">
                    mdi-pencil
                </v-icon>
                <v-icon v-if="item.name !== 'admin'" size="small" @click="openDeleteRoleDialog(item)">
                    mdi-delete
                </v-icon>
            </template>
        </v-data-table>
        <div>
            <v-chip v-if="error" color="red darken-1">{{ $t('common.error') }}</v-chip>
        </div>
    </v-container>

    <v-dialog v-model="addRoleDialog" max-width="600px">
        <v-card>
            <v-card-title>{{ $t('admin.rolesManagement.editRole') }}</v-card-title>
            <v-card-text>
                <v-text-field v-model="newRole.name" :label="$t('admin.rolesManagement.roleName')" outlined
                    :disabled="newRole.name === 'admin' ? true : false"></v-text-field>

                <v-select v-model="newRole.group1List" :items="group1ListAll" item-title="name" outlined chips
                    closable-chips small-chips :label="$t('admin.rolesManagement.canViewGroup1List')" multiple></v-select>

                <v-select v-model="newRole.group1CommandList" :items="group1ListAll" item-title="name" outlined chips
                    closable-chips small-chips :label="$t('admin.rolesManagement.canCommandGroup1List')" multiple></v-select>

                <v-select v-model="newRole.displayList" :items="displayListAll" item-title="name" outlined chips
                    closable-chips small-chips :label="$t('admin.rolesManagement.canAccessDisplayList')" multiple></v-select>

                <v-text-field type="number" min="0" max="9999"
                    :label="$t('admin.rolesManagement.maxSessionDays') + ' - ' + $t('admin.rolesManagement.maxSessionDaysHint')"
                    hide-details="auto" v-model="newRole.maxSessionDays"></v-text-field>

                <v-checkbox dense v-model="newRole.isAdmin" class="mt-n0"
                    :label="$t('admin.rolesManagement.isAdmin') + ' - ' + $t('admin.rolesManagement.isAdminHint')"
                    :disabled="newRole.name === 'admin' ? true : false">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.changePassword" class="my-n8"
                    :label="$t('admin.rolesManagement.canChangePassword') + ' - ' + $t('admin.rolesManagement.canChangePasswordHint')"
                    :disabled="newRole.name === 'admin' ? true : false">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.sendCommands" class="my-n8"
                    :label="$t('admin.rolesManagement.sendCommands') + ' - ' + $t('admin.rolesManagement.sendCommandsHint')">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.enterAnnotations" class="my-n8"
                    :label="$t('admin.rolesManagement.enterAnnotations') + ' - ' + $t('admin.rolesManagement.enterAnnotationsHint')">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.enterNotes" class="my-n8"
                    :label="$t('admin.rolesManagement.enterNotes') + ' - ' + $t('admin.rolesManagement.enterNotesHint')">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.enterManuals" class="my-n8"
                    :label="$t('admin.rolesManagement.enterManuals') + ' - ' + $t('admin.rolesManagement.enterManualsHint')">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.enterLimits" class="my-n8"
                    :label="$t('admin.rolesManagement.enterLimits') + ' - ' + $t('admin.rolesManagement.enterLimitsHint')">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.substituteValues" class="my-n8"
                    :label="$t('admin.rolesManagement.substituteValues') + ' - ' + $t('admin.rolesManagement.substituteValuesHint')">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.ackEvents" class="my-n8"
                    :label="$t('admin.rolesManagement.ackEvents') + ' - ' + $t('admin.rolesManagement.ackEventsHint')">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.ackAlarms" class="my-n8"
                    :label="$t('admin.rolesManagement.ackAlarms') + ' - ' + $t('admin.rolesManagement.ackAlarmsHint')">
                </v-checkbox>

                <v-checkbox dense v-model="newRole.disableAlarms" class="mb-n4"
                    :label="$t('admin.rolesManagement.disableAlarms') + ' - ' + $t('admin.rolesManagement.disableAlarmsHint')">
                </v-checkbox>

                <v-card-actions>
                    <v-spacer></v-spacer>
                    <v-btn color="blue darken-1" text @click="closeAddRoleDialog">{{ $t('common.cancel') }}</v-btn>
                    <v-btn color="blue darken-1" text @click="isNewRole ? createRole(newRole) : updateRole(newRole)">{{
                        $t('common.save')
                        }}</v-btn>
                    <v-chip v-if="error" color="red darken-1">{{ $t('common.error') }}</v-chip>
                </v-card-actions>
            </v-card-text>
        </v-card>
    </v-dialog>

    <v-dialog v-model="deleteRoleDialog" max-width="400px">
        <v-card>
            <v-card-title>{{ $t('admin.rolesManagement.deleteRole') }}</v-card-title>
            <v-card-text>
                {{ $t('admin.rolesManagement.deleteRoleConfirm') }}
            </v-card-text>
            <v-card-text>
                {{ roleToDelete.name }}
            </v-card-text>
            <v-card-actions>
                <v-spacer></v-spacer>
                <v-btn color="blue darken-1" text @click="closeDeleteRoleDialog">{{ $t('common.cancel') }}</v-btn>
                <v-btn color="red darken-1" text @click="confirmDeleteRole">{{ $t('common.delete') }}</v-btn>
            </v-card-actions>
            <v-chip v-if="error" color="red darken-1">{{ $t('common.error') }}</v-chip>
        </v-card>
    </v-dialog>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

// Computed properties
const headers = computed(() => [
    { title: '#', key: 'id' },
    { title: t('admin.rolesManagement.headers.name'), align: 'start', key: 'name' },
    { title: t('admin.rolesManagement.headers.rights'), key: 'rights', sortable: false },
    { title: t('admin.rolesManagement.headers.actions'), key: 'actions', sortable: false },
]);

// Reactive variables
const roles = ref([]);
const error = ref(false);
const newRole = ref({});
const addRoleDialog = ref(false);
const displayListAll = ref([]);
const group1ListAll = ref([]);
const deleteRoleDialog = ref(false);
const roleToDelete = ref({});
let isNewRole = ref(false);

// Constants
const defaultRoleConfig = {
    name: '',
    group1List: [],
    group1CommandList: [],
    displayList: [],
    isAdmin: false,
    changePassword: true,
    sendCommands: true,
    enterAnnotations: true,
    enterNotes: true,
    enterManuals: true,
    enterLimits: true,
    substituteValues: true,
    ackEvents: true,
    ackAlarms: true,
    disableAlarms: true,
    maxSessionDays: 3,
}

// Lifecycle hooks
onMounted(async () => {
    await fetchRoles();
    await fetchDisplayList();
    await fetchGroup1List();
    document.documentElement.style.overflowY = 'scroll';
});

onUnmounted(async () => {
    document.documentElement.style.overflowY = 'auto';
});

// Methods
const openDeleteRoleDialog = (role) => {
    error.value = false;
    roleToDelete.value = role;
    deleteRoleDialog.value = true;
};

const closeDeleteRoleDialog = () => {
    error.value = false;
    deleteRoleDialog.value = false;
};

const confirmDeleteRole = () => {
    error.value = false;
    deleteRole(roleToDelete.value);
};

const openAddRoleDialog = async () => {
    isNewRole.value = true;
    error.value = false;
    newRole.value = Object.assign({}, defaultRoleConfig);
    addRoleDialog.value = true;
};

const closeAddRoleDialog = () => {
    error.value = false;
    isNewRole.value = false;
    addRoleDialog.value = false;
};

const openEditRoleDialog = async (item) => {
    await fetchRoles();
    isNewRole.value = false;
    error.value = false;
    newRole.value = item;
    addRoleDialog.value = true;
};

// API calls
const deleteRole = async (role) => {
    if (role.name === "admin") {
        return;
    }
    return await fetch("/Invoke/auth/deleteRole", {
        method: "post",
        headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
        },
        body: JSON.stringify({
            name: role.name,
            _id: role._id,
        }),
    })
        .then((res) => res.json())
        .then((json) => {
            if (json.error === false) {
                fetchRoles(); // refreshes roles
                closeDeleteRoleDialog();
            } else {
                error.value = true;
            }
        })
        .catch((err) => { error.value = true; console.warn(err); });
};

const createRole = async () => {
    if (newRole.value.name === "") {
        return;
    }
    return await fetch("/Invoke/auth/createRole", {
        method: "post",
        headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
        },
        body: JSON.stringify({
            name: newRole.value.name,
        }),
    })
        .then((res) => res.json())
        .then(async (json) => {
            if (json.error === false) {
                await updateRole(newRole);
                addRoleDialog.value = false;
            } else {
                error.value = true;
            }
            await fetchRoles();
        })
        .catch((err) => { error.value = true; console.warn(err); });
};

const updateRole = async (role) => {
    if (role.value) role = role.value;
    var roleDup = Object.assign({}, role);
    if ('id' in roleDup) delete roleDup.id;
    if ('rights' in roleDup) delete roleDup.rights;
    if ('__v' in roleDup) delete roleDup.__v;
    return await fetch("/Invoke/auth/updateRole", {
        method: "post",
        headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
        },
        body: JSON.stringify(roleDup),
    })
        .then((res) => res.json())
        .then(async (json) => {
            if (json.error === false) {
                closeAddRoleDialog();
            } else {
                error.value = true;
            }
            await fetchRoles(); // refreshes roles
        })
        .catch((err) => { error.value = true; console.warn(err); });
};

const fetchDisplayList = async () => {
    return await fetch("/Invoke/auth/listDisplays")
        .then((res) => res.json())
        .then((json) => {
            if (json.error) { console.log(json); return; }  
            displayListAll.value.length = 0;
            displayListAll.value.push(...json);
        })
        .catch((err) => { error.value = true; console.warn(err); });
};

const fetchGroup1List = async () => {
    return await fetch("/Invoke/auth/listGroup1")
        .then((res) => res.json())
        .then((json) => {
            if (json.error) { console.log(json); return; }  
            group1ListAll.value.length = 0;
            group1ListAll.value.push(...json);
        })
        .catch((err) => { error.value = true; console.warn(err); });
};

const fetchRoles = async () => {
    error.value = false;
    return await fetch("/Invoke/auth/listRoles")
        .then((res) => res.json())
        .then((json) => {
            if (json.error) { console.log(json); return; }  
            for (let i = 0; i < json.length; i++) {
                let rights = "";
                for (const key in json[i]) {
                    if (json[i].hasOwnProperty(key)) {
                        if (typeof json[i][key] === 'boolean' && json[i][key]) {
                            rights += key + ", ";
                        }
                    }
                }
                json[i].id = i + 1;
                json[i].rights = rights + "...";
            }
            roles.value.length = 0;
            roles.value.push(...json);
        })
        .catch((err) => { error.value = true; console.warn(err); });
};

defineExpose({ fetchRoles })

</script>

<style scoped>
.v-checkbox :deep(.v-label) {
    font-size: .8em;
}

</style>