import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import HomeView from './views/HomeView.vue'
import CategoryView from './views/CategoryView.vue'
import SourceView from './views/SourceView.vue'
import TutorialView from './views/TutorialView.vue'
import AboutView from './views/AboutView.vue'
import AdminView from './views/AdminView.vue'
import ConfigureView from './views/ConfigureView.vue'
import './style.css'
import './layout.css'
import './menu.css'
import './site-wide.css'
import './relaypulse.css'

const router = createRouter({ history: createWebHistory(), routes: [
  { path: '/', component: HomeView },
  { path: '/status/:category', component: CategoryView },
  { path: '/source/:id', component: SourceView },
  { path: '/tutorial', component: TutorialView },
  { path: '/about', component: AboutView },
  { path: '/admin', component: AdminView },
  { path: '/configure', component: ConfigureView },
] })
createApp(App).use(router).mount('#app')
