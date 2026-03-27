# AlpineFlow: CAD-Based Agent Designer

> Circuit-style drag-and-drop workflow canvas for building AgentInc pipelines using Alpine.js

---

## 1. Overview

The current pipeline composer is a form-based tree builder. We replace it with **AlpineFlow**: a canvas-based node editor where users drag components onto a board, configure them, and wire them together — exactly like drawing a circuit schematic.

Every existing concept maps cleanly to a circuit analogy:

| Circuit | AlpineFlow |
|---------|------------|
| Component | Agent node / Container node |
| Wire / trace | Connection between nodes |
| Input pin | Node input port (receives data/control) |
| Output pin | Node output port (emits result) |
| PCB canvas | Flow canvas |
| Component library | Agent palette (left sidebar) |
| Schematic inspector | Property panel (right sidebar) |

The underlying data model (`CompositionNode` tree) **does not change**. We add only `x`, `y`, `w`, `h` layout fields to store positions. At save-time the canvas serializes back to the existing `Graph *CompositionNode` format the backend already understands.

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│  Agent Designer Dialog  (agent-designer-dialog.js)                  │
│                                                                     │
│  ┌──────────┐   ┌───────────────────────────────┐   ┌──────────┐   │
│  │  Agent   │   │      AlpineFlow Canvas         │   │ Property │   │
│  │ Palette  │   │  (alpineflow-canvas.js)         │   │  Panel   │   │
│  │          │   │                                 │   │          │   │
│  │ • agents │drag│  ┌──────┐     ┌──────────┐    │   │ edit     │   │
│  │ • seq    │──▶│  │Agent │─────▶│Sequential│    │   │ selected │   │
│  │ • par    │   │  │  A   │     │          │    │   │ node     │   │
│  │ • loop   │   │  └──────┘ ┌──▶│  ┌─────┐│    │   │          │   │
│  │ • remote │   │           │   │  │  B  ││    │   │          │   │
│  └──────────┘   │  ┌──────┐─┘   │  ├─────┤│    │   └──────────┘   │
│                 │  │Agent │     │  │  C  ││    │                   │
│                 │  │  D   │     │  └─────┘│    │                   │
│                 │  └──────┘     └──────────┘    │                   │
│                 │         SVG edge layer         │                   │
│                 └───────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────────┘
```

**Component responsibilities:**

| File | Responsibility |
|------|---------------|
| `alpineflow-canvas.js` | Canvas state, pan/zoom, drag, hit testing |
| `alpineflow-node.js` | Individual node rendering + port logic |
| `alpineflow-edges.js` | SVG wire routing (cubic bezier) |
| `alpineflow-palette.js` | Left-side component library |
| `alpineflow-property-panel.js` | Right-side inspector for selected node |
| `agent-designer-dialog.js` | Orchestrates all panels, saves to backend |

All files register via `Alpine.data()` and communicate through a shared **AlpineFlow store** (`Alpine.store('flow', ...)`).

---

## 3. Data Model Changes

### 3.1 Backend — Minimal Change

Add layout fields to `CompositionNode` in `modules/agentstore/types.go`:

```go
type CompositionNode struct {
    // --- existing fields (unchanged) ---
    ID                      string            `json:"id"`
    Type                    NodeType          `json:"type"`
    Label                   string            `json:"label,omitempty"`
    AgentID                 string            `json:"agent_id,omitempty"`
    A2AConnectorID          string            `json:"a2a_connector_id,omitempty"`
    OutputKey               string            `json:"output_key,omitempty"`
    DisallowTransferToParent *bool            `json:"disallow_transfer_to_parent,omitempty"`
    DisallowTransferToPeers  *bool            `json:"disallow_transfer_to_peers,omitempty"`
    IncludeContents          string           `json:"include_contents,omitempty"`
    MaxIterations            int              `json:"max_iterations,omitempty"`
    AllowEarlyExit           bool             `json:"allow_early_exit,omitempty"`
    Children                 []CompositionNode `json:"children,omitempty"`

    // --- NEW: visual layout (ignored by ADK-Go builder) ---
    X         float64 `json:"x,omitempty"`         // Canvas X position (px)
    Y         float64 `json:"y,omitempty"`          // Canvas Y position (px)
    W         float64 `json:"w,omitempty"`          // Node width (px, 0 = auto)
    H         float64 `json:"h,omitempty"`          // Node height (px, 0 = auto)
    Collapsed bool    `json:"collapsed,omitempty"` // Fold composite node
}
```

**Impact:** Zero runtime impact — `adapters/adkgo/workflow.go` ignores unknown fields. No schema migration needed if the `graph` column stores JSONB (new fields are simply preserved).

> If `graph` is stored as a typed struct (not raw JSON), run one migration to add nullable columns or widen the JSONB schema. Check `modules/agentstore/agent_definition_repo.go` scan logic.

### 3.2 Frontend — Graph Representation

The canvas works directly with the `CompositionNode` tree. Flat node + edge lists are computed **on-the-fly** for rendering:

```js
// Derived from the tree, never stored separately
get flatNodes() {
    const result = []
    const walk = (node, depth, parentId) => {
        result.push({ ...node, _depth: depth, _parentId: parentId })
        ;(node.children || []).forEach(c => walk(c, depth + 1, node.id))
    }
    if (this.graph) walk(this.graph, 0, null)
    return result
},

