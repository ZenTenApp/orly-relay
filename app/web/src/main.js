import App from "./App.svelte";
import "../public/global.css";
import { initConfig } from "./config.js";

// Initialize relay configuration before creating the app
// This sets up standalone mode detection and relay URL handling
initConfig();

const app = new App({
  target: document.body,
  props: {
    name: "world",
  },
});

export default app;
