---
name: rollup
description: This skill should be used when working with Rollup module bundler, including configuration, plugins, code splitting, and build optimization. Provides comprehensive knowledge of Rollup patterns, plugin development, and bundling strategies.
---

# Rollup Skill

This skill provides comprehensive knowledge and patterns for working with Rollup module bundler effectively.

## When to Use This Skill

Use this skill when:
- Configuring Rollup for web applications
- Setting up Rollup for library builds
- Working with Rollup plugins
- Implementing code splitting
- Optimizing bundle size
- Troubleshooting build issues
- Integrating Rollup with Svelte or other frameworks
- Developing custom Rollup plugins

## Core Concepts

### Rollup Overview

Rollup is a module bundler that:
- **Tree-shakes by default** - Removes unused code automatically
- **ES module focused** - Native ESM output support
- **Plugin-based** - Extensible architecture
- **Multiple outputs** - Generate multiple formats from single input
- **Code splitting** - Dynamic imports for lazy loading
- **Scope hoisting** - Flattens modules for smaller bundles

### Basic Configuration

```javascript
// rollup.config.js
export default {
  input: 'src/main.js',
  output: {
    file: 'dist/bundle.js',
    format: 'esm'
  }
};
```

### Output Formats

Rollup supports multiple output formats:

| Format | Description | Use Case |
|--------|-------------|----------|
| `esm` | ES modules | Modern browsers, bundlers |
| `cjs` | CommonJS | Node.js |
| `iife` | Self-executing function | Script tags |
| `umd` | Universal Module Definition | CDN, both environments |
| `amd` | Asynchronous Module Definition | RequireJS |
| `system` | SystemJS | SystemJS loader |

## Configuration

### Full Configuration Options

```javascript
// rollup.config.js
import resolve from '@rollup/plugin-node-resolve';
import commonjs from '@rollup/plugin-commonjs';
import terser from '@rollup/plugin-terser';

const production = !process.env.ROLLUP_WATCH;

export default {
  // Entry point(s)
  input: 'src/main.js',

  // Output configuration
  output: {
    // Output file or directory
    file: 'dist/bundle.js',
    // Or for code splitting:
    // dir: 'dist',

    // Output format
    format: 'esm',

    // Name for IIFE/UMD builds
    name: 'MyBundle',

    // Sourcemap generation
    sourcemap: true,

    // Global variables for external imports (IIFE/UMD)
    globals: {
      jquery: '$'
    },

    // Banner/footer comments
    banner: '/* My library v1.0.0 */',
    footer: '/* End of bundle */',

    // Chunk naming for code splitting
    chunkFileNames: '[name]-[hash].js',
    entryFileNames: '[name].js',

    // Manual chunks for code splitting
    manualChunks: {
      vendor: ['lodash', 'moment']
    },

    // Interop mode for default exports
    interop: 'auto',

    // Preserve modules structure
    preserveModules: false,

    // Exports mode
    exports: 'auto' // 'default', 'named', 'none', 'auto'
  },

  // External dependencies (not bundled)
  external: ['lodash', /^node:/],

  // Plugin array
  plugins: [
    resolve({
      browser: true,
      dedupe: ['svelte']
    }),
    commonjs(),
    production && terser()
  ],

  // Watch mode options
  watch: {
    include: 'src/**',
    exclude: 'node_modules/**',
    clearScreen: false
  },

  // Warning handling
  onwarn(warning, warn) {
    // Skip certain warnings
    if (warning.code === 'CIRCULAR_DEPENDENCY') return;
    warn(warning);
  },

  // Preserve entry signatures for code splitting
  preserveEntrySignatures: 'strict',

  // Treeshake options
  treeshake: {
    moduleSideEffects: false,
    propertyReadSideEffects: false
  }
};
```

### Multiple Outputs

```javascript
export default {
  input: 'src/main.js',
  output: [
    {
      file: 'dist/bundle.esm.js',
      format: 'esm'
    },
    {
      file: 'dist/bundle.cjs.js',
      format: 'cjs'
    },
    {
      file: 'dist/bundle.umd.js',
      format: 'umd',
      name: 'MyLibrary'
    }
  ]
};
```

### Multiple Entry Points

```javascript
export default {
  input: {
    main: 'src/main.js',
    utils: 'src/utils.js'
  },
  output: {
    dir: 'dist',
    format: 'esm'
  }
};
```

