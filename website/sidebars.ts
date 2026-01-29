import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/quickstart',
        'getting-started/installation',
        'getting-started/first-job',
      ],
    },
    {
      type: 'category',
      label: 'Core Concepts',
      items: [
        'core-concepts/architecture',
        'core-concepts/jobs',
        'core-concepts/scheduling',
        'core-concepts/webhooks',
        'core-concepts/clustering',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/kubernetes',
        'guides/high-availability',
        'guides/monitoring',
        'guides/authentication',
        'guides/workflows',
        'guides/multi-region',
        'guides/gitops',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/api',
        'reference/cli',
        'reference/configuration',
        'reference/metrics',
      ],
    },
    {
      type: 'category',
      label: 'Advanced',
      items: [
        'advanced/policy-as-code',
        'advanced/secrets',
        'advanced/wasm-plugins',
        'advanced/time-travel',
        'advanced/ai-optimization',
      ],
    },
    {
      type: 'category',
      label: 'Resources',
      items: [
        'resources/comparison',
        'resources/troubleshooting',
        'resources/faq',
        'resources/changelog',
      ],
    },
  ],
};

export default sidebars;
