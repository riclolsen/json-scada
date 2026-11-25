import { createRouter, createWebHashHistory } from 'vue-router'
import LoginPage from '../components/LoginPage.vue'
import DashboardPage from '../components/DashboardPage.vue'
import AboutPage from '../components/AboutPage.vue'
import AdminPage from '../components/AdminPage.vue'
import DisplayViewerPage from '../components/DisplayViewerPage.vue'
import TabularViewerPage from '../components/TabularViewerPage.vue'
import EventsViewerPage from '../components/EventsViewerPage.vue'
import AlarmsViewerPage from '../components/AlarmsViewerPage.vue'
import SecEventsViewerPage from '../components/SecEventsViewerPage.vue'
import TrendViewerPage from '../components/TrendViewerPage.vue'
import GrafanaPage from '../components/GrafanaPage.vue'
import ApolloPage from '../components/ApolloPage.vue'
import NodeRedPage from '../components/NodeRedPage.vue'
import N8nPage from '../components/N8nPage.vue'
import MetabasePage from '../components/MetabasePage.vue'
import LogViewerPage from '../components/LogViewerPage.vue' 
import CustomDevelopmentsPage from '../components/CustomDevelopmentsPage.vue'
import CimEmsPage from '../components/CimEmsPage.vue'
import SVGEditPage from '../components/SVGEditPage.vue'

const routes = [
  { path: '/', redirect: '/login' },
  { path: '/login', component: LoginPage },
  { path: '/dashboard', component: DashboardPage },
  { path: '/display-viewer', component: DisplayViewerPage },
  { path: '/tabular-viewer', component: TabularViewerPage },
  { path: '/alarms-viewer', component: AlarmsViewerPage },
  { path: '/events-viewer', component: EventsViewerPage },
  { path: '/sec-events-viewer', component: SecEventsViewerPage },
  { path: '/trend-viewer', component: TrendViewerPage },
  // former paths of the native viewers (kept for bookmarks and saved links)
  { path: '/display-viewer-new', redirect: (to) => ({ path: '/display-viewer', query: to.query }) },
  { path: '/tabular-viewer-new', redirect: (to) => ({ path: '/tabular-viewer', query: to.query }) },
  { path: '/alarms-viewer-new', redirect: (to) => ({ path: '/alarms-viewer', query: to.query }) },
  { path: '/events-viewer-new', redirect: (to) => ({ path: '/events-viewer', query: to.query }) },
  { path: '/log-viewer', component: LogViewerPage },
  { path: '/about', component: AboutPage },
  { path: '/admin', component: AdminPage },
  { path: '/cim-ems', component: CimEmsPage },
  { path: '/grafana', component: GrafanaPage },
  { path: '/metabase', component: MetabasePage },
  { path: '/apollo', component: ApolloPage },
  { path: '/nodered', component: NodeRedPage },
  { path: '/n8n', component: N8nPage },
  { path: '/custom-developments', component: CustomDevelopmentsPage },
  { path: '/svg-edit', component: SVGEditPage },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
