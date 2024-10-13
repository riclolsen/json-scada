<template>
  <v-container fluid>
    <v-row>
      <v-col
        v-for="shortcut in shortcuts"
        :key="shortcut.titleKey"
        cols="12"
        sm="6"
        md="4"
      >
        <v-card
          v-if="!(shortcut.titleKey === 'dashboard.admin' && !isAdmin()) && !(shortcut.titleKey === 'dashboard.logViewer' && !isAdmin())"
          dense
          :color="shortcut.color"
          hover
          @click="if(shortcut.route!='') navigateTo(shortcut.route)"
        >
          <v-card-title class="text-h6">
            {{ $t(shortcut.titleKey) }}
          </v-card-title>
          <v-card-text class="text-center pb-0">
            <component :is="shortcut.icon" size="64" />
          </v-card-text>
          <v-card-actions class="pt-0">
            <v-btn
              v-if="shortcut.page"
              icon
              :href="shortcut.page"
              :target="shortcut.target"
              @click.stop=""
            >
              <v-icon>mdi-open-in-new</v-icon>
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
  import { ref, onMounted, onUnmounted } from 'vue'
  import { useRouter } from 'vue-router'
  import {
    Monitor,
    Bell,
    Table,
    Calendar,
    FileText,
    UserCog,
    HelpCircle,
    BarChart,
    Database,
  } from 'lucide-vue-next'

  // Lifecycle hooks
  onMounted(async () => {
    document.documentElement.style.overflowY = 'scroll'
  })

  onUnmounted(async () => {
    document.documentElement.style.overflowY = 'hidden'
  })

  const router = useRouter()

  const shortcuts = ref([
    {
      titleKey: 'dashboard.displayViewer',
      icon: Monitor,
      color: 'primary',
      route: '/display-viewer',
      page: '/display.html',
      target: '_blank',
    },
    {
      titleKey: 'dashboard.alarmsViewer',
      icon: Bell,
      color: 'primary',
      route: '/alarms-viewer',
      page: '/tabular.html?SELMODULO=ALARMS_VIEWER',
      target: '_blank',
    },
    {
      titleKey: 'dashboard.tabularViewer',
      icon: Table,
      color: 'primary',
      route: '/tabular-viewer',
      page: '/tabular.html',
      target: '_blank',
    },
    {
      titleKey: 'dashboard.eventsViewer',
      icon: Calendar,
      color: 'primary',
      route: '/events-viewer',
      page: '/events.html',
      target: '_blank',
    },
    {
      titleKey: 'dashboard.grafana',
      icon: BarChart,
      color: 'secondary',
      route: '/grafana',
      page: '/grafana',
      target: '_blank',
    },
    {
      titleKey: 'dashboard.metabase',
      icon: Database,
      color: 'secondary',
      route: '', // metabase (is not working iframed)
      page: '/metabase',
      target: '_blank',
    },
    {
      titleKey: 'dashboard.admin',
      icon: UserCog,
      color: 'warning',
      route: '/admin',
      page: '/admin',
      target: '_blank',
    },
    {
      titleKey: 'dashboard.logViewer',
      icon: FileText,
      color: 'warning',
      route: '/log-viewer',
      page: '/log-io',
      target: '_blank',
    },
    {
      titleKey: 'dashboard.about',
      icon: HelpCircle,
      color: 'green',
      route: '/about',
    },
  ])

  const navigateTo = (route) => {
    router.push(route)
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

  const isAdmin = () => {
    let ck = parseCookie(document.cookie)
    if ('json-scada-user' in ck) {
      ck = JSON.parse(ck['json-scada-user'])
      if (ck?.rights?.isAdmin) return true
    }
    return false
  }
</script>
