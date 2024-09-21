<template>
  <div class="user-management-tab">
    <h2 class="text-h5 mb-4">{{ $t('admin.userManagement.title') }}</h2>

    <v-data-table
      :headers="headers"
      :items="users"
      :items-per-page="10"
      class="elevation-1"
      :load-children="fetchUsers"
      :items-per-page-text="$t('common.itemsPerPageText')"
    >
      <template #[`item.actions`]="{ item }">
        <v-icon size="small" class="me-2" @click="openEditUserDialog(item)">
          mdi-pencil
        </v-icon>
        <v-icon size="small" @click="deleteUser(item)">
          mdi-delete
        </v-icon>
      </template>
    </v-data-table>

    <v-btn color="primary" class="mt-4" @click="openAddUserDialog">
      {{ $t('admin.userManagement.addUser') }}
    </v-btn>

    <v-dialog v-model="addUserDialog" max-width="500px">
      <v-card>
        <v-card-title>{{ $t('admin.userManagement.addNewUser') }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="newUser.username" :label="$t('admin.userManagement.username')" required></v-text-field>
          <v-text-field v-model="newUser.email" :label="$t('admin.userManagement.email')" required type="email"></v-text-field>
          <v-text-field v-model="newUser.password" :label="$t('admin.userManagement.password')" required type="password"></v-text-field>
          <v-text-field v-model="newUser.roles" :label="$t('admin.userManagement.roles')" required type="text"></v-text-field>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="blue darken-1" text @click="closeAddUserDialog">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="blue darken-1" text @click="createUser">{{ $t('common.save') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="editUserDialog" max-width="500px">
      <v-card>
        <v-card-title>{{ $t('admin.userManagement.editUser') }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="editedUser.username" :label="$t('admin.userManagement.username')" required></v-text-field>
          <v-text-field v-model="editedUser.email" :label="$t('admin.userManagement.email')" required type="email"></v-text-field>
          <v-text-field v-model="editedUser.password" :label="$t('admin.userManagement.password')" required type="password"></v-text-field>
          <v-select
            v-model="editedUser.roles"
            :label="$t('admin.userManagement.roles')"
            :items="availableRoles"
            item-title="name"
            item-value="id"
            multiple
            chips
            closable-chips
          >
            <template v-slot:selection="{ item }">
              <v-chip
                :key="item.raw.id"
                closable
                @click:close="removeRole(item.raw)"
              >
                {{ item.raw.name }}
              </v-chip>
            </template>
          </v-select>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="blue darken-1" text @click="closeEditUserDialog">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="blue darken-1" text @click="updateUser">{{ $t('common.save') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';

onMounted(async () => {
  await fetchUsers();
});

const { t } = useI18n();

const headers = computed(() => [
  { title: t('admin.userManagement.headers.username'), align: 'start', key: 'username' },
  { title: t('admin.userManagement.headers.email'), key: 'email' },
  { title: t('admin.userManagement.headers.roles'), key: 'rolesText' },
  { title: t('admin.userManagement.headers.actions'), key: 'actions', sortable: false },
]);

const users = ref([]);

const addUserDialog = ref(false);
const newUser = ref({
  username: '',
  email: '',
  password: '',
});

const openAddUserDialog = () => {
  addUserDialog.value = true;
};  

const editUserDialog = ref(false);
const editedUser = ref({
  username: '',
  email: '',
  password: '',
});

const openEditUserDialog = (user) => {
  editedUser.value = user;
  editUserDialog.value = true;
};  

const closeEditUserDialog = () => {
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

const editUser = (user) => {
  // Implement edit user logic
  console.log('Edit user:', user);
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
          if (json.error) console.log(json);
          fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
};

const addUser = () => {
  // Implement add user logic
  console.log('Add new user');
};

const fetchUsers = async () => {
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
    .catch((err) => console.warn(err));
}
</script>

<script>

export default {

  data: () => ({
    dialog: false,
    active: [],
    open: [],
    users: [],
    roles: [],
  }),

  computed: {
    items() {
      return [
        {
          name: i18n.t("src\\components\\users.users"),
          children: this.users,
          roles: this.roles,
        },
      ];
    },
    selected() {
      if (!this.active.length) return undefined;

      const id = this.active[0];

      return this.users.find((user) => user.id === id);
    },
  },

  methods: {
    async addRoleToUser(evt, roleName) {
      if (this.selected.roles.some(e => e.name === roleName)) 
        return
 
      if (this.selected.roles.includes(roleName))
        return
      return await fetch("/Invoke/auth/userAddRole", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username: this.selected.username,
          role: roleName,
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
    },
    async removeRoleFromUser(evt, roleName) {
      return await fetch("/Invoke/auth/userRemoveRole", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username: this.selected.username,
          role: roleName,
        }),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
    },
    async fetchRoles() {
      return await fetch("/Invoke/auth/listRoles")
        .then((res) => res.json())
        .then((json) => {
          this.roles.length = 0;
          this.roles.push(...json);
        })
        .catch((err) => console.warn(err));
    },
    async updateUser() {
      var userDup = Object.assign({}, this.selected);
      delete userDup["id"];
      if ("password" in userDup)
        if (userDup.password === "" || userDup.password === null)
          delete userDup["password"];
      this.selected.password = "";
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
          if (json.error) console.log(json);
          this.fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
    },
    async createUser() {
      return await fetch("/Invoke/auth/createUser", {
        method: "post",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({username: i18n.t("src\\components\\users.newUserUsername")}),
      })
        .then((res) => res.json())
        .then((json) => {
          if (json.error) console.log(json);
          this.fetchUsers(); // refreshes users
        })
        .catch((err) => console.warn(err));
    },
  },
};
  </script>