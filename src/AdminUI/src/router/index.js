import { createRouter, createWebHistory } from 'vue-router'
import LoginPage from '../components/LoginPage.vue'
import DashboardPage from '../components/DashboardPage.vue'
import AboutPage from '../components/AboutPage.vue'
import AdminPage from '../components/AdminPage.vue'
import DisplayViewerPage from '../components/DisplayViewerPage.vue'
import TabularViewerPage from '../components/TabularViewerPage.vue'
import EventsViewerPage from '../components/EventsViewerPage.vue'
import GrafanaPage from '../components/GrafanaPage.vue'
import AlarmsViewerPage from '@/components/AlarmsViewerPage.vue'
import LogViewerPage from '@/components/LogViewerPage.vue' 

const routes = [
  { path: '/', redirect: '/login' },
  { path: '/login', component: LoginPage },
  { path: '/dashboard', component: DashboardPage },
  {
    path: '/display-viewer',
    component: DisplayViewerPage,
  },
  {
    path: '/alarms-viewer',
    component: AlarmsViewerPage,
  },
  {
    path: '/tabular-viewer',
    component: TabularViewerPage,
  },
  {
    path: '/events-viewer',
    component: EventsViewerPage,
  },
  { path: '/log-viewer', component: LogViewerPage },
  { path: '/about', component: AboutPage },
  { path: '/admin', component: AdminPage },
  { path: '/grafana', component: GrafanaPage },
  { path: '/metabase', component: { template: '<div>Metabase</div>' } },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