### Array of Configurations

```javascript
export default [
  {
    input: 'src/main.js',
    output: { file: 'dist/main.js', format: 'esm' }
  },
  {
    input: 'src/worker.js',
    output: { file: 'dist/worker.js', format: 'iife' }
  }
];
```

## Essential Plugins

### @rollup/plugin-node-resolve

Resolve node_modules imports:

```javascript
import resolve from '@rollup/plugin-node-resolve';

export default {
  plugins: [
    resolve({
      // Resolve browser field in package.json
      browser: true,

      // Prefer built-in modules
      preferBuiltins: true,

      // Only resolve these extensions
      extensions: ['.mjs', '.js', '.json', '.node'],

      // Dedupe packages (important for Svelte)
      dedupe: ['svelte'],

      // Main fields to check in package.json
      mainFields: ['module', 'main', 'browser'],

      // Export conditions
      exportConditions: ['svelte', 'browser', 'module', 'import']
    })
  ]
};
```

### @rollup/plugin-commonjs

Convert CommonJS to ES modules:

```javascript
import commonjs from '@rollup/plugin-commonjs';

export default {
  plugins: [
    commonjs({
      // Include specific modules
      include: /node_modules/,

      // Exclude specific modules
      exclude: ['node_modules/lodash-es/**'],

      // Ignore conditional requires
      ignoreDynamicRequires: false,

      // Transform mixed ES/CJS modules
      transformMixedEsModules: true,

      // Named exports for specific modules
      namedExports: {
        'react': ['createElement', 'Component']
      }
    })
  ]
};
```

### @rollup/plugin-terser

Minify output:

```javascript
import terser from '@rollup/plugin-terser';

export default {
  plugins: [
    terser({
      compress: {
        drop_console: true,
        drop_debugger: true
      },
      mangle: true,
      format: {
        comments: false
      }
    })
  ]
};
```

### rollup-plugin-svelte

Compile Svelte components:

```javascript
import svelte from 'rollup-plugin-svelte';
import css from 'rollup-plugin-css-only';

export default {
  plugins: [
    svelte({
      // Enable dev mode
      dev: !production,

      // Emit CSS as a separate file
      emitCss: true,

      // Preprocess (SCSS, TypeScript, etc.)
      preprocess: sveltePreprocess(),

      // Compiler options
      compilerOptions: {
        dev: !production
      },

      // Custom element mode
      customElement: false
    }),

    // Extract CSS to separate file
    css({ output: 'bundle.css' })
  ]
};
```

### Other Common Plugins

```javascript
import json from '@rollup/plugin-json';
import replace from '@rollup/plugin-replace';
import alias from '@rollup/plugin-alias';
import image from '@rollup/plugin-image';
import copy from 'rollup-plugin-copy';
import livereload from 'rollup-plugin-livereload';

export default {
  plugins: [
    // Import JSON files
    json(),

    // Replace strings in code
    replace({
      preventAssignment: true,
      'process.env.NODE_ENV': JSON.stringify('production'),
      '__VERSION__': JSON.stringify('1.0.0')
    }),

    // Path aliases
    alias({
      entries: [
        { find: '@', replacement: './src' },
        { find: 'utils', replacement: './src/utils' }
      ]
    }),

    // Import images
    image(),

    // Copy static files
    copy({
      targets: [
        { src: 'public/*', dest: 'dist' }
      ]
    }),

    // Live reload in dev
    !production && livereload('dist')
  ]
};
```

## Code Splitting

### Dynamic Imports

```javascript
// Automatically creates chunks
async function loadFeature() {
  const { feature } = await import('./feature.js');
  feature();
}
```

Configuration for code splitting:

```javascript
export default {
  input: 'src/main.js',
  output: {
    dir: 'dist',
    format: 'esm',
    chunkFileNames: 'chunks/[name]-[hash].js'
  }
};
```

### Manual Chunks

```javascript
export default {
  output: {
    manualChunks: {
      // Vendor chunk
      vendor: ['lodash', 'moment'],

      // Or use a function for more control
      manualChunks(id) {
        if (id.includes('node_modules')) {
          return 'vendor';
        }
      }
    }
  }
};
```

### Advanced Chunking Strategy

