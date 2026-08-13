import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

function requiredEnvironment(name: string, value: string | undefined): string {
  const normalized = value?.trim();
  if (!normalized) {
    throw new Error(`${name} wajib diatur melalui documentation/.env`);
  }
  return normalized;
}

const documentationName = requiredEnvironment('DOCS_APP_NAME', process.env.DOCS_APP_NAME);
const documentationTagline = requiredEnvironment('DOCS_TAGLINE', process.env.DOCS_TAGLINE);

const config: Config = {
  title: documentationName,
  tagline: documentationTagline,
  favicon: 'img/favicon.svg',

  url: requiredEnvironment('DOCS_SITE_URL', process.env.DOCS_SITE_URL),
  baseUrl: requiredEnvironment('DOCS_BASE_URL', process.env.DOCS_BASE_URL),
  trailingSlash: false,
  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'id',
    locales: ['id'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.ts',
          breadcrumbs: true,
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    colorMode: {
      defaultMode: 'light',
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: documentationName,
      logo: {
        alt: 'Logo IPNU IPPNU ID',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'integrationSidebar',
          position: 'left',
          label: 'Panduan integrasi',
        },
        {to: '/reference/endpoints', label: 'Endpoint', position: 'left'},
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Dokumentasi',
          items: [
            {label: 'Quickstart', to: '/getting-started/quickstart'},
            {label: 'Contoh integrasi', to: '/integrations/nextjs-node'},
          ],
        },
        {
          title: 'Referensi',
          items: [
            {label: 'Endpoint', to: '/reference/endpoints'},
            {label: 'Scope dan claim', to: '/reference/scopes-claims'},
          ],
        },
      ],
      copyright: `© ${new Date().getFullYear()} ${documentationName}. Dokumentasi integrasi SSO.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'go', 'php'],
    },
    tableOfContents: {
      minHeadingLevel: 2,
      maxHeadingLevel: 3,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