get edges() {
    const result = []
    const walk = (node) => {
        ;(node.children || []).forEach((child, idx) => {
            result.push({ id: `${node.id}→${child.id}`, source: node.id, target: child.id, order: idx })
            walk(child)
        })
    }
    if (this.graph) walk(this.graph)
    return result
},
```

---

## 4. Frontend Implementation

### 4.1 AlpineFlow Store (`Alpine.store('flow', ...)`)

Central shared state — all panels read/write here:

```js
Alpine.store('flow', {
    // The composition tree (same structure saved to backend)
    graph: null,

    // Interaction state
    selectedNodeId: null,
    hoveredNodeId: null,
    connecting: null,        // { sourceId, sourcePort } during wire-drag

    // Canvas viewport
    panX: 0,
    panY: 0,
    zoom: 1.0,               // 0.25 – 2.0
    canvasW: 0,
    canvasH: 0,

    // Drag state
    dragging: null,          // { nodeId, startX, startY, origX, origY }

    // Computed helpers
    get flatNodes() { /* walk tree */ },
    get edges() { /* extract parent→child pairs */ },
    nodeById(id) { /* find in tree */ },
    parentOf(id) { /* find parent node */ },

    // Mutations
    moveNode(id, dx, dy) { /* update x,y on node */ },
    connectNodes(sourceId, targetId) { /* append target as child of source */ },
    disconnectNode(id) { /* remove from parent.children */ },
    reparentNode(id, newParentId, insertIndex) { /* move within tree */ },
    deleteNode(id) { /* remove subtree */ },
    addNode(type, x, y, agentId) { /* create new CompositionNode */ },
    autoLayout() { /* Sugiyama / dagre-style layout */ },
    serialize() { return this.graph },  // → POST body
})
```

### 4.2 Canvas Component (`alpineflow-canvas.js`)

```js
Alpine.data('alpineflowCanvas', () => ({
    init() {
        this.$watch('$store.flow.graph', () => this._scheduleRender())
        this._bindCanvasEvents()
    },

    // Coordinate transforms
    screenToCanvas(sx, sy) {
        const s = this.$store.flow
        return { x: (sx - s.panX) / s.zoom, y: (sy - s.panY) / s.zoom }
    },
    canvasToScreen(cx, cy) {
        const s = this.$store.flow
        return { x: cx * s.zoom + s.panX, y: cy * s.zoom + s.panY }
    },

    // Pan (middle mouse / space+drag)
    onPanStart(e) { /* capture start */ },
    onPanMove(e)  { /* update panX/panY */ },

    // Zoom (wheel)
    onWheel(e) {
        e.preventDefault()
        const delta = e.deltaY > 0 ? 0.9 : 1.1
        const next = Math.max(0.25, Math.min(2.0, this.$store.flow.zoom * delta))
        // zoom toward cursor position
        this.$store.flow.zoom = next
    },

    // Drop from palette
    onDrop(e) {
        const type = e.dataTransfer.getData('nodeType')
        const agentId = e.dataTransfer.getData('agentId')
        const pos = this.screenToCanvas(e.clientX, e.clientY)
        this.$store.flow.addNode(type, pos.x, pos.y, agentId)
    },

    // Background click → deselect
    onBackgroundClick() { this.$store.flow.selectedNodeId = null },
}))
```

**HTML template (inside the dialog):**

```html
<div
    x-data="alpineflowCanvas"
    class="relative overflow-hidden bg-gray-950 rounded-xl border border-white/10"
    style="height: 600px;"
    @wheel.prevent="onWheel"
    @dragover.prevent
    @drop="onDrop"
    @mousedown="onPanStart"
    @mousemove="onPanMove"
    @mouseup="onPanEnd"
    @click.self="onBackgroundClick"
