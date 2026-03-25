import type { Configuration } from 'webpack';

module.exports = {
  entry: {
    background: {
      import: 'src/background.ts',
      runtime: false,
    },
    'smesh-signer-extension': {
      import: 'src/smesh-signer-extension.ts',
      runtime: false,
    },
    'smesh-signer-content-script': {
      import: 'src/smesh-signer-content-script.ts',
      runtime: false,
    },
    prompt: {
      import: 'src/prompt.ts',
      runtime: false,
    },
    options: {
      import: 'src/options.ts',
      runtime: false,
    },
    unlock: {
      import: 'src/unlock.ts',
      runtime: false,
    },
  },
} as Configuration;
