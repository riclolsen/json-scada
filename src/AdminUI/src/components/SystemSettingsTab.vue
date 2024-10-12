<template>
  <v-container fluid class="system-settings-tab ma-0 pa-0">
    <v-row class="fill-height" justify="center">
      <v-col cols="12" sm="8" md="6" lg="4">
        <v-card class="mb-4" min-width="500" max-width="700">
          <v-card-title class="mb-8">
            {{ $t('admin.systemSettings.title') }}
          </v-card-title>
          <v-card-text>
            <v-expansion-panels>
              <v-expansion-panel
                :title="$t('admin.systemSettings.exportProject')"
              >
                <v-expansion-panel-text>
                  <v-form @submit.prevent="exportProject">
                    <v-row>
                      <v-col>
                        <v-text-field
                          v-model="projectName"
                          :label="$t('admin.systemSettings.projectName')"
                        ></v-text-field>
                      </v-col>
                      <v-col>
                        <v-btn
                          type="submit"
                          color="primary"
                          variant="tonal"
                          class="mt-4"
                          :disabled="!projectName"
                        >
                          {{ $t('admin.systemSettings.exportProject') }}
                        </v-btn>
                      </v-col>
                    </v-row>
                  </v-form>
                </v-expansion-panel-text>
              </v-expansion-panel>

              <v-expansion-panel
                :title="$t('admin.systemSettings.importProject')"
              >
                <v-expansion-panel-text>
                  <v-form @submit.prevent="importProject" class="mt-8">
                    <v-file-input
                      v-model="file"
                      :label="$t('admin.systemSettings.uploadFile')"
                      accept=".zip"
                      @change="handleFileUpload"
                    ></v-file-input>

                    <v-row>
                      <v-col>
                        <v-sheet
                          class="pa-8 mb-4"
                          color="secondary"
                          rounded
                          outlined
                          :class="{ dragover: isDragging }"
                          @dragenter.prevent="isDragging = true"
                          @dragleave.prevent="isDragging = false"
                          @dragover.prevent
                          @drop.prevent="handleDrop"
                          accept=".zip"
                        >
                          <div v-if="!file">
                            {{ $t('admin.systemSettings.dragDropHere') }}
                          </div>
                          <div v-else>
                            {{ file.name }}
                          </div>
                        </v-sheet>
                      </v-col>

                      <v-col>
                        <v-btn
                          type="submit"
                          color="primary"
                          variant="tonal"
                          class="mt-4"
                          :disabled="!file"
                        >
                          {{ $t('admin.systemSettings.importProject') }}
                        </v-btn>
                      </v-col>
                    </v-row>

                    <v-alert v-if="fileError" type="error" class="mt-4">
                      {{ fileError }}
                    </v-alert>

                    <v-label v-if="importSuccessful" class="mt-4">
                      {{ $t('admin.systemSettings.importSuccessful') }}
                    </v-label>
                  </v-form>
                </v-expansion-panel-text>
              </v-expansion-panel>

              <v-expansion-panel
                :title="$t('admin.systemSettings.advancedOptions')"
              >
                <v-expansion-panel-text>
                  <v-row class="mt-4">
                    <v-btn
                      color="red"
                      variant="tonal"
                      @click="showRestartConfirmation"
                    >
                      {{ $t('admin.systemSettings.restartProcesses') }}
                    </v-btn>
                  </v-row>
                  <v-row class="my-8">
                    <v-btn
                      color="red"
                      variant="tonal"
                      @click="sanitizeDatabase"
                      disabled
                    >
                      {{ $t('admin.systemSettings.sanitizeDatabase') }}
                    </v-btn>
                  </v-row>
                </v-expansion-panel-text>
              </v-expansion-panel>
            </v-expansion-panels>
          </v-card-text>
        </v-card>
      </v-col>
    </v-row>

    <v-dialog v-model="showRestartDialog" max-width="400">
      <v-card>
        <v-card-title>{{
          $t('admin.systemSettings.restartProcesses')
        }}</v-card-title>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="orange" text @click="showRestartDialog = false">
            {{ $t('common.cancel') }}
          </v-btn>
          <v-btn color="red" @click="confirmRestart">
            {{ $t('common.confirm') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-container>
</template>

<script setup>
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'

  const { t } = useI18n()

  const error = ref(false)
  const importSuccessful = ref(false)
  const fileError = ref('')
  const projectName = ref('jsonscada_project')
  const file = ref(null)
  const isDragging = ref(false)
  const showRestartDialog = ref(false)

  const isZipFile = (fileName) => {
    return fileName.toLowerCase().endsWith('.zip')
  }

  const handleFileUpload = (event) => {
    if (event && event.target && event.target.files.length > 0) {
      const selectedFile = event.target.files[0]
      if (isZipFile(selectedFile.name)) {
        file.value = selectedFile
        fileError.value = ''
      } else {
        file.value = null
        fileError.value = t('admin.systemSettings.invalidFileType')
      }
    }
  }

  const handleDrop = (event) => {
    isDragging.value = false
    if (event.dataTransfer.files.length > 0) {
      const droppedFile = event.dataTransfer.files[0]
      if (isZipFile(droppedFile.name)) {
        file.value = droppedFile
        fileError.value = ''
      } else {
        file.value = null
        fileError.value = t('admin.systemSettings.invalidFileType')
      }
    }
  }

  const importProject = async () => {
    importSuccessful.value = false
    try {
      const formData = new FormData()
      formData.append('projectFileName', file.value.name)
      formData.append('projectFileData', file.value)

      const response = await fetch('/Invoke/auth/importProject', {
        method: 'POST',
        body: formData,
      })
      const json = await response.json()
      if (json.error) {
        setError(json)
        return
      }
      importSuccessful.value = true
      fileError.value = ''
    } catch (err) {
      setError(err)
    }
  }

  const exportProject = async () => {
    try {
      await fetch('/Invoke/auth/exportProject', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          project: {
            fileName: projectName.value + '.zip',
          },
        }),
      })
        .then((response) => response.blob()) // Get the response as a Blob
        .then((blob) => {
          // Create a temporary URL for the Blob
          const url = URL.createObjectURL(blob)

          // Create a link and set its href to the temporary URL
          const link = document.createElement('a')
          link.href = url

          // Set the link attributes for downloading
          link.setAttribute('download', projectName.value + '.zip')

          // Programmatically click the link to initiate the download
          link.click()

          // Clean up the temporary URL
          URL.revokeObjectURL(url)
        })
        .catch((err) => {
          setError(err)
        })

      //const json = await response.json();
      //if (json.error) { setError(json); return; }
    } catch (err) {
      setError(err)
    }
  }

  const showRestartConfirmation = () => {
    showRestartDialog.value = true
  }

  const confirmRestart = async () => {
    showRestartDialog.value = false
    await restartProcesses()
  }

  const restartProcesses = async () => {
    try {
      const response = await fetch('/Invoke/auth/restartProcesses', {
        method: 'POST',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
      })
      const json = await response.json()
      if (json.error) {
        setError(json)
        return
      }
      console.log('System restart initiated')
    } catch (err) {
      setError(err)
    }
  }

  const sanitizeDatabase = async () => {
    try {
      const response = await fetch('/Invoke/auth/sanitizeDatabase', {
        method: 'POST',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
      })
      const json = await response.json()
      if (json.error) {
        setError(json)
        return
      }
      console.log('Sanitization initiated')
    } catch (err) {
      setError(err)
    }
  }

  const setError = (err) => {
    error.value = true
    console.warn(err)
    setTimeout(() => {
      error.value = false
    }, 2000)
  }
</script>

<style scoped>
  .dragover {
    background-color: #f0f0f0;
    border: 2px dashed #ccc;
  }
</style>
