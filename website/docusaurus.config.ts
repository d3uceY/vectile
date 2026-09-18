import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const REPO = 'https://github.com/d3uceY/vectile';

const config: Config = {
  title: 'vectile',
  tagline: 'Your private library, searchable on one machine',
  favicon: 'img/vectile-logo.png',

  url: 'https://d3ucey.github.io',
  baseUrl: '/vectile/',
  organizationName: 'd3uceY',
  projectName: 'vectile',
  trailingSlash: false,

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    mermaid: true,
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  themes: ['@docusaurus/theme-mermaid'],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: `${REPO}/tree/main/website/`,
          breadcrumbs: true,
          showLastUpdateAuthor: false,
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/vectile-logo.png',
    colorMode: {
      defaultMode: 'light',
      disableSwitch: true,
      respectPrefersColorScheme: false,
    },
    navbar: {
      title: 'vectile',
      logo: {
        alt: 'vectile',
        src: 'img/vectile-logo.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docs',
          position: 'left',
          label: 'Docs',
        },
        {
          to: '/docs/what-is-vectile',
          position: 'left',
          label: 'How it works',
        },
        {
          href: REPO,
          label: 'GitHub',
          position: 'right',
        },
        {
          to: '/docs/download',
          label: 'Download',
          position: 'right',
          className: 'navbar-download',
        },
      ],
    },
    footer: {
      style: 'light',
      links: [
        {
          title: 'Start here',
          items: [
            {label: 'What is vectile', to: '/docs/what-is-vectile'},
            {label: 'Download', to: '/docs/download'},
            {label: 'First run', to: '/docs/first-run'},
          ],
        },
        {
          title: 'Using it',
          items: [
            {label: 'What you can index', to: '/docs/sources'},
            {label: 'Search', to: '/docs/search'},
            {label: 'Models', to: '/docs/models'},
            {label: 'AI assistants', to: '/docs/mcp'},
          ],
        },
        {
          title: 'Reference',
          items: [
            {label: 'settings.json keys', to: '/docs/configuration'},
            {label: 'Keyboard shortcuts', to: '/docs/keyboard'},
            {label: 'Troubleshooting', to: '/docs/troubleshooting'},
          ],
        },
        {
          title: 'Project',
          items: [
            {label: 'GitHub', href: REPO},
            {label: 'Releases', href: `${REPO}/releases`},
            {label: 'How it is built', to: '/docs/architecture'},
            {label: 'License (MIT)', href: `${REPO}/blob/main/LICENSE`},
          ],
        },
      ],
      copyright: 'MIT licensed. vectile runs on your machine and talks to no one.',
    },
    prism: {
      theme: prismThemes.oneLight,
      additionalLanguages: ['go', 'bash', 'json', 'sql', 'toml', 'powershell'],
    },
    docs: {
      sidebar: {
        hideable: true,
      },
    },
    tableOfContents: {
      minHeadingLevel: 2,
      maxHeadingLevel: 3,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
