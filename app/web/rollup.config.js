import { spawn } from "child_process";
import svelte from "rollup-plugin-svelte";
import commonjs from "@rollup/plugin-commonjs";
import terser from "@rollup/plugin-terser";
import resolve from "@rollup/plugin-node-resolve";
import livereload from "rollup-plugin-livereload";
import css from "rollup-plugin-css-only";
import copy from "rollup-plugin-copy";

const production = !process.env.ROLLUP_WATCH;

// In dev mode, output to public/ so sirv can serve it
// In production, output to dist/ for embedding
const outputDir = production ? "dist" : "public";

function serve() {
  let server;

  function toExit() {
    if (server) server.kill(0);
  }

  return {
    writeBundle() {
      if (server) return;
      server = spawn("npm", ["run", "start", "--", "--dev"], {
        stdio: ["ignore", "inherit", "inherit"],
        shell: true,
      });

      process.on("SIGTERM", toExit);
      process.on("exit", toExit);
    },
  };
}

export default {
  input: "src/main.js",
  output: {
    sourcemap: true,
    format: "iife",
    name: "app",
    file: `${outputDir}/bundle.js`,
  },
  plugins: [
    svelte({
      compilerOptions: {
        // enable run-time checks when not in production
        dev: !production,
      },
    }),
    // we'll extract any component CSS out into
    // a separate file - better for performance
    css({ output: "bundle.css" }),

    // If you have external dependencies installed from
    // npm, you'll most likely need these plugins. In
    // some cases you'll need additional configuration -
    // consult the documentation for details:
    // https://github.com/rollup/plugins/tree/master/packages/commonjs
    resolve({
      browser: true,
      dedupe: ["svelte"],
      exportConditions: ["svelte"],
    }),
    commonjs(),

    // In dev mode, call `npm run start` once
    // the bundle has been generated
    !production && serve(),

    // Watch the `public` directory and refresh the
    // browser on changes when not in production
    !production && livereload("public"),

    // If we're building for production (npm run build
    // instead of npm run dev), minify
    production && terser(),

    // Copy static files from public to dist (only in production)
    production && copy({
      targets: [
        { src: 'public/index.html', dest: 'dist' },
        { src: 'public/global.css', dest: 'dist' },
        { src: 'public/favicon.png', dest: 'dist' },
        { src: 'public/orly.png', dest: 'dist' },
        { src: 'public/orly-favicon.png', dest: 'dist' },
        { src: 'public/manifest.json', dest: 'dist' },
        { src: 'public/sw.js', dest: 'dist' },
        { src: 'public/icon-192.png', dest: 'dist' },
        { src: 'public/icon-512.png', dest: 'dist' }
      ]
    }),
  ],
  watch: {
    clearScreen: false,
  },
};