>
    <!-- Grid background (CSS) -->
    <div class="absolute inset-0 pointer-events-none" :style="gridStyle"></div>

    <!-- Transform group — all nodes + edges live here -->
    <div
        class="absolute top-0 left-0 origin-top-left"
        :style="`transform: translate(${$store.flow.panX}px, ${$store.flow.panY}px) scale(${$store.flow.zoom})`"
    >
        <!-- SVG layer for edges (behind nodes) -->
        <svg class="absolute inset-0 overflow-visible pointer-events-none" x-ref="edgeSvg">
            <template x-for="edge in $store.flow.edges" :key="edge.id">
                <g x-data="alpineflowEdge(edge)">
                    <path
                        :d="bezierPath"
                        fill="none"
                        :stroke="edge.id === $store.flow.connectingEdgeId ? '#6366f1' : '#4b5563'"
                        stroke-width="2"
                        stroke-dasharray="none"
                    />
                    <!-- Order badge for sequential nodes -->
                    <circle :cx="midX" :cy="midY" r="8" fill="#1f2937" stroke="#4b5563"/>
                    <text :x="midX" :y="midY + 4" text-anchor="middle" fill="#9ca3af" font-size="9"
                          x-text="edge.order + 1"></text>
                </g>
            </template>

            <!-- Live wire during connection drag -->
            <path x-show="$store.flow.connecting" :d="liveWirePath"
                  fill="none" stroke="#6366f1" stroke-width="2" stroke-dasharray="6 3"/>
        </svg>

        <!-- Nodes layer -->
        <template x-for="node in $store.flow.flatNodes" :key="node.id">
            <div
                x-data="alpineflowNode(node)"
                :style="nodeStyle"
                class="absolute select-none"
            >
                <!-- rendered by alpineflowNode component -->
            </div>
        </template>
    </div>

    <!-- Zoom controls -->
    <div class="absolute bottom-4 right-4 flex gap-1">
        <button @click="$store.flow.zoom = Math.min(2, $store.flow.zoom * 1.2)"
                class="p-1.5 rounded bg-gray-800 text-white text-xs">+</button>
        <button @click="$store.flow.zoom = 1" class="p-1.5 rounded bg-gray-800 text-white text-xs"
                x-text="Math.round($store.flow.zoom * 100) + '%'"></button>
        <button @click="$store.flow.zoom = Math.max(0.25, $store.flow.zoom * 0.8)"
                class="p-1.5 rounded bg-gray-800 text-white text-xs">−</button>
        <button @click="$store.flow.autoLayout()"
                class="p-1.5 rounded bg-gray-800 text-white text-xs ml-2">Auto</button>
    </div>
</div>
```

### 4.3 Node Component (`alpineflow-node.js`)

Each node type gets a distinct visual treatment:

```
┌─────────────────────┐       ┌─────────────────────────────┐
│  ● [icon] Agent Name│       │  ⟳ Sequential               │
│  ─────────────────  │       │  ┌───────────────────────┐  │
│  model: gpt-4o      │       │  │  slot 1  slot 2  slot3 │  │
│                     │       │  └───────────────────────┘  │
│  ○ out              │       │  ○ out                      │
└─────────────────────┘       └─────────────────────────────┘

