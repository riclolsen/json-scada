import { createRouter, createWebHashHistory } from 'vue-router'
import LoginPage from '../components/LoginPage.vue'
import DashboardPage from '../components/DashboardPage.vue'
import AboutPage from '../components/AboutPage.vue'
import AdminPage from '../components/AdminPage.vue'
import DisplayViewerPage from '../components/DisplayViewerPage.vue'
import TabularViewerPage from '../components/TabularViewerPage.vue'
import EventsViewerPage from '../components/EventsViewerPage.vue'
import GrafanaPage from '../components/GrafanaPage.vue'
import MetabasePage from '../components/MetabasePage.vue'
import AlarmsViewerPage from '../components/AlarmsViewerPage.vue'
import LogViewerPage from '../components/LogViewerPage.vue' 
import CustomDevelopmentsPage from '../components/CustomDevelopmentsPage.vue'
import SVGEditPage from '../components/SVGEditPage.vue'

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
  { path: '/metabase', component: MetabasePage },
  { path: '/custom-developments', component: CustomDevelopmentsPage },
  { path: '/svg-edit', component: SVGEditPage },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
