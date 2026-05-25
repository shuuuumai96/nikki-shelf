import { createPinia } from "pinia";
import { createApp } from "vue";
import App from "./App.vue";
import { i18n } from "./shared/i18n";
import "./shared/styles/base.css";
import "./shared/styles/primitives.css";
import "./shared/styles/tokens.css";

createApp(App).use(createPinia()).use(i18n).mount("#app");
