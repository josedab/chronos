import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Chronos',
  tagline: 'Distributed Cron System - Reliable job scheduling without operational complexity',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  url: 'https://chronos.github.io',
  baseUrl: '/',

  organizationName: 'chronos',
  projectName: 'chronos',

  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  themes: [
    [
      '@easyops-cn/docusaurus-search-local',
      {
        hashed: true,
        language: ['en'],
        highlightSearchTermsOnTargetPage: true,
        explicitSearchResultPath: true,
      },
    ],
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/chronos/chronos/tree/main/website/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/chronos-social-card.png',
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    announcementBar: {
      id: 'star_us',
      content: '⭐ If you like Chronos, give it a star on <a target="_blank" rel="noopener noreferrer" href="https://github.com/chronos/chronos">GitHub</a>!',
      backgroundColor: '#4f46e5',
      textColor: '#ffffff',
      isCloseable: true,
    },
    navbar: {
      title: 'Chronos',
      logo: {
        alt: 'Chronos Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Documentation',
        },
        {
          to: '/docs/getting-started/quickstart',
          label: 'Quick Start',
          position: 'left',
        },
        {
          href: 'https://github.com/chronos/chronos',
          position: 'right',
          className: 'header-github-link',
          'aria-label': 'GitHub repository',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {
              label: 'Getting Started',
              to: '/docs/getting-started/quickstart',
            },
            {
              label: 'Core Concepts',
              to: '/docs/core-concepts/architecture',
            },
            {
              label: 'API Reference',
              to: '/docs/reference/api',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub Discussions',
              href: 'https://github.com/chronos/chronos/discussions',
            },
            {
              label: 'Discord',
              href: 'https://discord.gg/chronos',
            },
            {
              label: 'Twitter',
              href: 'https://twitter.com/chronos_cron',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/chronos/chronos',
            },
            {
              label: 'Releases',
              href: 'https://github.com/chronos/chronos/releases',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Chronos Authors. Apache 2.0 License.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'yaml', 'json', 'go', 'docker'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