```javascript
export default {
  output: {
    manualChunks(id, { getModuleInfo }) {
      // Separate chunks by feature
      if (id.includes('/features/auth/')) {
        return 'auth';
      }
      if (id.includes('/features/dashboard/')) {
        return 'dashboard';
      }

      // Vendor chunks by package
      if (id.includes('node_modules')) {
        const match = id.match(/node_modules\/([^/]+)/);
        if (match) {
          const packageName = match[1];
          // Group small packages
          const smallPackages = ['lodash', 'date-fns'];
          if (smallPackages.includes(packageName)) {
            return 'vendor-utils';
          }
          return `vendor-${packageName}`;
        }
      }
    }
  }
};
```

## Watch Mode

### Configuration

```javascript
export default {
  watch: {
    // Files to watch
    include: 'src/**',

    // Files to ignore
    exclude: 'node_modules/**',

    // Don't clear screen on rebuild
    clearScreen: false,

    // Rebuild delay
    buildDelay: 0,

    // Watch chokidar options
    chokidar: {
      usePolling: true
    }
  }
};
```

### CLI Watch Mode

```bash
# Watch mode
rollup -c -w

# With environment variable
ROLLUP_WATCH=true rollup -c
```

## Plugin Development

### Plugin Structure

```javascript
function myPlugin(options = {}) {
  return {
    // Plugin name (required)
    name: 'my-plugin',

    // Build hooks
    options(inputOptions) {
      // Modify input options
      return inputOptions;
    },

    buildStart(inputOptions) {
      // Called on build start
    },

    resolveId(source, importer, options) {
      // Custom module resolution
      if (source === 'virtual-module') {
        return source;
      }
      return null; // Defer to other plugins
    },

    load(id) {
      // Load module content
      if (id === 'virtual-module') {
        return 'export default "Hello"';
      }
      return null;
    },

    transform(code, id) {
      // Transform module code
      if (id.endsWith('.txt')) {
        return {
          code: `export default ${JSON.stringify(code)}`,
          map: null
        };
      }
    },

    buildEnd(error) {
      // Called when build ends
      if (error) {
        console.error('Build failed:', error);
      }
    },

    // Output generation hooks
    renderStart(outputOptions, inputOptions) {
      // Called before output generation
    },

    banner() {
      return '/* Custom banner */';
    },

    footer() {
      return '/* Custom footer */';
    },

    renderChunk(code, chunk, options) {
      // Transform output chunk
      return code;
    },

    generateBundle(options, bundle) {
      // Modify output bundle
      for (const fileName in bundle) {
        const chunk = bundle[fileName];
        if (chunk.type === 'chunk') {
          // Modify chunk
        }
      }
    },

    writeBundle(options, bundle) {
      // After bundle is written
    },

    closeBundle() {
      // Called when bundle is closed
    }
  };
}

export default myPlugin;
```

### Plugin with Rollup Utils

```javascript
import { createFilter } from '@rollup/pluginutils';

function myTransformPlugin(options = {}) {
  const filter = createFilter(options.include, options.exclude);

  return {
    name: 'my-transform',

    transform(code, id) {
      if (!filter(id)) return null;

      // Transform code
      const transformed = code.replace(/foo/g, 'bar');

      return {
        code: transformed,
        map: null // Or generate sourcemap
      };
    }
  };
}
```

## Svelte Integration

### Complete Svelte Setup

```javascript
// rollup.config.js
import svelte from 'rollup-plugin-svelte';
import commonjs from '@rollup/plugin-commonjs';
import resolve from '@rollup/plugin-node-resolve';
import terser from '@rollup/plugin-terser';
import css from 'rollup-plugin-css-only';
import livereload from 'rollup-plugin-livereload';

const production = !process.env.ROLLUP_WATCH;

function serve() {
  let server;

  function toExit() {
    if (server) server.kill(0);
  }

  return {
    writeBundle() {
      if (server) return;
      server = require('child_process').spawn(
        'npm',
        ['run', 'start', '--', '--dev'],
        {
          stdio: ['ignore', 'inherit', 'inherit'],
          shell: true
        }
      );

      process.on('SIGTERM', toExit);
      process.on('exit', toExit);
    }
  };
}

export default {
  input: 'src/main.js',
  output: {
    sourcemap: true,
    format: 'iife',
    name: 'app',
    file: 'public/build/bundle.js'
  },
  plugins: [
    svelte({
      compilerOptions: {
        dev: !production
      }
    }),
    css({ output: 'bundle.css' }),

    resolve({
      browser: true,
      dedupe: ['svelte']
    }),
    commonjs(),

    // Dev server
    !production && serve(),
    !production && livereload('public'),

    // Minify in production
    production && terser()
  ],
  watch: {
    clearScreen: false
  }
};
```

