<template>
  <v-container fluid>
    <v-row>
      <v-col cols="12">
        <h1 class="text-h4 mb-6">{{ $t('dashboard.title') }}</h1>
      </v-col>
    </v-row>
    <v-row>
      <v-col v-for="shortcut in shortcuts" :key="shortcut.title" cols="12" sm="6" md="4">
        <v-card dense :color="shortcut.color" hover @click="navigateTo(shortcut.route)">
          <v-card-title class="text-h6">
            {{ $t(shortcut.titleKey) }}
          </v-card-title>
          <v-card-text class="text-center pb-0">
            <component :is="shortcut.icon" size="64" />
          </v-card-text>
          <v-card-actions class="pt-0">
            <v-btn v-if="shortcut.page" icon :href="shortcut.page" :target="shortcut.target" @click.stop="">
              <v-icon>mdi-open-in-new</v-icon>
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { Monitor, Bell, Table, Calendar, FileText, UserCog, HelpCircle,BarChart, Database } from 'lucide-vue-next';

const router = useRouter();

const shortcuts = ref([
  { titleKey: 'dashboard.displayViewer', icon: Monitor, color: 'primary', route: '/display-viewer', page: '/display.html', target:"_blank" },
  { titleKey: 'dashboard.alarmsViewer', icon: Bell, color: 'error', route: '/alarms-viewer', page: '/tabular.html?SELMODULO=ALARMS_VIEWER', target:"_blank" },
  { titleKey: 'dashboard.tabularViewer', icon: Table, color: 'success', route: '/tabular-viewer', page: '/tabular.html', target:"_blank" },
  { titleKey: 'dashboard.eventsViewer', icon: Calendar, color: 'info', route: '/events-viewer', page: '/events.html', target:"_blank" },
  { titleKey: 'dashboard.logViewer', icon: FileText, color: 'warning', route: '/log-viewer', page: '/log-io', target:"_blank" },
  { titleKey: 'dashboard.grafana', icon: BarChart, color: 'secondary', route: '/grafana', page: '/grafana', target:"_blank" },
  { titleKey: 'dashboard.metabase', icon: Database, color: 'primary', route: '/metabase', page: '/metabase', target:"_blank" },
  { titleKey: 'dashboard.admin', icon: UserCog, color: 'primary', route: '/admin' },
  { titleKey: 'dashboard.about', icon: HelpCircle, color: 'secondary', route: '/about' },
]);

const navigateTo = (route) => {
  router.push(route);
};
</script>