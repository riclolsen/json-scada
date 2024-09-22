<template>
  <v-container fluid class="user-management-tab">

    <v-btn color="primary" class="mt-4" @click="openAddUserDialog">
      {{ $t('admin.userManagement.addUser') }}
    </v-btn>

    <v-data-table :headers="headers" :items="users" :items-per-page="5" class="elevation-1" :load-children="fetchUsers"
      :items-per-page-text="$t('common.itemsPerPageText')">
      <template #[`item.actions`]="{ item }">
        <v-icon size="small" class="me-2" @click="openEditUserDialog(item)">
          mdi-pencil
        </v-icon>
        <v-icon v-if="item.username !== 'admin'" size="small" @click="openDeleteConfirmDialog(item)">
          mdi-delete
        </v-icon>
      </template>
    </v-data-table>
    <div>
      <v-chip v-if="error" color="red darken-1">{{ $t('common.error') }}</v-chip>
    </div>
  </v-container>


  <v-dialog v-model="addUserDialog" max-width="500px">
    <v-card>
      <v-card-title>{{ $t('admin.userManagement.addNewUser') }}</v-card-title>
      <v-card-text>
        <v-text-field v-model="newUser.username" :label="$t('admin.userManagement.username')" required></v-text-field>
        <v-text-field v-model="newUser.email" :label="$t('admin.userManagement.email')" required
          type="email"></v-text-field>
        <v-text-field v-model="newUser.password" :label="$t('admin.userManagement.password')" required
          type="password"></v-text-field>
        <v-autocomplete v-model="newUser.roles" :items="roles" item-title="name" outlined chips closable-chips
          small-chips :label="$t('admin.userManagement.roles')" multiple></v-autocomplete>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn color="blue darken-1" text @click="closeAddUserDialog">{{ $t('common.cancel') }}</v-btn>
        <v-btn color="blue darken-1" text @click="createUser">{{ $t('common.save') }}</v-btn>
        <v-chip v-if="error" color="red darken-1">{{ $t('common.error') }}</v-chip>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="editUserDialog" max-width="500px">
    <v-card>
      <v-card-title>{{ $t('admin.userManagement.editUser') }}</v-card-title>
      <v-card-text>
        <v-text-field v-model="editedUser.username" :label="$t('admin.userManagement.username')"
          required></v-text-field>
        <v-text-field v-model="editedUser.email" :label="$t('admin.userManagement.email')" required
          type="email"></v-text-field>
        <v-text-field v-model="editedUser.password" :label="$t('admin.userManagement.password')" required
          type="password"></v-text-field>
        <v-autocomplete v-model="editedUser.roles" :items="roles" item-title="name" outlined chips closable-chips
          small-chips :label="$t('admin.userManagement.roles')" multiple></v-autocomplete>
      </v-card-text>
      <v-card-actions>
        <v-spacer> </v-spacer>
        <v-btn color="blue darken-1" text @click="closeEditUserDialog">{{ $t('common.cancel') }}</v-btn>
        <v-btn color="blue darken-1" text @click="updateUser(editedUser)">{{ $t('common.save') }}</v-btn>
        <v-chip v-if="error" color="red darken-1">{{ $t('common.error') }}</v-chip>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="deleteConfirmDialog" max-width="400px">
    <v-card>
      <v-card-title>{{ $t('admin.userManagement.confirmDelete') }}</v-card-title>
      <v-card-text>
        {{ $t('admin.userManagement.deleteConfirmMessage') }}
      </v-card-text>
      <v-card-text>
        {{ userToDelete.username }}
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn color="blue darken-1" text @click="closeDeleteConfirmDialog">{{ $t('common.cancel') }}</v-btn>
        <v-btn color="red darken-1" text @click="deleteUser(userToDelete)">{{ $t('common.delete') }}</v-btn>
      </v-card-actions>
      <v-chip v-if="error" color="red darken-1">{{ $t('common.error') }}</v-chip>
    </v-card>
  </v-dialog>

</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useI18n } from 'vue-i18n';

onMounted(async () => {
  await fetchUsers();
  await fetchRoles();
  document.documentElement.style.overflowY = 'scroll';
});

onUnmounted(async () => {
  document.documentElement.style.overflowY = 'auto';
});

const { t } = useI18n();

const headers = computed(() => [
  { title: '#', key: 'id' },
  { title: t('admin.userManagement.headers.username'), align: 'start', key: 'username' },
  { title: t('admin.userManagement.headers.email'), key: 'email' },
  { title: t('admin.userManagement.headers.roles'), key: 'rolesText' },
  { title: t('admin.userManagement.headers.actions'), key: 'actions', sortable: false },
]);

const users = ref([]);
const roles = ref([]);
const error = ref(false);

const addUserDialog = ref(false);
const newUser = ref({
  username: '',
  email: '',
  password: '',
  roles: [],
});

const deleteConfirmDialog = ref(false);
const userToDelete = ref({});

const openDeleteConfirmDialog = (user) => {
  error.value = false;
  userToDelete.value = user;
  deleteConfirmDialog.value = true;
};

