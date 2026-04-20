/**
 * App Plugins
 * Main entry point for global Alpine.js components and plugins
 */
import { themePlugin } from './components/theme-plugin.js';
import { pdfViewer } from './components/pdf-viewer.js';
import { resourcesDialog } from './components/resources-dialog.js';
import { toolsDialog } from './components/tools-sidebar.js';
import { agentsManager } from './components/agents-dialog.js';
import { agentCapabilities } from './components/agent-capabilities.js';
import { agentDesigner } from './components/agent-designer-dialog.js';
import { convSearchDialog } from './components/conv-search-dialog.js';
import { mcpConnectors } from './components/mcp-connectors.js';
import { a2aConnectors } from './components/a2a-connectors.js';
import { evalSetsDialog } from './components/evalsets-dialog.js';
import { metricConfigDialog } from './components/metric-config.js';
import { evaluationDialog } from './components/evaluation-ui.js';
import { settingsManager } from './components/settings-manager.js';
import { workflowTreeBuilder } from './components/workflow-tree-builder.js';
import { workflowTreeVisualizer } from './components/workflow-tree-visualizer.js';
import { workflowGraphPanel } from './components/workflow-graph-panel.js';
import { editableWorkflowTree } from './components/editable-workflow-tree.js';
import { renderMarkdown } from './chat/markdown-renderer.js';
import {
    initFlowStore,
    alpineflowCanvas,
    alpineflowNode,
    alpineflowPalette,
    alpineflowPropertyPanel,
    createFlowNode,
} from './components/alpineflow.js';

// Expose markdown renderer globally so Alpine expressions in any scope can use it
window.renderMarkdown = renderMarkdown;
window.renderToolDescription = (description) => description ? renderMarkdown(description) : '';

// Expose createFlowNode globally so Alpine inline expressions can call it
window.createFlowNode = createFlowNode;

document.addEventListener('alpine:init', () => {
    Alpine.plugin(themePlugin);

    // Register AlpineFlow store first (other components may reference it)
    initFlowStore();

    // Register components
    Alpine.data('pdfViewer', pdfViewer);
    Alpine.data('resourcesDialog', resourcesDialog);
    Alpine.data('toolsDialog', toolsDialog);
    Alpine.data('agentsManager', agentsManager);
    Alpine.data('agentCapabilities', agentCapabilities);
    Alpine.data('agentDesigner', agentDesigner);
    Alpine.data('convSearchDialog', convSearchDialog);
    Alpine.data('mcpConnectors', mcpConnectors);
    Alpine.data('a2aConnectors', a2aConnectors);
    Alpine.data('evalSetsDialog', evalSetsDialog);
    Alpine.data('metricConfigDialog', metricConfigDialog);
    Alpine.data('evaluationDialog', evaluationDialog);
    Alpine.data('settingsManager', settingsManager);
    Alpine.data('workflowTreeBuilder', workflowTreeBuilder);
    Alpine.data('workflowTreeVisualizer', workflowTreeVisualizer);
    Alpine.data('workflowGraphPanel', workflowGraphPanel);
    Alpine.data('editableWorkflowTree', editableWorkflowTree);

    // AlpineFlow canvas components
    Alpine.data('alpineflowCanvas',        alpineflowCanvas);
    Alpine.data('alpineflowNode',          alpineflowNode);
    Alpine.data('alpineflowPalette',       alpineflowPalette);
    Alpine.data('alpineflowPropertyPanel', alpineflowPropertyPanel);
});
