# Extract Workflow Activity Tracking & Styling

This plan refactors the workflow execution tracking logic so that it can be shared between the main chat and the test panel, and extracts visual states into global CSS.

## Overview
1. Extract `activeAgentNames`, `completedNodes`, and `failedNodes` logic from `chat-event-handlers.js` into a shared utility.
2. Ensure both `agentDesigner` (Test Panel) and the main chat use this uniform logic for processing SSE workflow events.
3. Extract inline pulse animations and CSS classes from `workflow-tree-visualizer.js` into global CSS (`input.css`) so they can be reused.
4. Update `agenteditor.templ` to apply these CSS classes to the editable node cards based on the active tracking state.

## Implementation Steps

### 1. Extract Tracking Logic to `assets/js/utils/workflow-tracker.js`
- Create a new utility exporting `trackWorkflowEvent(event, state)`.
- Move the state mutations for `AGENT_TRANSFER_STARTED` and `WORKFLOW_NODE_COMPLETED` here.

### 2. Update `chat-event-handlers.js`
- Import `trackWorkflowEvent`.
- Replace the inline mutations for transfer and completion events with calls to the utility, ensuring identical functionality.

### 3. Update `agent-designer-dialog.js`
- Add tracking state variables (`testActiveAgentNames: {}`, etc.) to the component.
- In `__testHandleEvent`, handle `AGENT_TRANSFER_STARTED` and `WORKFLOW_NODE_COMPLETED` by calling the shared `trackWorkflowEvent(event, this)`.

### 4. Extract CSS Animations to `assets/css/input.css`
- Extract `@keyframes wfPulseActive` (rename to `nodePulseActive`).
- Extract the `.is-active`, `.is-completed`, and `.is-failed` classes to `.node-active`, `.node-completed`, and `.node-failed`.
- Ensure they are available globally via Tailwind/Custom CSS.

### 5. Update `workflow-tree-visualizer.js`
- Remove the inline style injection block for these classes.
- Update the class names attached to nodes to match the new global names.

### 6. Update `agenteditor.templ`
- Bind the editable node cards (`<div class="editor-node...">`) to compute their states from `typeof testActiveAgentNames !== 'undefined' && testActiveAgentNames[node.agent_id || node.id]`.
- Attach `.node-active` when the card should pulsate according to the tracking state.
