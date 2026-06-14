import { createRouter, createWebHashHistory } from 'vue-router'
import LoginPage from '../components/LoginPage.vue'
import DashboardPage from '../components/DashboardPage.vue'
import AboutPage from '../components/AboutPage.vue'
import AdminPage from '../components/AdminPage.vue'
import DisplayViewerPage from '../components/DisplayViewerPage.vue'
import DisplayViewerNewPage from '../components/DisplayViewerNewPage.vue'
import TabularViewerPage from '../components/TabularViewerPage.vue'
import TabularViewerNewPage from '../components/TabularViewerNewPage.vue'
import EventsViewerPage from '../components/EventsViewerPage.vue'
import EventsViewerNewPage from '../components/EventsViewerNewPage.vue'
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
    path: '/display-viewer-new',
    component: DisplayViewerNewPage,
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
    path: '/tabular-viewer-new',
    component: TabularViewerNewPage,
  },
  {
    path: '/alarms-viewer-new',
    component: TabularViewerNewPage,
    props: { mode: 'ALARMS_VIEWER' },
  },
  {
    path: '/events-viewer',
    component: EventsViewerPage,
  },
  {
    path: '/events-viewer-new',
    component: EventsViewerNewPage,
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