● = input port     ○ = output port
```

```js
Alpine.data('alpineflowNode', (node) => ({
    node,
    dragging: false,
    dragOffX: 0,
    dragOffY: 0,

    get nodeStyle() {
        const s = this.$store.flow
        return `left:${this.node.x}px; top:${this.node.y}px; width:${this.node.w || this.defaultW}px; z-index:${s.selectedNodeId === this.node.id ? 20 : 10}`
    },

    get isSelected() { return this.$store.flow.selectedNodeId === this.node.id },
    get isComposite() { return ['sequential','parallel','loop'].includes(this.node.type) },
    get defaultW() { return this.isComposite ? 240 : 180 },

    // Node type → color + icon mapping
    get typeStyle() {
        return {
            agent:      { color: 'indigo', icon: '🤖', label: this.node.label || 'Agent' },
            sequential: { color: 'violet', icon: '▶▶',  label: this.node.label || 'Sequential' },
            parallel:   { color: 'sky',    icon: '⫠',   label: this.node.label || 'Parallel' },
            loop:       { color: 'amber',  icon: '↺',   label: this.node.label || 'Loop' },
            remote:     { color: 'emerald',icon: '🌐',  label: this.node.label || 'Remote' },
        }[this.node.type]
    },

    // --- Drag to move ---
    onDragStart(e) {
        if (e.target.dataset.port) return  // port drag handled separately
        this.dragging = true
        const rect = this.$el.getBoundingClientRect()
        this.dragOffX = (e.clientX - rect.left) / this.$store.flow.zoom
        this.dragOffY = (e.clientY - rect.top)  / this.$store.flow.zoom
        this.$store.flow.selectedNodeId = this.node.id
        document.addEventListener('mousemove', this._onDragMove = e => this.onDragMove(e))
        document.addEventListener('mouseup',   this._onDragEnd  = e => this.onDragEnd(e))
    },
    onDragMove(e) {
        if (!this.dragging) return
        const canvas = this.$store.flow
        const cx = (e.clientX - canvas.panX) / canvas.zoom - this.dragOffX
        const cy = (e.clientY - canvas.panY) / canvas.zoom - this.dragOffY
        canvas.moveNode(this.node.id, cx, cy)
    },
    onDragEnd(e) {
        this.dragging = false
        document.removeEventListener('mousemove', this._onDragMove)
        document.removeEventListener('mouseup',   this._onDragEnd)
    },

    // --- Port connection ---
    onPortMouseDown(e, portType) {
        e.stopPropagation()
        this.$store.flow.connecting = { sourceId: this.node.id, sourcePort: portType }
        document.addEventListener('mouseup', this._onPortRelease = e => this.onPortRelease(e))
    },
    onPortMouseUp(e, portType) {
        const conn = this.$store.flow.connecting
        if (!conn || conn.sourceId === this.node.id) return
        this.$store.flow.connectNodes(conn.sourceId, this.node.id)
        this.$store.flow.connecting = null
    },
    onPortRelease(e) {
        // Released on background — cancel
        this.$store.flow.connecting = null
        document.removeEventListener('mouseup', this._onPortRelease)
    },

    // Context menu (right-click)
    onContextMenu(e) {
        e.preventDefault()
        this.$dispatch('flow-context-menu', { nodeId: this.node.id, x: e.clientX, y: e.clientY })
    },
}))
```

**Node HTML template:**

```html
<template x-data="alpineflowNode(node)">
<div
    class="rounded-lg border-2 cursor-grab active:cursor-grabbing transition-shadow"
    :class="{
        'border-indigo-500 shadow-indigo-500/30 shadow-lg': isSelected,
        'border-white/10 hover:border-white/25': !isSelected,
        'bg-gray-900': true
    }"
    @mousedown="onDragStart"
    @click.stop="$store.flow.selectedNodeId = node.id"
    @contextmenu="onContextMenu"
>
    <!-- Input port -->
    <div class="flex justify-center -mt-2.5" x-show="node._parentId !== null || node.type !== node._rootType">
        <div
            data-port="input"
            class="w-4 h-4 rounded-full bg-gray-700 border-2 border-gray-500 hover:border-indigo-400 cursor-crosshair transition-colors"
            @mouseup="onPortMouseUp($event, 'input')"
        ></div>
    </div>

    <!-- Header bar -->
    <div class="px-3 py-2 flex items-center gap-2 border-b border-white/10">
        <span x-text="typeStyle.icon" class="text-sm"></span>
        <span class="text-xs font-semibold text-white truncate flex-1" x-text="typeStyle.label"></span>
        <span class="text-xs px-1.5 py-0.5 rounded text-gray-400 font-mono"
              :class="`bg-${typeStyle.color}-900/40 text-${typeStyle.color}-300`"
              x-text="node.type"></span>
    </div>

    <!-- Body: agent detail or composite children slots -->
    <div class="px-3 py-2 text-xs text-gray-400">
        <template x-if="node.type === 'agent'">
            <div>
                <div x-text="node.agentName || node.agent_id" class="truncate text-gray-300"></div>
                <div x-show="node.output_key" class="mt-0.5 font-mono text-indigo-300" x-text="'→ ' + node.output_key"></div>
            </div>
        </template>
        <template x-if="isComposite">
            <div class="text-gray-500 italic">
                <span x-text="(node.children||[]).length"></span> children
                <span x-show="node.type === 'loop'" x-text="' · max ' + node.max_iterations + 'x'"></span>
            </div>
        </template>
        <template x-if="node.type === 'remote'">
            <div class="truncate text-emerald-300" x-text="node.connectorName || node.a2a_connector_id"></div>
        </template>
    </div>

    <!-- Output port -->
    <div class="flex justify-center -mb-2.5">
        <div
            data-port="output"
            class="w-4 h-4 rounded-full bg-gray-700 border-2 border-gray-500 hover:border-indigo-400 cursor-crosshair transition-colors"
            @mousedown="onPortMouseDown($event, 'output')"
        ></div>
    </div>
