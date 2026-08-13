import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  integrationSidebar: [
    {
      type: 'category',
      label: 'Mulai',
      collapsed: false,
      items: ['index', 'getting-started/quickstart', 'getting-started/register-client'],
    },
    {
      type: 'category',
      label: 'Cara kerja SSO',
      collapsed: false,
      items: [
        'protocol/authorization-code-pkce',
        'protocol/application-access',
        'security/validation',
        'security/tokens-revocation',
      ],
    },
    {
      type: 'category',
      label: 'Referensi',
      items: [
        'protocol/discovery-jwks',
        'reference/endpoints',
        'reference/scopes-claims',
        'security/errors',
        'reference/standards',
      ],
    },
    {
      type: 'category',
      label: 'Contoh integrasi',
      items: ['integrations/nextjs-node', 'integrations/laravel', 'integrations/go'],
    },
  ],
};

export default sidebars;
