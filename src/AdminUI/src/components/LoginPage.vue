<template>
  <v-container class="fill-height" fluid>
    <v-row align="center" justify="center">
      <v-col cols="12" sm="8" md="6" lg="4">
        <v-card class="elevation-12">
          <v-toolbar color="primary" dark flat>
            <v-toolbar-title>{{ $t('login.title') }}</v-toolbar-title>
          </v-toolbar>
          <v-card-text>
            <v-form @submit.prevent="login">
              <v-text-field v-model="username" :label="$t('login.username')" name="username" prepend-icon="mdi-account"
                type="text" required ref="usernameField" autofocus></v-text-field>
              <v-text-field v-model="password" :label="$t('login.password')" name="password" prepend-icon="mdi-lock"
                type="password" required></v-text-field>
              <v-alert v-if="errorMessage" type="error" dense>
                {{ errorMessage }}
              </v-alert>
            </v-form>
          </v-card-text>
          <v-card-actions>
            <v-spacer></v-spacer>
            <v-btn color="primary" variant="tonal" @click="login">{{
              $t('login.submit')
            }}</v-btn>
          </v-card-actions>
        </v-card>
      </v-col>
    </v-row>

  </v-container>
</template>

<script setup>
import { ref, inject, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const { t } = useI18n()
const setLoggedInUser = inject('setLoggedInUser')

const username = ref('')
const password = ref('')
const errorMessage = ref('')

onMounted(() => {
  testLogin()
})

const login = () => {
  fetch('/Invoke/auth/signin', {
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    method: 'POST',
    body: JSON.stringify({
      username: username.value,
      password: password.value,
    }),
  })
    .then((resp) => resp.json())
    .then((data) => {
      if ('ok' in data && data.ok == false) {
        errorMessage.value = t('login.errorMessage')
        return
      }
      setLoggedInUser(username.value)
      errorMessage.value = ""
      username.value = password.value = ""
      router.push('/dashboard')
    })
    .catch((error) => {
      console.log('Error on a fetch operation: ' + error.message)
    })
    .finally(() => {
      testLogin()
    })
}

const parseCookie = (str) => {
  if (str === '') return {}
  return str
    .split(';')
    .map((v) => v.split('='))
    .reduce((acc, v) => {
      acc[decodeURIComponent(v[0].trim())] = decodeURIComponent(v[1].trim())
      return acc
    }, {})
}

function testLogin() {
  let ck = parseCookie(document.cookie)
  if ('json-scada-user' in ck) {
    ck = JSON.parse(ck['json-scada-user'])
  }

  fetch('/Invoke/test/user', {
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    method: 'GET',
  })
    .then((resp) => resp.json())
    .then((data) => {
      if ('ok' in data && data.ok) {
        router.push('/dashboard')
        // console.log('User is logged in')
      } else {
        router.push('/login')
        // console.log('User is not logged in')
      }
    })
    .catch((error) => {
      router.push('/login')
      console.log(
        'There has been a problem with your fetch operation: ' + error.message
      )
    })
}

</script>