</div>
</template>
```

### 4.4 Edge Rendering (`alpineflow-edges.js`)

Cubic bezier curves connecting output port of parent to input port of child:

```js
Alpine.data('alpineflowEdge', (edge) => ({
    edge,
    get sourceNode() { return this.$store.flow.nodeById(this.edge.source) },
    get targetNode() { return this.$store.flow.nodeById(this.edge.target) },

    // Port center positions (relative to canvas origin)
    get srcPt() {
        const n = this.sourceNode
        const w = n.w || 180
        return { x: n.x + w / 2, y: n.y + (n.h || 80) + 10 }  // bottom center
    },
    get tgtPt() {
        const n = this.targetNode
        const w = n.w || 180
        return { x: n.x + w / 2, y: n.y - 10 }  // top center
    },
    get midX() { return (this.srcPt.x + this.tgtPt.x) / 2 },
    get midY() { return (this.srcPt.y + this.tgtPt.y) / 2 },

    // Cubic bezier: vertical tangents for cleaner routing
    get bezierPath() {
        const { x: x1, y: y1 } = this.srcPt
        const { x: x2, y: y2 } = this.tgtPt
        const dy = Math.max(40, Math.abs(y2 - y1) * 0.5)
        return `M ${x1} ${y1} C ${x1} ${y1 + dy}, ${x2} ${y2 - dy}, ${x2} ${y2}`
    },
}))
```

### 4.5 Palette Component (`alpineflow-palette.js`)

Left sidebar — drag source for creating new nodes:

```js
Alpine.data('alpineflowPalette', () => ({
    search: '',
    agentDefs: [],   // loaded from /v1/agent-definitions

    async init() {
        const r = await fetch('/v1/agent-definitions')
        this.agentDefs = await r.json()
    },

    get filteredAgents() {
        const q = this.search.toLowerCase()
        return this.agentDefs.filter(a => a.name.toLowerCase().includes(q))
    },

    // Built-in node types
    compositeTypes: [
        { type: 'sequential', icon: '▶▶', label: 'Sequential', desc: 'Run agents in order' },
        { type: 'parallel',   icon: '⫠',  label: 'Parallel',  desc: 'Run agents concurrently' },
        { type: 'loop',       icon: '↺',  label: 'Loop',      desc: 'Repeat until condition' },
        { type: 'remote',     icon: '🌐', label: 'Remote A2A', desc: 'Call external agent' },
    ],

    onDragStartBuiltin(e, type) {
        e.dataTransfer.setData('nodeType', type)
        e.dataTransfer.effectAllowed = 'copy'
    },

    onDragStartAgent(e, agent) {
        e.dataTransfer.setData('nodeType', 'agent')
        e.dataTransfer.setData('agentId', agent.id)
        e.dataTransfer.setData('agentName', agent.name)
        e.dataTransfer.effectAllowed = 'copy'
    },
}))
```

**Palette HTML:**

```html
<div x-data="alpineflowPalette" class="w-56 flex-none border-r border-white/10 flex flex-col">
    <div class="p-3 border-b border-white/10">
        <input x-model="search" type="text" placeholder="Search agents..."
               class="w-full text-xs bg-gray-800 rounded px-2 py-1.5 text-white placeholder-gray-500 outline-none"/>
    </div>

    <!-- Composite node types -->
    <div class="p-2 border-b border-white/10">
        <p class="text-[10px] uppercase tracking-wider text-gray-500 mb-1.5 px-1">Containers</p>
        <template x-for="t in compositeTypes" :key="t.type">
            <div
                draggable="true"
                @dragstart="onDragStartBuiltin($event, t.type)"
                class="flex items-center gap-2 p-2 rounded-lg hover:bg-white/5 cursor-grab active:cursor-grabbing mb-0.5"
            >
                <span x-text="t.icon" class="w-5 text-center"></span>
                <div>
                    <div class="text-xs text-white font-medium" x-text="t.label"></div>
                    <div class="text-[10px] text-gray-500" x-text="t.desc"></div>
                </div>
            </div>
        </template>
    </div>

    <!-- Agent definitions -->
    <div class="flex-1 overflow-y-auto p-2">
        <p class="text-[10px] uppercase tracking-wider text-gray-500 mb-1.5 px-1">Agents</p>
        <template x-for="agent in filteredAgents" :key="agent.id">
            <div
                draggable="true"
                @dragstart="onDragStartAgent($event, agent)"
                class="flex items-center gap-2 p-2 rounded-lg hover:bg-white/5 cursor-grab active:cursor-grabbing mb-0.5"
            >
                <span class="text-sm">🤖</span>
                <div class="min-w-0">
                    <div class="text-xs text-white font-medium truncate" x-text="agent.name"></div>
                    <div class="text-[10px] text-gray-500 truncate" x-text="agent.description"></div>
                </div>
            </div>
        </template>
    </div>
