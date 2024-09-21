<template>
  <v-container fluid class="user-management-tab">
    <h2 class="text-h5 mb-4">{{ $t('admin.userManagement.title') }}</h2>

    <v-data-table :headers="headers" :items="users" :items-per-page="5" class="elevation-1" :load-children="fetchUsers"
      :items-per-page-text="$t('common.itemsPerPageText')">
      <template #[`item.actions`]="{ item }">
        <v-icon size="small" class="me-2" @click="openEditUserDialog(item)">
          mdi-pencil
        </v-icon>
        <v-icon v-if="item.username !== 'admin'" size="small" @click="deleteUser(item)">
          mdi-delete
        </v-icon>
      </template>
    </v-data-table>

    <v-btn color="primary" class="mt-4" @click="openAddUserDialog">
      {{ $t('admin.userManagement.addUser') }}
    </v-btn>
    <div>
      <v-chip v-if="error" color="red darken-1">{{ $t('common.error') }}</v-chip>
    </div>

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
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';

onMounted(async () => {
  await fetchUsers();
  await fetchRoles();
});

const { t } = useI18n();

const headers = computed(() => [
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
  addUserDialog.value = false;
  newUser.value = {
    username: '',
    email: '',
    password: '',
  };
};

const deleteUser = async (user) => {
  // Implement delete user logic
  console.log('Delete user:', user);
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
  roleChange(user);
  var userDup = Object.assign({}, user);
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
      fetchUsers(); // refreshes users
      closeEditUserDialog();
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
  for (let i = 0; i < user.roles.length; i++)
    if (!editedUserRoles.value.includes(user.roles[i]))
      addRoleToUser(user.username, user.roles[i]);
  for (let i = 0; i < editedUserRoles.value.length; i++)
    if (!user.roles.includes(editedUserRoles.value[i]))
      removeRoleFromUser(user.username, editedUserRoles.value[i]);
}

const createUser = async () => {
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
      await fetchUsers(); // refreshes users
      for (let i = 0; i < users.value.length; i++) {
        if (users.value[i].username === newUser.value.username) {
          newUser.value._id = users.value[i]._id;
          await updateUser(newUser.value);
          break;
        }
      }
      closeAddUserDialog();
    })
    .catch((err) => { error.value = true; console.warn(err); });
}
</script>
