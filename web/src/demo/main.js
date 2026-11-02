import { createApp } from "vue";
import "element-plus/dist/index.css";
import "element-plus/theme-chalk/dark/css-vars.css";
import "../styles/theme.css";
import "./demo.css";
import DemoApp from "./DemoApp.vue";

createApp(DemoApp).mount("#app");