</div>
```

### 4.6 Property Panel (`alpineflow-property-panel.js`)

Right sidebar — shows and edits the selected node's configuration:

```js
Alpine.data('alpineflowPropertyPanel', () => ({
    get selected() {
        const id = this.$store.flow.selectedNodeId
        return id ? this.$store.flow.nodeById(id) : null
    },

    // Live-edit helpers — directly mutate store tree
    set(key, val) {
        if (!this.selected) return
        this.$store.flow.nodeById(this.selected.id)[key] = val
    },

    deleteSelected() {
        if (!this.selected) return
        if (!confirm(`Delete "${this.selected.label || this.selected.type}" and all children?`)) return
        this.$store.flow.deleteNode(this.selected.id)
    },

    duplicateSelected() {
        // Deep clone node + new IDs, place offset by 20px
        this.$store.flow.duplicateNode(this.selected.id)
    },

    fitToChildren() {
        // Auto-resize composite node to enclose children
        this.$store.flow.fitNodeToChildren(this.selected.id)
    },
}))
```

**Property panel HTML:**

```html
<div x-data="alpineflowPropertyPanel"
     class="w-64 flex-none border-l border-white/10 overflow-y-auto">
    <template x-if="!selected">
        <div class="p-4 text-xs text-gray-500 italic">Select a node to edit its properties.</div>
    </template>

    <template x-if="selected">
        <div class="p-3 space-y-3">
            <!-- Header -->
            <div class="flex items-center justify-between">
                <h3 class="text-sm font-semibold text-white" x-text="selected.type"></h3>
                <div class="flex gap-1">
                    <button @click="duplicateSelected" title="Duplicate"
                            class="p-1 hover:bg-white/10 rounded text-gray-400">⧉</button>
                    <button @click="deleteSelected" title="Delete"
                            class="p-1 hover:bg-red-900/40 rounded text-red-400">✕</button>
                </div>
            </div>

            <!-- Label -->
            <div>
                <label class="text-[10px] uppercase tracking-wider text-gray-500">Label</label>
                <input :value="selected.label"
                       @input="set('label', $event.target.value)"
                       class="w-full mt-1 text-xs bg-gray-800 rounded px-2 py-1.5 text-white outline-none"/>
            </div>

            <!-- Agent-specific -->
            <template x-if="selected.type === 'agent'">
                <div class="space-y-2">
                    <div>
                        <label class="text-[10px] uppercase tracking-wider text-gray-500">Output Key</label>
                        <input :value="selected.output_key"
                               @input="set('output_key', $event.target.value)"
                               placeholder="session variable name"
                               class="w-full mt-1 text-xs bg-gray-800 rounded px-2 py-1.5 text-white font-mono outline-none"/>
                    </div>
                    <div class="flex gap-3">
                        <label class="flex items-center gap-1.5 text-xs text-gray-400">
                            <input type="checkbox"
                                   :checked="selected.disallow_transfer_to_parent"
                                   @change="set('disallow_transfer_to_parent', $event.target.checked)"/>
                            No parent transfer
                        </label>
                        <label class="flex items-center gap-1.5 text-xs text-gray-400">
                            <input type="checkbox"
                                   :checked="selected.disallow_transfer_to_peers"
                                   @change="set('disallow_transfer_to_peers', $event.target.checked)"/>
                            No peer transfer
                        </label>
                    </div>
                </div>
            </template>

            <!-- Loop-specific -->
            <template x-if="selected.type === 'loop'">
                <div class="space-y-2">
                    <div>
                        <label class="text-[10px] uppercase tracking-wider text-gray-500">Max Iterations</label>
                        <input type="number" min="1"
                               :value="selected.max_iterations || 5"
                               @input="set('max_iterations', parseInt($event.target.value))"
                               class="w-full mt-1 text-xs bg-gray-800 rounded px-2 py-1.5 text-white outline-none"/>
                    </div>
                    <label class="flex items-center gap-1.5 text-xs text-gray-400">
                        <input type="checkbox"
                               :checked="selected.allow_early_exit"
                               @change="set('allow_early_exit', $event.target.checked)"/>
                        Allow early exit
                    </label>
                </div>
            </template>

            <!-- Include contents -->
            <div>
                <label class="text-[10px] uppercase tracking-wider text-gray-500">Context Forwarding</label>
                <select :value="selected.include_contents || 'default'"
                        @change="set('include_contents', $event.target.value)"
                        class="w-full mt-1 text-xs bg-gray-800 rounded px-2 py-1.5 text-white outline-none">
                    <option value="default">Default</option>
                    <option value="none">None</option>
                </select>
            </div>

            <!-- Actions -->
            <template x-if="selected.type !== 'agent' && selected.type !== 'remote'">
                <button @click="fitToChildren"
                        class="w-full text-xs py-1.5 bg-gray-800 hover:bg-gray-700 rounded text-gray-300 transition-colors">
                    Fit to children
                </button>
            </template>
        </div>
    </template>
