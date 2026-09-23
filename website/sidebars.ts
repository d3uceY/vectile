import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    'what-is-vectile',
    {
      type: 'category',
      label: 'Getting started',
      collapsed: false,
      items: ['download', 'first-run', 'sources'],
    },
    {
      type: 'category',
      label: 'Using vectile',
      collapsed: false,
      items: ['search', 'library', 'indexing', 'models', 'mcp', 'ocr'],
    },
    {
      type: 'category',
      label: 'Reference',
      items: ['settings', 'configuration', 'keyboard', 'troubleshooting'],
    },
    {
      type: 'category',
      label: 'Under the hood',
      items: ['architecture', 'build-from-source', 'privacy'],
    },
  ],
};

export default sidebars;