## Best Practices

### Bundle Optimization

1. **Enable tree shaking** - Use ES modules
2. **Mark side effects** - Set `sideEffects` in package.json
3. **Use terser** - Minify production builds
4. **Analyze bundles** - Use rollup-plugin-visualizer
5. **Code split** - Lazy load routes and features

### External Dependencies

```javascript
export default {
  // Don't bundle peer dependencies for libraries
  external: [
    'react',
    'react-dom',
    /^lodash\//
  ],
  output: {
    globals: {
      react: 'React',
      'react-dom': 'ReactDOM'
    }
  }
};
```

### Development vs Production

```javascript
const production = !process.env.ROLLUP_WATCH;

export default {
  plugins: [
    replace({
      preventAssignment: true,
      'process.env.NODE_ENV': JSON.stringify(
        production ? 'production' : 'development'
      )
    }),
    production && terser()
  ].filter(Boolean)
};
```

### Error Handling

```javascript
export default {
  onwarn(warning, warn) {
    // Ignore circular dependency warnings
    if (warning.code === 'CIRCULAR_DEPENDENCY') {
      return;
    }

    // Ignore unused external imports
    if (warning.code === 'UNUSED_EXTERNAL_IMPORT') {
      return;
    }

    // Treat other warnings as errors
    if (warning.code === 'UNRESOLVED_IMPORT') {
      throw new Error(warning.message);
    }

    // Use default warning handling
    warn(warning);
  }
};
```

## Common Patterns

### Library Build

```javascript
import pkg from './package.json';

export default {
  input: 'src/index.js',
  external: Object.keys(pkg.peerDependencies || {}),
  output: [
    {
      file: pkg.main,
      format: 'cjs',
      sourcemap: true
    },
    {
      file: pkg.module,
      format: 'esm',
      sourcemap: true
    }
  ]
};
```

### Application Build

```javascript
export default {
  input: 'src/main.js',
  output: {
    dir: 'dist',
    format: 'esm',
    chunkFileNames: 'chunks/[name]-[hash].js',
    entryFileNames: '[name]-[hash].js',
    sourcemap: true
  },
  plugins: [
    // All dependencies bundled
    resolve({ browser: true }),
    commonjs(),
    terser()
  ]
};
```

### Web Worker Build

```javascript
export default [
  // Main application
  {
    input: 'src/main.js',
    output: {
      file: 'dist/main.js',
      format: 'esm'
    },
    plugins: [resolve(), commonjs()]
  },
  // Web worker (IIFE format)
  {
    input: 'src/worker.js',
    output: {
      file: 'dist/worker.js',
      format: 'iife'
    },
    plugins: [resolve(), commonjs()]
  }
];
```

## Troubleshooting

### Common Issues

**Module not found:**
- Check @rollup/plugin-node-resolve is configured
- Verify package is installed
- Check `external` array

**CommonJS module issues:**
- Add @rollup/plugin-commonjs
- Check `namedExports` configuration
- Try `transformMixedEsModules: true`

**Circular dependencies:**
- Use `onwarn` to suppress or fix
- Refactor to break cycles
- Check import order

**Sourcemaps not working:**
- Set `sourcemap: true` in output
- Ensure plugins pass through maps
- Check browser devtools settings

**Large bundle size:**
- Use rollup-plugin-visualizer
- Check for duplicate dependencies
- Verify tree shaking is working
- Mark unused packages as external

## CLI Reference

```bash
# Basic build
rollup -c

# Watch mode
rollup -c -w

# Custom config
rollup -c rollup.custom.config.js

# Output format
rollup src/main.js --format esm --file dist/bundle.js

# Environment variables
NODE_ENV=production rollup -c

# Silent mode
rollup -c --silent

# Generate bundle stats
rollup -c --perf
```

## References

- **Rollup Documentation**: https://rollupjs.org
- **Plugin Directory**: https://github.com/rollup/plugins
- **Awesome Rollup**: https://github.com/rollup/awesome
- **GitHub**: https://github.com/rollup/rollup

## Related Skills

- **svelte** - Using Rollup with Svelte
- **typescript** - TypeScript compilation with Rollup
- **nostr-tools** - Bundling Nostr applications