</div>
```

---

## 5. Auto Layout Algorithm

When nodes have no `x,y` (first open of legacy pipelines, or after "Auto" button), use a simple Sugiyama-inspired top-down layout:

```js
autoLayout() {
    // 1. Assign levels (BFS from root)
    // 2. Within each level, sort nodes left to right
    // 3. Position: y = level * 140, x = centered on parent

    const LEVEL_GAP = 140   // vertical spacing
    const NODE_W    = 200   // assumed node width
    const NODE_GAP  = 20    // horizontal gap between siblings

    const assignPositions = (node, depth, siblingIndex, siblingCount, parentCenterX) => {
        const totalW = siblingCount * NODE_W + (siblingCount - 1) * NODE_GAP
        const startX = parentCenterX - totalW / 2
        node.x = startX + siblingIndex * (NODE_W + NODE_GAP)
        node.y = depth * LEVEL_GAP
        ;(node.children || []).forEach((child, i) =>
            assignPositions(child, depth + 1, i, node.children.length, node.x + NODE_W / 2)
        )
    }

    if (this.graph) {
        assignPositions(this.graph, 0, 0, 1, 600)  // 600 = canvas center
    }
},
```

---

## 6. Integration with Agent Designer Dialog

### 6.1 Mode Switch

The existing dialog has two tabs: **Node** (simple agent) and **Pipeline** (compose). We add a **third tab: Flow Canvas**.

In `agent-designer-dialog.js`, update `composePipelineView` to toggle between:
- `'tree'` — existing form-based tree builder (keep for power users)
- `'canvas'` — new AlpineFlow CAD canvas

```js
// In agentDesigner() data object, add:
flowView: 'canvas',   // 'canvas' | 'tree'
```

```html
<!-- Tab switcher inside the compose mode header -->
<div class="flex gap-1 rounded-lg bg-gray-800 p-0.5">
    <button @click="flowView = 'canvas'"
            :class="flowView === 'canvas' ? 'bg-gray-700 text-white' : 'text-gray-400'"
            class="px-3 py-1 rounded-md text-xs transition-colors">Canvas</button>
    <button @click="flowView = 'tree'"
            :class="flowView === 'tree' ? 'bg-gray-700 text-white' : 'text-gray-400'"
            class="px-3 py-1 rounded-md text-xs transition-colors">Tree</button>
</div>

<!-- Canvas view -->
<template x-if="flowView === 'canvas'">
    <div class="flex h-[600px]">
        <div x-data="alpineflowPalette"><!-- palette --></div>
        <div x-data="alpineflowCanvas" class="flex-1"><!-- canvas --></div>
        <div x-data="alpineflowPropertyPanel"><!-- inspector --></div>
    </div>
</template>

<!-- Tree view (existing) -->
<template x-if="flowView === 'tree'">
    <!-- existing tree builder UI -->
