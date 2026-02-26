/**
 * App Plugins
 * Main entry point for global Alpine.js components and plugins
 */
import { themePlugin } from './components/theme-plugin.js';
import { pdfViewer } from './components/pdf-viewer.js';
import { resourcesDialog } from './components/resources-dialog.js';
import { toolsSidebar } from './components/tools-sidebar.js';
import { agentsDialog } from './components/agents-dialog.js';
import { convSearchDialog } from './components/conv-search-dialog.js';
import { mcpConnectors } from './components/mcp-connectors.js';

document.addEventListener('alpine:init', () => {
    Alpine.plugin(themePlugin);

    // Register components
    Alpine.data('pdfViewer', pdfViewer);
    Alpine.data('resourcesDialog', resourcesDialog);
    Alpine.data('toolsSidebar', toolsSidebar);
    Alpine.data('agentsDialog', agentsDialog);
    Alpine.data('convSearchDialog', convSearchDialog);
    Alpine.data('mcpConnectors', mcpConnectors);
});