const closeDeleteConfirmDialog = () => {
  error.value = false;
  userToDelete.value = {};
  deleteConfirmDialog.value = false;
};

const openAddUserDialog = () => {
  error.value = false;
  addUserDialog.value = true;
  editedUserRoles.value = [];
  newUser.value.roles = [];
  newUser.value.password = "";
  newUser.value.email = "";
  newUser.value.username = "";
};

const editUserDialog = ref(false);
const editedUser = ref({
  username: '',
  email: '',
  password: '',
  roles: [],
});
const editedUserRoles = ref([]);

const openEditUserDialog = (user) => {
  error.value = false;
  editedUserRoles.value = user.roles.map(role => role.name);
  editedUser.value = user;
  editUserDialog.value = true;
  editedUser.value.password = "";
};

const closeEditUserDialog = () => {
  error.value = false;
  editedUser.value = {
    username: '',
    email: '',
    password: '',
  };
  editUserDialog.value = false;
};

const closeAddUserDialog = () => {
  error.value = false;
  addUserDialog.value = false;
  newUser.value = {
    username: '',
    email: '',
    password: '',
  };
};

const deleteUser = async (user) => {
  error.value = false;
  if (user.username === "admin") {
    return;
  }
  return await fetch("/Invoke/auth/deleteUser", {
    method: "post",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      username: user.username,
      _id: user._id,
    }),
  })
    .then((res) => res.json())
    .then((json) => {
      fetchUsers(); // refreshes users
      closeDeleteConfirmDialog();
    })
    .catch((err) => { error.value = true; console.warn(err); });
};

const fetchUsers = async () => {
  error.value = false;
  return await fetch("/Invoke/auth/listUsers")
    .then((res) => res.json())
    .then((json) => {
      for (let i = 0; i < json.length; i++) {
        json[i].id = i + 1;
        json[i].rolesText = json[i].roles.map(role => role.name).join(', ');
      }
      users.value.length = 0;
      users.value.push(...json);
    })
    .catch((err) => { error.value = true; console.warn(err); });
}

const fetchRoles = async () => {
  error.value = false;
  return await fetch("/Invoke/auth/listRoles")
    .then((res) => res.json())
    .then((json) => {
      for (let i = 0; i < json.length; i++)
        json[i].id = i + 1;
      roles.value.length = 0;
      roles.value.push(...json);
    })
    .catch((err) => { error.value = true; console.warn(err); });
}

const updateUser = async (user) => {
  error.value = false;
  if (user.value) user = user.value;
  roleChange(user);
  const userDup = Object.assign({}, user);
  delete userDup["id"];
  delete userDup["rolesText"];
  delete userDup["roles"];
  delete userDup["__v"];
  if ("password" in userDup)
    if (userDup.password === "" || userDup.password === null)
      delete userDup["password"];
  return await fetch("/Invoke/auth/updateUser", {
    method: "post",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(userDup),
  })
    .then((res) => res.json())
    .then((json) => {
      if (json.error === false) {
        closeEditUserDialog();
      } else {
        error.value = true;
      }
      fetchUsers(); // refreshes users
    })
    .catch((err) => { error.value = true; console.warn(err); });
}

const addRoleToUser = async (username, roleName) => {
  return await fetch("/Invoke/auth/userAddRole", {
    method: "post",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      username: username,
      role: roleName,
    }),
  })
    .then((res) => res.json())
    .then((json) => {
      // fetchUsers(); // refreshes users
    })
    .catch((err) => { error.value = true; console.warn(err); });
}

const removeRoleFromUser = async (username, roleName) => {
  if (username === "admin" && roleName === "admin") {
    return;
  }
  return await fetch("/Invoke/auth/userRemoveRole", {
    method: "post",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      username: username,
      role: roleName,
    }),
  })
    .then((res) => res.json())
    .then((json) => {
      // fetchUsers(); // refreshes users
    })
    .catch((err) => { error.value = true; console.warn(err); });
}

const roleChange = (user) => {
  for (let i = 0; i < user.roles.length; i++) {
    const roleName = user.roles[i]?.name || user.roles[i];
    if (!editedUserRoles.value.includes(roleName))
      addRoleToUser(user.username, roleName);
  }

  for (let i = 0; i < editedUserRoles.value.length; i++) {
    const roleName = editedUserRoles.value[i]?.name || editedUserRoles.value[i];
    if (!user.roles.includes(roleName))
      removeRoleFromUser(user.username, roleName);
  }
}

const createUser = async () => {
  if (newUser.value.username === "admin") {
    return;
  }
  return await fetch("/Invoke/auth/createUser", {
    method: "post",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ username: newUser.value.username }),
  })
    .then((res) => res.json())
    .then(async (json) => {
      if (json.error === false) {
        await fetchUsers(); // refreshes users
        for (let i = 0; i < users.value.length; i++) {
          if (users.value[i].username === newUser.value.username) {
            newUser.value._id = users.value[i]._id;
            await updateUser(newUser);
            break;
          }
        }
        closeAddUserDialog();
      } else {
        error.value = true;
      }
    })
    .catch((err) => { error.value = true; console.warn(err); });
}
</script>
