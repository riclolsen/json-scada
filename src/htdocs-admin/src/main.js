import Vue from 'vue'
import App from './App.vue'
import vuetify from './plugins/vuetify';
import i18n from "./i18n";
import 'roboto-fontface/css/roboto/roboto-fontface.css'
import '@mdi/font/css/materialdesignicons.css'
import router from './router'

// vue-clickaway to close langaugeSwitcher when clicking outside of it
import { directive as onClickaway } from "vue-clickaway";
Vue.directive("on-clickaway", onClickaway);

Vue.config.productionTip = false

new Vue({
  vuetify,
  router,
  i18n,
  render: h => h(App)
}).$mount('#app')
