<template>
  <div class="tags-tab">
    <div class="admin-toolbar">
      <v-btn color="primary" variant="flat" prepend-icon="mdi-plus" @click="newTagOpenDialog">
        {{ $t('admin.tags.newTag') }}
      </v-btn>
      <v-text-field
        v-model="searchTag"
        class="admin-search"
        density="compact"
        variant="outlined"
        hide-details
        clearable
        prepend-inner-icon="mdi-magnify"
        :label="$t('admin.tags.searchTag')"
        @keyup.enter="applyFilters"
        @click:clear="clearAndApply('searchTag')"
      ></v-text-field>
      <v-text-field
        v-model="searchDescription"
        class="admin-search"
        density="compact"
        variant="outlined"
        hide-details
        clearable
        prepend-inner-icon="mdi-text-search"
        :label="$t('admin.tags.searchDescription')"
        @keyup.enter="applyFilters"
        @click:clear="clearAndApply('searchDescription')"
      ></v-text-field>
      <v-btn
        variant="tonal"
        :color="activeFilterCount ? 'primary' : undefined"
        prepend-icon="mdi-filter-variant"
        @click="showFilters = !showFilters"
      >
        {{ $t('admin.common.filters') }}
        <v-badge v-if="activeFilterCount" :content="activeFilterCount" color="primary" inline></v-badge>
      </v-btn>
      <v-spacer></v-spacer>
      <span class="admin-count">{{ $t('admin.common.results', { n: totalTags.toLocaleString() }) }}</span>
      <v-btn icon variant="text" size="small" :title="$t('common.refresh')" @click="fetchTags">
        <v-icon>mdi-refresh</v-icon>
      </v-btn>
      <v-btn
        icon
        variant="text"
        size="small"
        :color="autoRefresh ? 'primary' : undefined"
        :title="$t('admin.common.autoRefresh')"
        @click="autoRefresh = !autoRefresh"
      >
        <v-icon>{{ autoRefresh ? 'mdi-timer-sync-outline' : 'mdi-timer-off-outline' }}</v-icon>
      </v-btn>
      <v-menu :close-on-content-click="false" location="bottom end">
        <template #activator="{ props }">
          <v-btn icon variant="text" size="small" :title="$t('admin.common.columns')" v-bind="props">
            <v-icon>mdi-view-column-outline</v-icon>
          </v-btn>
        </template>
        <v-list density="compact" min-width="220">
          <v-list-subheader>{{ $t('admin.common.columns') }}</v-list-subheader>
          <v-list-item v-for="c in optionalColumns" :key="c.key" @click="toggleColumn(c.key)">
            <template #prepend>
              <v-checkbox-btn :model-value="visibleColumns.includes(c.key)" density="compact"></v-checkbox-btn>
            </template>
            <v-list-item-title>{{ c.title }}</v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>
      <v-btn
        icon
        variant="text"
        size="small"
        :loading="exporting"
        :title="$t('admin.common.exportCsv')"
        @click="exportCsv"
      >
        <v-icon>mdi-file-delimited-outline</v-icon>
      </v-btn>
    </div>

    <v-expand-transition>
      <div v-show="showFilters" class="admin-filters">
        <v-select
          v-model="filterTypes"
          :items="typeItems"
          :label="$t('admin.tags.filterType')"
          multiple
          chips
          closable-chips
          clearable
          density="compact"
          variant="outlined"
          hide-details
          @update:model-value="applyFilters"
        ></v-select>
        <v-select
          v-model="filterOrigins"
          :items="originItems"
          :label="$t('admin.tags.filterOrigin')"
          multiple
          chips
          closable-chips
          clearable
          density="compact"
          variant="outlined"
          hide-details
          @update:model-value="applyFilters"
        ></v-select>
        <v-autocomplete
          v-model="filterConnections"
          :items="connectionItems"
          :label="$t('admin.tags.filterConnection')"
          multiple
          chips
          closable-chips
          clearable
          density="compact"
          variant="outlined"
          hide-details
          @update:model-value="applyFilters"
        ></v-autocomplete>
        <v-text-field
          v-model="filterGroup1"
          :label="$t('admin.tags.filterGroup1')"
          clearable
          density="compact"
          variant="outlined"
          hide-details
          @keyup.enter="applyFilters"
          @click:clear="clearAndApply('filterGroup1')"
        ></v-text-field>
        <v-select
          v-model="filterQuality"
          :items="qualityItems"
          :label="$t('admin.tags.filterQuality')"
          density="compact"
          variant="outlined"
          hide-details
          @update:model-value="applyFilters"
        ></v-select>
        <v-select
          v-model="filterAlarm"
          :items="alarmItems"
          :label="$t('admin.tags.filterAlarm')"
          density="compact"
          variant="outlined"
          hide-details
          @update:model-value="applyFilters"
        ></v-select>
        <div class="d-flex align-center">
          <v-btn variant="text" prepend-icon="mdi-filter-remove-outline" :disabled="!activeFilterCount" @click="clearFilters">
            {{ $t('admin.common.clearFilters') }}
          </v-btn>
        </div>
      </div>
    </v-expand-transition>

    <v-data-table-server
      :headers="visibleHeaders"
      :items="tags"
      :items-per-page="itemsPerPage"
      :page="page"
      :items-length="totalTags"
      :loading="loading"
      :items-per-page-options="[10, 25, 50, 100, 250, 500, 1000]"
      :items-per-page-text="$t('common.itemsPerPageText')"
      @update:options="handleOptionsUpdate"
      multi-sort
      hover
      class="admin-table"
      density="compact"
    >
      <template #[`item.tag`]="{ item }">
        <span class="admin-mono">{{ item.tag }}</span>
        <v-chip v-if="item.commStatus" size="x-small" label class="ms-1" :title="$t('admin.tags.systemTagHint')">
          {{ $t('admin.tags.system') }}
        </v-chip>
      </template>
      <template #[`item._id`]="{ item }">
        <span class="admin-mono">{{ item._id }}</span>
      </template>
      <template #[`item.protocolSourceConnectionNumber`]="{ item }">
        <span v-if="item.protocolSourceConnectionNumber" class="text-body-small">
          <span class="admin-mono">{{ item.protocolSourceConnectionNumber }}</span>
          <span v-if="connectionName(item.protocolSourceConnectionNumber)" class="text-medium-emphasis">
            · {{ connectionName(item.protocolSourceConnectionNumber) }}
          </span>
        </span>
        <span v-else class="text-disabled">—</span>
      </template>
      <template #[`item.type`]="{ item }">
        <v-chip size="x-small" label variant="tonal" :color="typeColor[item.type]">{{ item.type }}</v-chip>
      </template>
      <template #[`item.origin`]="{ item }">
        <span class="text-body-small">{{ item.origin }}</span>
      </template>
      <template #[`item.description`]="{ item }">
        <span class="text-body-small">{{ item.description }}</span>
      </template>
      <template #[`item.value`]="{ item }">
        <span class="admin-mono tag-value" :class="{ 'text-disabled': item.invalid }" :title="valueTitle(item)">
          {{ valueText(item) }}
        </span>
      </template>
      <template #[`item.invalid`]="{ item }">
        <div class="d-flex flex-wrap ga-1">
          <v-chip v-if="item.invalid" size="x-small" label color="error" variant="tonal">{{ $t('admin.tags.qualityInvalid') }}</v-chip>
          <v-chip v-else size="x-small" label color="success" variant="tonal">{{ $t('admin.tags.qualityValid') }}</v-chip>
          <v-chip v-if="item.alarmed" size="x-small" label color="warning" variant="tonal">{{ $t('admin.tags.alarmed') }}</v-chip>
          <v-chip v-if="item.frozen" size="x-small" label color="info" variant="tonal">{{ $t('admin.tags.frozen') }}</v-chip>
          <v-chip v-if="item.substituted" size="x-small" label color="purple" variant="tonal">{{ $t('admin.tags.substituted') }}</v-chip>
          <v-chip v-if="item.alarmDisabled" size="x-small" label variant="tonal">{{ $t('admin.tags.alarmDisabled') }}</v-chip>
        </div>
      </template>
      <template #[`item.timeTag`]="{ item }">
        <span class="text-body-small">{{ item.timeTag ? new Date(item.timeTag).toLocaleString() : '' }}</span>
      </template>
      <template #[`item.address`]="{ item }">
        <span class="admin-mono">{{ addressText(item) }}</span>
      </template>

      <template #[`item.Actions`]="{ item }">
        <div class="admin-row-actions">
          <v-btn icon variant="text" size="x-small" :title="$t('admin.tags.editTag')" @click="editTagOpenDialog(item)">
            <v-icon size="18">mdi-pencil-outline</v-icon>
          </v-btn>
          <v-btn
            v-if="item.type === 'analog'"
            icon
            variant="text"
            size="x-small"
            :title="$t('admin.common.openTrend')"
            :href="trendHref(item)"
            target="_blank"
          >
            <v-icon size="18">mdi-chart-line</v-icon>
          </v-btn>
          <v-menu location="bottom end">
            <template #activator="{ props }">
              <v-btn icon variant="text" size="x-small" v-bind="props"><v-icon size="18">mdi-dots-vertical</v-icon></v-btn>
            </template>
            <v-list density="compact">
              <v-list-item prepend-icon="mdi-content-duplicate" :disabled="!!item.commStatus" @click="cloneTagOpenDialog(item)">
                <v-list-item-title>{{ $t('admin.common.clone') }}</v-list-item-title>
              </v-list-item>
              <v-list-item prepend-icon="mdi-content-copy" @click="copyName(item.tag)">
                <v-list-item-title>{{ $t('admin.common.copyName') }}</v-list-item-title>
              </v-list-item>
              <v-list-item prepend-icon="mdi-chart-line" :href="trendHref(item)" target="_blank" :disabled="item.type !== 'analog'">
                <v-list-item-title>{{ $t('admin.common.openTrend') }}</v-list-item-title>
              </v-list-item>
              <v-divider></v-divider>
              <v-list-item
                prepend-icon="mdi-delete-outline"
                base-color="error"
                :disabled="!!item.commStatus"
                @click="deleteTagOpenDialog(item)"
              >
                <v-list-item-title>{{ $t('common.delete') }}</v-list-item-title>
              </v-list-item>
            </v-list>
          </v-menu>
        </div>
      </template>
    </v-data-table-server>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color" :timeout="3500" location="bottom right">
      {{ snackbar.text }}
    </v-snackbar>
  </div>

  <v-dialog scrollable v-model="dialogDeleteTag" max-width="500px">
    <v-card>
      <v-card-title class="headline">{{
        $t('admin.tags.confirmErase')
      }}</v-card-title>
      <v-card-text>
        <v-text-field
          density="compact"
          readonly
          clearable
          variant="outlined"
          v-model="editedTag._id"
          :label="$t('admin.tags.eraseId')"
        ></v-text-field>
        <v-spacer></v-spacer>
        <v-text-field
          density="compact"
          readonly
          variant="outlined"
          v-model="editedTag.tag"
          :label="$t('admin.tags.eraseName')"
        ></v-text-field>
      </v-card-text>
      <v-card-actions>
        <v-chip v-if="error" color="red darken-1">{{
          $t('common.error')
        }}</v-chip>
        <v-spacer></v-spacer>
        <v-btn
          color="orange darken-1"
          text
          variant="tonal"
          @click="closeDeleteTag"
          >{{ $t('common.cancel') }}</v-btn
        >
        <v-btn color="red darken-1" text variant="tonal" @click="deleteTag">{{
          $t('common.delete')
        }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog scrollable v-model="dialogEditTag" max-width="750px">
    <v-card>
      <v-card-title>
        <span class="headline">{{
          isNewTag ? $t('admin.tags.newTag') : $t('admin.tags.editTag')
        }}</span>
      </v-card-title>

      <v-card-text>
        <v-text-field
          type="number"
          variant="outlined"
          v-model="editedTag._id"
          :label="$t('admin.tags.editId')"
          :disabled="!isNewTag"
          min="1"
        ></v-text-field>

        <v-text-field
          clearable
          type="text"
          variant="outlined"
          v-model="editedTag.tag"
          :label="$t('admin.tags.editName')"
        ></v-text-field>

        <v-select
          variant="outlined"
          :items="['supervised', 'command', 'calculated', 'manual']"
          :label="$t('admin.tags.editOrigin')"
          v-model="editedTag.origin"
          class="ma-0"
        ></v-select>

        <v-select
          variant="outlined"
          :items="['digital', 'analog', 'string', 'json']"
          :label="$t('admin.tags.editType')"
          v-model="editedTag.type"
          class="ma-0"
        ></v-select>

        <v-expansion-panels variant="inset">
          <v-expansion-panel :title="$t('admin.tags.editDescription')">
            <v-expansion-panel-text>
              <v-text-field
                variant="outlined"
                clearable
                v-model="editedTag.description"
                :label="$t('admin.tags.editDescription')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                clearable
                v-model="editedTag.ungroupedDescription"
                :label="$t('admin.tags.editUngroupedDescription')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                clearable
                v-model="editedTag.group1"
                :label="$t('admin.tags.editGroup1')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                clearable
                v-model="editedTag.group2"
                :label="$t('admin.tags.editGroup2')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                clearable
                v-model="editedTag.group3"
                :label="$t('admin.tags.editGroup3')"
              ></v-text-field>

              <v-text-field
                variant="outlined"
                clearable
                v-if="editedTag.type == 'digital'"
                v-model="editedTag.stateTextFalse"
                :label="$t('admin.tags.editStateTextFalse')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                clearable
                v-if="editedTag.type == 'digital'"
                v-model="editedTag.stateTextTrue"
                :label="$t('admin.tags.editStateTextTrue')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                clearable
                v-if="editedTag.type == 'digital'"
                v-model="editedTag.eventTextFalse"
                :label="$t('admin.tags.editEventTextFalse')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                clearable
                v-if="editedTag.type == 'digital'"
                v-model="editedTag.eventTextTrue"
                :label="$t('admin.tags.editEventTextTrue')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                clearable
                v-if="editedTag.type == 'analog'"
                v-model="editedTag.unit"
                :label="$t('admin.tags.editUnit')"
              ></v-text-field>

              <CalculatedPointEditor
                v-if="editedTag.origin === 'calculated'"
                v-model:formula="editedTag.formula"
                v-model:parcels="editedTag.parcels"
                :self-key="editedTag._id"
              />
            </v-expansion-panel-text>
          </v-expansion-panel>
          <v-expansion-panel :title="$t('admin.tags.sectionProtocol')">
            <v-expansion-panel-text>
              <v-select
                variant="outlined"
                :items="protocolConnections"
                :label="$t('admin.tags.editProtocolSourceConnectionNumber')"
                :item-title="
                  (item) => `${item.protocolConnectionNumber} | ${item.name}`
                "
                :item-value="(item) => item.protocolConnectionNumber"
                v-if="['supervised', 'command'].includes(editedTag.origin)"
                v-model="editedTag.protocolSourceConnectionNumber"
                class="ma-0"
              ></v-select>

              <v-text-field
                variant="outlined"
                v-if="['supervised', 'command'].includes(editedTag.origin)"
                v-model="editedTag.protocolSourceASDU"
                :label="$t('admin.tags.editProtocolSourceASDU')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                v-if="['supervised', 'command'].includes(editedTag.origin)"
                v-model="editedTag.protocolSourceCommonAddress"
                :label="$t('admin.tags.editProtocolSourceCommonAddress')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                v-if="['supervised', 'command'].includes(editedTag.origin)"
                v-model="editedTag.protocolSourceObjectAddress"
                :label="$t('admin.tags.editProtocolSourceObjectAddress')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                type="number"
                v-if="['supervised'].includes(editedTag.origin)"
                v-model="editedTag.protocolSourcePublishingInterval"
                :label="$t('admin.tags.editProtocolSourcePublishingInterval')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                type="number"
                v-if="['supervised'].includes(editedTag.origin)"
                v-model="editedTag.protocolSourceSamplingInterval"
                :label="$t('admin.tags.editProtocolSourceSamplingInterval')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                type="number"
                v-if="['supervised'].includes(editedTag.origin)"
                v-model="editedTag.protocolSourceQueueSize"
                :label="$t('admin.tags.editProtocolSourceQueueSize')"
              ></v-text-field>
              <v-switch
                v-if="['supervised'].includes(editedTag.origin)"
                v-model="editedTag.protocolSourceDiscardOldest"
                inset
                color="primary"
                :label="`${$t('admin.tags.editProtocolSourceDiscardOldest')}${
                  editedTag.protocolSourceDiscardOldest
                    ? $t('common.true')
                    : $t('common.false')
                }`"
                class="mt-0"
              ></v-switch>

              <v-text-field
                variant="outlined"
                type="number"
                v-if="['command'].includes(editedTag.origin)"
                min="0"
                v-model="editedTag.protocolSourceCommandDuration"
                :label="$t('admin.tags.editProtocolSourceCommandDuration')"
              ></v-text-field>

              <v-switch
                v-if="
                  ['command'].includes(editedTag.origin) &&
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
                    '4',
                  ].includes('' + editedTag.protocolSourceASDU)
                "
                v-model="editedTag.protocolSourceCommandUseSBO"
                inset
                color="primary"
                :label="`${$t('admin.tags.editProtocolSourceCommandUseSBO')}${
                  editedTag.protocolSourceCommandUseSBO
                    ? $t('admin.tags.editProtocolSourceCommandUseSBOTrue')
                    : $t('admin.tags.editProtocolSourceCommandUseSBOFalse')
                }`"
                class="mt-0"
              ></v-switch>

              <v-text-field
                variant="outlined"
                type="number"
                v-if="['supervised', 'command'].includes(editedTag.origin)"
                v-model="editedTag.kconv1"
                :label="$t('admin.tags.editKconv1')"
              ></v-text-field>
              <v-text-field
                variant="outlined"
                type="number"
                v-if="['supervised', 'command'].includes(editedTag.origin)"
                v-model="editedTag.kconv2"
                :label="$t('admin.tags.editKconv2')"
              ></v-text-field>

              <v-row>
                <v-col>
                  <v-select
                    variant="outlined"
                    v-model="editedTag.protocolDestinations"
                    :items="editedTag.protocolDestinations"
                    :item-title="
                      (item) => `
                            CN:${item.protocolDestinationConnectionNumber} | 
                            OA:${item.protocolDestinationObjectAddress} |
                            CA:${item.protocolDestinationCommonAddress} | 
                            AS:${item.protocolDestinationASDU} 
                            `
                    "
                    :item-value="(item) => item"
                    chips
                    small-chips
                    closable-chips
                    :label="$t('admin.tags.protocolDestinations')"
                    multiple
                  ></v-select>
                </v-col>
                <v-col>
                  <v-btn
                    color="blue darken-1"
                    text
                    variant="tonal"
                    @click="dialogAddProtocolDestination = true"
                  >
                    <v-icon dark> mdi-plus </v-icon>
                    {{ $t('admin.tags.protocolDestinationAddNew') }}
                  </v-btn>
                </v-col>
              </v-row>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <v-expansion-panel :title="$t('admin.tags.sectionOtherSettings')">
            <v-expansion-panel-text>
              <v-text-field
                variant="outlined"
                type="number"
                v-model="editedTag.priority"
                min="0"
                max="3"
                :label="$t('admin.tags.editPriority')"
              ></v-text-field>

              <v-select
                variant="outlined"
                v-if="editedTag.type == 'digital'"
                :items="alarmStateItems"
                item-title="name"
                item-value="value"
                :label="$t('admin.tags.editAlarmState')"
                v-model="editedTag.alarmState"
                class="ma-0"
              ></v-select>

              <v-switch
                v-if="['supervised'].includes(editedTag.origin)"
                v-model="editedTag.isEvent"
                inset
                color="primary"
                :label="`${$t('admin.tags.editIsEvent')}${
                  editedTag.isEvent
                    ? $t('admin.tags.editIsEventTrue')
                    : $t('admin.tags.editIsEventFalse')
                }`"
                class="mt-0"
              ></v-switch>

              <v-text-field
                variant="outlined"
                type="number"
                v-if="['supervised'].includes(editedTag.origin)"
                v-model="editedTag.commandOfSupervised"
                :label="$t('admin.tags.editCommandOfSupervised')"
              ></v-text-field>

              <v-text-field
                variant="outlined"
                type="number"
                v-if="['command'].includes(editedTag.origin)"
                v-model="editedTag.supervisedOfCommand"
                :label="$t('admin.tags.editSupervisedOfCommand')"
              ></v-text-field>

              <v-text-field
                variant="outlined"
                type="number"
                v-if="['supervised'].includes(editedTag.origin)"
                v-model="editedTag.invalidDetectTimeout"
                :label="$t('admin.tags.editInvalidDetectTimeout')"
              ></v-text-field>

              <v-text-field
                variant="outlined"
                type="number"
                v-if="editedTag.type === 'analog'"
                v-model="editedTag.frozenDetectTimeout"
                :label="$t('admin.tags.editFrozenDetectTimeout')"
              ></v-text-field>

              <!-- out-of-range alarm limits (empty = no limit) -->
              <template v-if="editedTag.type === 'analog' && editedTag.origin !== 'command'">
                <v-row dense>
                  <v-col v-for="k in limitFieldsShown" :key="k" cols="6">
                    <v-text-field
                      variant="outlined"
                      type="number"
                      clearable
                      v-model="editedTag[k]"
                      :label="$t('admin.tags.edit' + k.charAt(0).toUpperCase() + k.slice(1))"
                    ></v-text-field>
                  </v-col>
                </v-row>
                <v-text-field
                  variant="outlined"
                  type="number"
                  min="0"
                  v-model="editedTag.hysteresis"
                  :label="$t('admin.tags.editHysteresis')"
                ></v-text-field>
              </template>

              <v-text-field
                variant="outlined"
                type="number"
                v-if="
                  editedTag.origin !== 'command' && editedTag.type === 'analog'
                "
                min="0"
                v-model="editedTag.historianDeadBand"
                :label="$t('admin.tags.editHistorianDeadBand')"
              ></v-text-field>

              <v-text-field
                variant="outlined"
                type="number"
                v-if="editedTag.origin !== 'command'"
                min="0"
                v-model="editedTag.historianPeriod"
                :label="$t('admin.tags.editHistorianPeriod')"
              ></v-text-field>
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>
      </v-card-text>

      <v-card-actions>
        <v-chip v-if="error" color="red darken-1">{{
          $t('common.error')
        }}</v-chip>
        <v-spacer></v-spacer>
        <v-btn
          color="orange darken-1"
          text
          variant="tonal"
          @click="closeEditTag"
        >
          {{ $t('common.cancel') }}
        </v-btn>
        <v-btn
          color="blue darken-1"
          text
          variant="tonal"
          @click="updateOrCreateTag"
        >
          {{ $t('common.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog
    scrollable
    v-model="dialogAddProtocolDestination"
    max-width="400"
    class="pa-8"
  >
    <v-card>
      <v-card-title class="headline">
        {{ $t('admin.tags.protocolDestinationAddNew') }}
      </v-card-title>

      <v-card-text>
        <v-select
          :items="protocolConnectionsDestinations"
          :label="$t('admin.tags.protocolDestinationConnectionNumber')"
          :item-title="
            (item) => `${item.protocolConnectionNumber} | ${item.name}`
          "
          :item-value="(item) => item.protocolConnectionNumber"
          v-model="newProtocolDestination.protocolDestinationConnectionNumber"
          class="ma-0"
        ></v-select>

        <v-text-field
          :label="$t('admin.tags.protocolDestinationCommonAddress')"
          v-model="newProtocolDestination.protocolDestinationCommonAddress"
        ></v-text-field>

        <v-text-field
          :label="$t('admin.tags.protocolDestinationObjectAddress')"
          v-model="newProtocolDestination.protocolDestinationObjectAddress"
        ></v-text-field>

        <v-text-field
          :label="$t('admin.tags.protocolDestinationASDU')"
          v-model="newProtocolDestination.protocolDestinationASDU"
        ></v-text-field>

        <v-text-field
          v-if="
            !isNaN(parseInt(newProtocolDestination.protocolDestinationASDU)) &&
            newProtocolDestination.protocolDestinationASDU >= 45 &&
            newProtocolDestination.protocolDestinationASDU <= 64
          "
          :label="$t('admin.tags.protocolDestinationCommandDuration')"
          type="number"
          min="0"
          v-model="newProtocolDestination.protocolDestinationCommandDuration"
        ></v-text-field>

        <v-switch
          v-if="
            newProtocolDestination.protocolDestinationASDU >= 45 &&
            newProtocolDestination.protocolDestinationASDU <= 64
          "
          v-model="newProtocolDestination.protocolDestinationCommandUseSBO"
          inset
          color="primary"
          :label="`${$t('admin.tags.protocolDestinationCommandUseSBO')}${
            newProtocolDestination.protocolDestinationCommandUseSBO
              ? ': ' + $t('common.true')
              : ': ' + $t('common.false')
          }`"
          class="mt-n3"
        ></v-switch>

        <v-text-field
          :label="$t('admin.tags.protocolDestinationKConv1')"
          type="number"
          v-model="newProtocolDestination.protocolDestinationKConv1"
        ></v-text-field>

        <v-text-field
          :label="$t('admin.tags.protocolDestinationKConv2')"
          type="number"
          v-model="newProtocolDestination.protocolDestinationKConv2"
        ></v-text-field>

        <v-text-field
          :label="$t('admin.tags.protocolDestinationGroup')"
          v-model="newProtocolDestination.protocolDestinationGroup"
        ></v-text-field>

        <v-text-field
          :label="$t('admin.tags.protocolDestinationHoursShift')"
          type="number"
          v-model="newProtocolDestination.protocolDestinationHoursShift"
        ></v-text-field>
      </v-card-text>

      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="orange darken-1"
          text
          variant="tonal"
          @click="dialogAddProtocolDestination = false"
        >
          {{ $t('common.cancel') }}
        </v-btn>
        <v-btn
          color="blue darken-1"
          text
          variant="tonal"
          @click="addNewProtocolDestination"
        >
          {{ $t('common.ok') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

</template>

<script setup>
  import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { postJson, fetchAllPages, toCsv, downloadText, fileStamp, loadPref, savePref, copyText } from '../lib/adminList'
  import CalculatedPointEditor from './CalculatedPointEditor.vue'

  const props = defineProps({
    active: { type: Boolean, default: false },
    // { n: <connection number> } from the Connections section
    connectionFilter: { type: Object, default: null },
  })
  const { t } = useI18n()

  const defaultTagValue = ref({
    tag: '',
    description: '',
    ungroupedDescription: '',
    group1: '',
    group2: '',
    group3: '',
    stateTextFalse: '',
    stateTextTrue: '',
    eventTextFalse: '',
    eventTextTrue: '',
    unit: '',
    origin: 'supervised',
    type: 'digital',
    commandOfSupervised: 0,
    supervisedOfCommand: 0,
    invalidDetectTimeout: 3600,
    frozenDetectTimeout: 0,
    formula: 0,
    priority: 0,
    parcels: [],
    protocolDestinations: [],
    protocolSourceASDU: '',
    protocolSourceCommonAddress: '',
    protocolSourceObjectAddress: '',
    protocolSourceCommandDuration: 0,
    protocolSourceCommandUseSBO: false,
    protocolSourcePublishingInterval: 5.0,
    protocolSourceSamplingInterval: 5.0,
    protocolSourceQueueSize: 5.0,
    protocolSourceDiscardOldest: true,
    historianDeadBand: 0,
    historianPeriod: 0,
    alarmState: 2,
    kconv1: 1,
    kconv2: 0,
    isEvent: false,
    location: {},
  })
  const editedTag = ref({ ...defaultTagValue.value })
  const dialogEditTag = ref(false)
  const dialogDeleteTag = ref(false)
  const isNewTag = ref(false)
  const tags = ref([])
  const protocolConnections = ref([])
  const protocolDriveNameByConnNumber = ref({})
  const protocolConnectionsDestinations = ref([])
  const totalTags = ref(0)
  const searchTag = ref('')
  const searchDescription = ref('')
  const loading = ref(true)
  const itemsPerPage = ref(loadPref('tags-page-size', 25))
  const page = ref(1)
  const sortBy = ref([])
  const dialogAddProtocolDestination = ref(false)
  const error = ref(false)
  const newProtocolDestination = ref({
    protocolDestinationConnectionNumber: 0,
    protocolDestinationCommonAddress: 0,
    protocolDestinationObjectAddress: 0,
    protocolDestinationASDU: 0,
    protocolDestinationCommandDuration: 0,
    protocolDestinationCommandUseSBO: false,
    protocolDestinationKConv1: 1,
    protocolDestinationKConv2: 0,
    protocolDestinationGroup: 0,
    protocolDestinationHoursShift: 0,
  })

  const alarmStateItems = ref([
    {
      value: 0,
      name: t('admin.tags.editAlarmStateOff'),
    },
    {
      value: 1,
      name: t('admin.tags.editAlarmStateOn'),
    },
    {
      value: 2,
      name: t('admin.tags.editAlarmStateBoth'),
    },
    {
      value: 3,
      name: t('admin.tags.editAlarmStateOffToOn'),
    },
    {
      value: -1,
      name: t('admin.tags.editAlarmStateNone'),
    },
  ])

  // columns: `optional` ones can be hidden from the column menu (choice kept
  // per browser); the tag name and actions are always shown
  const allHeaders = computed(() => [
    { title: t('admin.tags.headers.tag'), key: 'tag', sortable: true },
    { title: t('admin.tags.headers.id'), key: '_id', sortable: true, align: 'end', optional: true },
    {
      title: t('admin.tags.headers.protocolSourceConnectionNumber'),
      key: 'protocolSourceConnectionNumber',
      sortable: true,
      optional: true,
    },
    { title: t('admin.tags.headers.address'), key: 'address', sortable: false, optional: true },
    { title: t('admin.tags.headers.type'), key: 'type', sortable: true, optional: true },
    { title: t('admin.tags.headers.origin'), key: 'origin', sortable: true, optional: true },
    { title: t('admin.tags.headers.group1'), key: 'group1', sortable: true, optional: true },
    { title: t('admin.tags.headers.group2'), key: 'group2', sortable: true, optional: true },
    { title: t('admin.tags.headers.group3'), key: 'group3', sortable: true, optional: true },
    { title: t('admin.tags.headers.description'), key: 'description', sortable: true, optional: true },
    { title: t('admin.tags.headers.value'), key: 'value', sortable: true, align: 'end', optional: true },
    { title: t('admin.tags.headers.quality'), key: 'invalid', sortable: true, optional: true },
    { title: t('admin.tags.headers.timeTag'), key: 'timeTag', sortable: true, optional: true },
    { title: t('admin.tags.headers.priority'), key: 'priority', sortable: true, align: 'end', optional: true },
    { title: '', key: 'Actions', sortable: false, align: 'end', width: 110 },
  ])
  const defaultColumns = ['_id', 'protocolSourceConnectionNumber', 'type', 'group1', 'description', 'value', 'invalid']
  const visibleColumns = ref(loadPref('tags-columns', defaultColumns))
  const optionalColumns = computed(() => allHeaders.value.filter((h) => h.optional))
  const visibleHeaders = computed(() =>
    allHeaders.value.filter((h) => !h.optional || visibleColumns.value.includes(h.key))
  )
  function toggleColumn(key) {
    const v = visibleColumns.value
    visibleColumns.value = v.includes(key) ? v.filter((k) => k !== key) : [...v, key]
    savePref('tags-columns', visibleColumns.value)
  }

  // list filters (sent to listTags as a field filter)
  const showFilters = ref(false)
  const filterTypes = ref([])
  const filterOrigins = ref([])
  const filterConnections = ref([])
  const filterGroup1 = ref('')
  const filterQuality = ref('all')
  const filterAlarm = ref('all')
  const typeItems = ['analog', 'digital', 'string', 'json']
  const originItems = ['supervised', 'calculated', 'command', 'manual']
  const typeColor = { analog: 'primary', digital: 'teal', string: 'deep-purple', json: 'orange' }
  const qualityItems = computed(() => [
    { title: t('admin.common.all'), value: 'all' },
    { title: t('admin.tags.qualityValid'), value: 'valid' },
    { title: t('admin.tags.qualityInvalid'), value: 'invalid' },
  ])
  const alarmItems = computed(() => [
    { title: t('admin.common.all'), value: 'all' },
    { title: t('admin.tags.alarmed'), value: 'alarmed' },
    { title: t('admin.tags.notAlarmed'), value: 'normal' },
  ])
  const connectionItems = computed(() => [
    { title: '0 · ' + t('admin.tags.noConnection'), value: 0 },
    ...protocolConnections.value.map((c) => ({
      title: c.protocolConnectionNumber + ' · ' + c.name,
      value: c.protocolConnectionNumber,
    })),
  ])
  const activeFilterCount = computed(
    () =>
      [filterTypes.value.length, filterOrigins.value.length, filterConnections.value.length].filter((n) => n > 0)
        .length +
      (filterGroup1.value?.trim() ? 1 : 0) +
      (filterQuality.value !== 'all' ? 1 : 0) +
      (filterAlarm.value !== 'all' ? 1 : 0)
  )

  function buildFilter() {
    const filter = {}
    const re = (v) => ({ $regex: v.trim(), $options: 'i' })
    if (searchTag.value?.trim()) filter.tag = re(searchTag.value)
    if (searchDescription.value?.trim()) filter.description = re(searchDescription.value)
    if (filterGroup1.value?.trim()) filter.group1 = re(filterGroup1.value)
    if (filterTypes.value.length) filter.type = { $in: filterTypes.value }
    if (filterOrigins.value.length) filter.origin = { $in: filterOrigins.value }
    if (filterConnections.value.length) filter.protocolSourceConnectionNumber = { $in: filterConnections.value }
    if (filterQuality.value !== 'all') filter.invalid = filterQuality.value === 'invalid'
    if (filterAlarm.value !== 'all') filter.alarmed = filterAlarm.value === 'alarmed'
    return filter
  }
  function applyFilters() {
    // a page change makes the table ask for the list (update:options)
    if (page.value !== 1) page.value = 1
    else fetchTags()
  }
  function clearAndApply(name) {
    const refs = { searchTag, searchDescription, filterGroup1 }
    refs[name].value = ''
    applyFilters()
  }
  function clearFilters() {
    filterTypes.value = []
    filterOrigins.value = []
    filterConnections.value = []
    filterGroup1.value = ''
    filterQuality.value = 'all'
    filterAlarm.value = 'all'
    applyFilters()
  }

  // the Connections section asks for the tags of one connection (also before
  // this section was first shown: then the table's first request uses it)
  let isMounted = false
  watch(
    () => props.connectionFilter,
    (f) => {
      if (!f) return
      filterConnections.value = [f.n]
      showFilters.value = true
      if (isMounted) applyFilters()
    },
    { immediate: true }
  )

  const connectionName = (n) => protocolDriveNameByConnNumber.value[n] || ''

  function valueText(item) {
    switch (item.type) {
      case 'digital': {
        const on = !!item.value
        const txt = on ? item.stateTextTrue : item.stateTextFalse
        return (txt ? txt + ' ' : '') + '(' + (on ? 1 : 0) + ')'
      }
      case 'analog': {
        const v = Number(item.value)
        if (!Number.isFinite(v)) return String(item.value ?? '')
        const s = Math.abs(v) >= 1e6 || (v !== 0 && Math.abs(v) < 1e-3) ? v.toExponential(3) : v.toFixed(3)
        return item.unit ? s + ' ' + item.unit : s
      }
      case 'json': {
        const s = item.valueJson !== undefined ? JSON.stringify(item.valueJson) : item.valueString || ''
        return s.length > 60 ? s.slice(0, 60) + '…' : s
      }
      default: {
        const s = String(item.valueString ?? '')
        return s.length > 60 ? s.slice(0, 60) + '…' : s
      }
    }
  }
  function valueTitle(item) {
    if (item.type === 'json' && item.valueJson !== undefined) return JSON.stringify(item.valueJson, null, 2)
    if (item.type === 'string') return String(item.valueString ?? '')
    return ''
  }
  function addressText(item) {
    return [item.protocolSourceCommonAddress, item.protocolSourceObjectAddress]
      .filter((v) => v !== undefined && v !== null && v !== '')
      .join(' / ')
  }
  const trendHref = (item) => '/#/trend-viewer?point=' + encodeURIComponent(item._id)

  const snackbar = ref({ show: false, text: '', color: undefined })
  function notify(text, color) {
    snackbar.value = { show: true, text, color }
  }
  async function copyName(name) {
    if (await copyText(name)) notify(t('admin.common.copied'))
  }

  // copy of the tag configuration as a new tag: the server gives the next
  // free point key; runtime fields (value, quality, times) are left out
  const runtimeFields = [
    '_id', 'value', 'valueString', 'valueJson', 'valueDefault', 'timeTag', 'timeTagAtSource', 'timeTagAtSourceOk',
    'timeTagAlarm', 'timeTagAlarmRange', 'alarmed', 'alarmRange', 'invalid', 'overflow', 'transient', 'frozen',
    'substituted', 'sourceDataUpdate', 'beepType', 'beepGroup', 'updatesCnt', 'commStatus', 'id',
  ]
  const cloneTagOpenDialog = async (item) => {
    const copy = JSON.parse(JSON.stringify(item))
    for (const f of runtimeFields) delete copy[f]
    copy.tag = (item.tag || '') + '_copy'
    copy.value = 0
    copy.valueString = ''
    copy.invalid = true
    isNewTag.value = true
    editedTag.value = { ...defaultTagValue.value, ...copy, _id: 0 }
    await fetchProtocolConnections()
    dialogEditTag.value = true
  }

  // CSV of every tag that matches the filters, in the visible columns
  const exporting = ref(false)
  const EXPORT_MAX = 100000
  async function exportCsv() {
    exporting.value = true
    try {
      const { items, truncated } = await fetchAllPages(
        '/Invoke/auth/listTags',
        { sortBy: sortBy.value, filter: buildFilter() },
        'tags',
        EXPORT_MAX
      )
      const cols = visibleHeaders.value
        .filter((h) => h.key !== 'Actions')
        .flatMap((h) => {
          switch (h.key) {
            case 'value':
              return [
                { title: 'value', value: (r) => r.value },
                { title: 'valueString', value: (r) => r.valueString },
                { title: 'unit', value: (r) => r.unit },
              ]
            case 'invalid':
              return [
                { title: 'invalid', value: (r) => !!r.invalid },
                { title: 'alarmed', value: (r) => !!r.alarmed },
              ]
            case 'address':
              return [
                { title: 'protocolSourceCommonAddress', value: (r) => r.protocolSourceCommonAddress },
                { title: 'protocolSourceObjectAddress', value: (r) => r.protocolSourceObjectAddress },
              ]
            case 'timeTag':
              return [{ title: 'timeTag', value: (r) => (r.timeTag ? new Date(r.timeTag).toISOString() : '') }]
            default:
              return [{ title: h.key, value: (r) => r[h.key] }]
          }
        })
      if (!cols.some((c) => c.title === 'tag')) cols.unshift({ title: 'tag', value: (r) => r.tag })
      downloadText('tags_' + fileStamp() + '.csv', toCsv(items, cols))
      if (truncated) notify(t('admin.common.exportLimited', { n: EXPORT_MAX.toLocaleString() }), 'warning')
    } catch (err) {
      setError(err)
    } finally {
      exporting.value = false
    }
  }

  // live values: the page is read again every 5 s while the section is shown
  const autoRefresh = ref(false)
  let refreshTimer = null
  watch(
    [autoRefresh, () => props.active],
    ([on, active]) => {
      clearInterval(refreshTimer)
      refreshTimer = null
      if (on && active)
        refreshTimer = setInterval(() => {
          if (!document.hidden && !loading.value && !dialogEditTag.value) fetchTags(true)
        }, 5000)
    }
  )
  onUnmounted(() => clearInterval(refreshTimer))

  const handleOptionsUpdate = (newOptions) => {
    page.value = newOptions.page
    itemsPerPage.value = newOptions.itemsPerPage
    sortBy.value = newOptions.sortBy
    savePref('tags-page-size', newOptions.itemsPerPage)
    fetchTags()
  }

  const newTagOpenDialog = async () => {
    isNewTag.value = true
    editedTag.value = Object.assign({}, defaultTagValue.value)
    await fetchProtocolConnections()
    dialogEditTag.value = true
  }

  const editTagOpenDialog = async (item) => {
    isNewTag.value = false
    editedTag.value = Object.assign({}, item)
    await fetchProtocolConnections()
    dialogEditTag.value = true
  }

  const deleteTagOpenDialog = (item) => {
    editedTag.value = Object.assign({}, item)
    dialogDeleteTag.value = true
  }

  const deleteTag = async () => {
    try {
      const response = await fetch('/Invoke/auth/deleteTag', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          tag: editedTag.value.tag,
          _id: editedTag.value._id,
        }),
      })
      const json = await response.json()
      if (json.error) {
        setError(json)
        return
      }
      dialogDeleteTag.value = false
    } catch (err) {
      setError(err)
    }
    fetchTags()
  }

  const closeEditTag = () => {
    dialogEditTag.value = false
    fetchTags()
  }

  const closeDeleteTag = () => {
    dialogDeleteTag.value = false
  }

  // limits shown in pairs: high levels left, low levels right, most severe first
  const limitFieldsShown = [
    'hihihiLimit', 'lololoLimit', 'hihiLimit', 'loloLimit', 'hiLimit', 'loLimit',
  ]
  // the edited tag as sent: limits and hysteresis as numbers, an empty
  // limit as null (no limit)
  const tagToSend = () => {
    const tag = { ...editedTag.value }
    for (const k of limitFieldsShown)
      if (k in tag) {
        const v = parseFloat(tag[k])
        tag[k] = isNaN(v) ? null : v
      }
    if ('hysteresis' in tag) {
      const h = parseFloat(tag.hysteresis)
      tag.hysteresis = isNaN(h) || h < 0 ? 0 : h
    }
    return tag
  }

  const updateOrCreateTag = async () => {
    if (isNewTag.value) {
      await createTag()
    } else {
      await updateTag()
    }
  }

  const createTag = async () => {
    try {
      const response = await fetch('/Invoke/auth/createTag', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(tagToSend()),
      })
      const json = await response.json()
      if (json.error || !json._id) {
        setError(json)
        return
      }
      dialogEditTag.value = false
    } catch (err) {
      setError(err)
    }
    fetchTags()
  }

  const updateTag = async () => {
    try {
      const response = await fetch('/Invoke/auth/updateTag', {
        method: 'post',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(tagToSend()),
      })
      const json = await response.json()
      if (json.error) {
        setError(json)
        return
      }
      closeEditTag()
    } catch (err) {
      setError(err)
    }
    fetchTags()
  }

  // `quiet`: auto refresh, without the loading bar; answers of older requests
  // (filters changed meanwhile) are dropped
  let fetchSeq = 0
  const fetchTags = async (quiet = false) => {
    const seq = ++fetchSeq
    if (quiet !== true) loading.value = true
    try {
      const json = await postJson('/Invoke/auth/listTags', {
        itemsPerPage: itemsPerPage.value,
        sortBy: sortBy.value,
        page: page.value,
        filter: buildFilter(),
      })
      if (seq !== fetchSeq) return
      if (json.error) {
        setError(json)
        return
      }
      tags.value = json.tags || []
      totalTags.value = json.countTotal || 0
    } catch (err) {
      setError(err)
    } finally {
      if (seq === fetchSeq) loading.value = false
    }
  }

  // the table asks for the first page itself (update:options); the connection
  // names label the connection column and filter
  onMounted(() => {
    isMounted = true
    fetchProtocolConnections()
  })

  const fetchProtocolConnections = async () => {
    try {
      const response = await fetch('/Invoke/auth/listProtocolConnections')
      const json = await response.json()
      for (let i = 0; i < json.length; i++) {
        json[i].id = i + 1
      }
      protocolConnections.value = json
      protocolConnectionsDestinations.value = []
      protocolConnections.value.forEach((item) => {
        if (
          item.protocolDriver === 'OPC-UA_SERVER' ||
          item.protocolDriver === 'IEC60870-5-104_SERVER' ||
          item.protocolDriver === 'IEC60870-5-101_SERVER'
        ) {
          protocolConnectionsDestinations.value.push(item)
        }
        protocolDriveNameByConnNumber.value[item.protocolConnectionNumber] = item.name
      })
    } catch (err) {
      setError(err)
    }
  }

  const addNewProtocolDestination = async () => {
    if (!isNaN(parseInt(newProtocolDestination.value.protocolDestinationObjectAddress))) {
      newProtocolDestination.value.protocolDestinationObjectAddress = parseInt(newProtocolDestination.value.protocolDestinationObjectAddress)
    }
    if (!isNaN(parseInt(newProtocolDestination.value.protocolDestinationCommonAddress))) {
      newProtocolDestination.value.protocolDestinationCommonAddress = parseInt(newProtocolDestination.value.protocolDestinationCommonAddress)
    }
    if (!isNaN(parseInt(newProtocolDestination.value.protocolDestinationASDU))) {
      newProtocolDestination.value.protocolDestinationASDU = parseInt(newProtocolDestination.value.protocolDestinationASDU)
    }
    if (!isNaN(parseInt(newProtocolDestination.value.protocolDestinationCommandDuration))) {
      newProtocolDestination.value.protocolDestinationCommandDuration = parseInt(newProtocolDestination.value.protocolDestinationCommandDuration)
    }
    if (!isNaN(parseInt(newProtocolDestination.value.protocolDestinationGroup))) {
      newProtocolDestination.value.protocolDestinationGroup = parseInt(newProtocolDestination.value.protocolDestinationGroup)
    }
    editedTag.value.protocolDestinations.push(newProtocolDestination.value)
    dialogAddProtocolDestination.value = false
  }

  const setError = (err) => {
    error.value = true
    console.warn(err)
    notify(t('common.error') + ' ' + (err?.error || err?.message || ''), 'error')
    setTimeout(() => {
      error.value = false
    }, 2000)
  }

  defineExpose({ fetchTags })
</script>

<style scoped>
  .tag-value {
    white-space: nowrap;
  }
</style>
