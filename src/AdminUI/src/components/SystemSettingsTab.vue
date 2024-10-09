<template>
  <div class="system-settings-tab">

    <v-card class="mb-4" max-width="500">
      <v-card-title>
        {{ $t('admin.systemSettings.title') }}
      </v-card-title>
      <v-card-text>

        <v-form @submit.prevent="saveProject">

          <v-text-field v-model="settings.projectName" :label="$t('admin.systemSettings.projectName')"></v-text-field>

          <v-btn type="submit" color="primary" variant="tonal" class="mt-4">
            {{ $t('admin.systemSettings.saveProject') }}
          </v-btn>

        </v-form>

      </v-card-text>
    </v-card>

  </div>
</template>

<script setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const error = ref(false)
const settings = ref({
  projectName: 'jsonscada_project',
});

const saveProject = async () => {
  try {
    const response = await fetch("/Invoke/auth/saveProject", {
      method: "post",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        project: {
          fileName: settings.value.projectName + '.zip',
        }
      }),
    });
    const json = await response.json();
    if (json.error) { setError(json); return; }
  }
  catch (err) {
    setError(err)
  }
};

const setError = (err) => {
  error.value = true
  console.warn(err);
  setTimeout(() => { error.value = false }, 2000)
}

</script>