</template>
```

### 6.2 Save Flow

The canvas writes directly into the `graph` state already used by `savePipeline()`. No change to the save logic:

```js
// In agent-designer-dialog.js savePipeline() — unchanged
async savePipeline() {
    // ...existing validation...
    const body = {
        ...this.pipelineForm,
        graph: this.$store.flow.graph,  // picks up x,y positions automatically
    }
    // POST/PUT to /v1/agent-definitions
}
```

### 6.3 Load Flow

When editing an existing pipeline, initialize the store:

```js
showEditPipelineForm(agent) {
    this.$store.flow.graph = agent.graph
    // If no positions yet, auto-layout
    if (agent.graph && agent.graph.x === undefined) {
        this.$store.flow.autoLayout()
    }
    this.flowView = 'canvas'
    // ...rest of existing logic...
}
```

---

## 7. Backend Changes Summary

### Required

| File | Change | Why |
|------|--------|-----|
| `modules/agentstore/types.go` | Add `X, Y, W, H float64` and `Collapsed bool` to `CompositionNode` | Store canvas positions |

That is the **only required backend change**.

### Optional Enhancements

| File | Change | Why |
|------|--------|-----|
| `modules/agentstore/agentstore.go` | Add `GET /v1/agent-definitions?with_layout=true` | Separate endpoint for canvas (avoids sending layout data in every list call if large) |
| `modules/agentstore/agent_definition_repo.go` | Ensure JSONB column round-trips unknown fields | Prevents position data from being dropped on update if struct scan strips unknowns |

### No Change Required

- `adapters/adkgo/workflow.go` — ignores `x, y, w, h, collapsed`
- `foundation/agent/agent.go` — interfaces unchanged
- All runtime execution paths — layout is purely presentational

---

## 8. Database Migration

If `graph` is stored as JSONB (`jsonb` column type), **no migration needed** — new fields are stored transparently.

If `graph` is stored as a typed Go struct that is marshaled/unmarshaled without preserving unknown fields, add one migration to ensure round-trip safety:

```sql
-- No schema change needed if column is already jsonb
-- Only needed if you're adding an index or separating layout from graph:
ALTER TABLE agent_definitions
    ADD COLUMN IF NOT EXISTS graph_layout jsonb;
-- (optional: move x/y into a separate column to avoid polluting the execution graph)
```

The simpler path is to keep `x, y` inside the `graph` JSONB — the ADK-Go builder ignores them.

---

## 9. Implementation Phases

### Phase 1 — Core Canvas (1–2 weeks)
- [ ] Create `alpineflow-canvas.js` with pan, zoom, drop
- [ ] Create `alpineflow-node.js` with drag-to-move
- [ ] Create `alpineflow-edges.js` with bezier wires
- [ ] Add `x, y, w, h` to `CompositionNode` (backend)
- [ ] Auto-layout on first open of existing pipelines
- [ ] Integrate Canvas tab into agent-designer-dialog

### Phase 2 — Full Interaction (1 week)
- [ ] Port-to-port connection drawing (live wire)
- [ ] Disconnect (right-click → remove connection)
- [ ] Node deletion with child cleanup
- [ ] Duplicate node subtree
- [ ] Undo/redo stack (`history: []` of graph snapshots)

### Phase 3 — Property Panel & Polish (1 week)
- [ ] Full property panel for all node types
- [ ] Collapsed composite nodes (click to expand/collapse)
- [ ] Minimap (SVG thumbnail in corner)
- [ ] Keyboard shortcuts: Delete, Ctrl+Z, Ctrl+D, Ctrl+A, Escape
- [ ] Selection box (drag to select multiple nodes)

### Phase 4 — Validation & UX (0.5 weeks)
- [ ] Visual validation: red highlight on invalid nodes
- [ ] Constraint badges (depth counter, node count)
- [ ] Animated execution overlay (highlight active node during run)

---

## 10. Key Design Decisions

### Why not use a dedicated graph library (React Flow, Drawflow, etc.)?

Our stack is Go+templ+Alpine.js. Bringing in a React-based library would require a build pipeline mismatch. Pure Alpine.js + SVG gives us:
- Zero additional dependencies
- SSR-compatible (templ renders the dialog shell; Alpine hydrates)
- Full control over look and feel

### Why keep the tree model instead of moving to nodes+edges arrays?

The existing `CompositionNode` tree maps 1:1 to ADK-Go's agent composition API. A pure graph with explicit edge arrays would require a conversion step and introduce ambiguity (e.g., can a node have two parents?). The tree constraint is actually a feature — it prevents invalid topologies and matches ADK-Go's execution model.

### Why store `x, y` inside `CompositionNode` instead of a separate layout document?

Simplicity: one field (`graph`) stores the full designer state. The ADK-Go builder ignores unknown fields. If the layout data ever needs to be separated (e.g., for performance), it's straightforward to extract into a `graph_layout` column.

---

## 11. File Inventory

**New files to create:**

```
assets/js/components/
├── alpineflow-canvas.js          # Canvas host (pan/zoom/drop)
├── alpineflow-node.js            # Individual node component
├── alpineflow-edges.js           # SVG edge renderer
├── alpineflow-palette.js         # Left sidebar component library
└── alpineflow-property-panel.js  # Right sidebar inspector
```

**Files to modify:**

```
assets/js/components/agent-designer-dialog.js  # Add flowView tab + store init
modules/agentstore/types.go                     # Add x,y,w,h,collapsed to CompositionNode
components/*.templ                              # Load new JS files
```

**Total new JS:** ~5 files, ~600–800 lines
**Backend changes:** 5 new fields on `CompositionNode` struct
