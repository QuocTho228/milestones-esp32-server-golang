import { createApp } from 'vue';
import { createPinia } from 'pinia';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
// Nhập Vant 4 theo nhu cầu để giảm kích thước gói đóng gói.
import { NavBar, Tabbar, TabbarItem, Form, Field, CellGroup, Button, Tabs, Tab, Cell, Popup, Icon } from 'vant';
import 'vant/lib/index.css';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';
import App from './App.vue';
import router from './router';
import './styles/apple-light.css';

const app = createApp(App);

// Đăng ký tất cả biểu tượng của Element Plus.
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}

// Đăng ký các thành phần Vant (nhập theo nhu cầu).
app.use(NavBar);
app.use(Tabbar);
app.use(TabbarItem);
app.use(Form);
app.use(Field);
app.use(CellGroup);
app.use(Button);
app.use(Tabs);
app.use(Tab);
app.use(Cell);
app.use(Popup);
app.use(Icon);

app.use(createPinia());
app.use(router);
app.use(ElementPlus); // Dành cho phiên bản máy tính.

app.mount('#app');
