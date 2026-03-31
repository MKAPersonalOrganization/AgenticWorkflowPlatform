/**
 * AlpineFlow — CAD-style workflow canvas for building AgentInc pipelines.
 *
 * Exports:
 *   initFlowStore()           — Call once inside alpine:init to register Alpine.store('flow')
 *   alpineflowCanvas          — Main canvas component (pan/zoom/drop/wire routing)
 *   alpineflowNode(nodeId)    — Individual node card component
 *   alpineflowPalette         — Left-side component library
 *   alpineflowPropertyPanel   — Right-side property inspector
 *   createFlowNode(type, ...) — Factory for new CompositionNode objects
 *
 * Architecture:
 *   All mutable state lives in Alpine.store('flow'). The canvas, nodes, palette and
 *   property panel all read/write through the store. The parent agentDesigner component
 *   syncs pipelineGraph ↔ store on open/save.
 */

// ─── Node ID counter ─────────────────────────────────────────────────────────
let _flowNodeCounter = 100; // start high to avoid collisions with existing node-N ids

function nextFlowId() {
    _flowNodeCounter++;
    return `fnode-${_flowNodeCounter}`;
}

// ─── Type colour palette ──────────────────────────────────────────────────────
const TYPE_COLORS = {
    sequential:  '#7c3aed',
    parallel:    '#0284c7',
    loop:        '#d97706',
    conditional: '#dc2626',
    agent:       '#059669',
    remote:      '#be123c',
};
function typeColor(type) { return TYPE_COLORS[type] || '#475569'; }

// ─── Node factory ─────────────────────────────────────────────────────────────
export function createFlowNode(type, opts = {}) {
    const id = opts.id || nextFlowId();
    if (type === 'agent') {
        return {
            id, type: 'agent',
            label: opts.label || '',
            agent_id: opts.agent_id || '',
            output_key: '',
            disallow_transfer_to_parent: null,
            disallow_transfer_to_peers: null,
            include_contents: '',
            x: opts.x || 0, y: opts.y || 0,
        };
    }
    if (type === 'remote') {
        return {
            id, type: 'remote',
            label: opts.label || '',
            a2a_connector_id: opts.a2a_connector_id || '',
            output_key: '',
            x: opts.x || 0, y: opts.y || 0,
        };
    }
    // Composite: sequential | parallel | loop | conditional
    const node = {
        id, type,
        label: opts.label || '',
        children: [],
        x: opts.x || 0, y: opts.y || 0,
    };
    if (type === 'loop') node.max_iterations = 3;
    // Conditional nodes need an output_key on the evaluator (first child)
    // which is added when the evaluator agent is attached.
    return node;
}

// ─── Shared Store ─────────────────────────────────────────────────────────────
export function initFlowStore() {
    Alpine.store('flow', {
        // Core data
        graph: null,        // CompositionNode tree — mirrors pipelineGraph
        agents: [],         // AgentDefinition list (for palette + labels)
        connectors: [],     // A2A connector list
        systemPrompt: '',   // pipeline-level system prompt (synced on open/save)

        // Interaction state
        selectedId: null,
        connecting: null,       // { sourceId } while dragging a wire
        connectMouseX: 0,       // canvas-space mouse position during wire drag
        connectMouseY: 0,
        canvasEl: null,         // set by alpineflowCanvas.init()

        // Viewport
        panX: 60,
        panY: 60,
        zoom: 1.0,

        // Undo history
        _history: [],
        _histIdx: -1,

        // ── Load / persist ──────────────────────────────────────────────────
        load(graph, agents, connectors, systemPrompt) {
            this.graph = graph ? JSON.parse(JSON.stringify(graph)) : null;
            this.agents = agents || [];
            this.connectors = connectors || [];
            this.systemPrompt = systemPrompt || '';
            this.selectedId = null;
            this.connecting = null;
            this.panX = 60; this.panY = 60; this.zoom = 1.0;
            this._history = [];
            this._histIdx = -1;
            if (this.graph && (this.graph.x === undefined || this.graph.x === 0)) {
                this.autoLayout();
            }
            this._snapshot();
        },

        // Returns a deep copy of the current graph (for saving back to pipelineGraph)
        export() {
            return this.graph ? JSON.parse(JSON.stringify(this.graph)) : null;
        },

        // ── Undo ────────────────────────────────────────────────────────────
        _snapshot() {
            this._history = this._history.slice(0, this._histIdx + 1);
            this._history.push(JSON.parse(JSON.stringify(this.graph)));
            if (this._history.length > 40) this._history.shift();
            this._histIdx = this._history.length - 1;
        },
        undo() {
            if (this._histIdx <= 0) return;
            this._histIdx--;
            this.graph = JSON.parse(JSON.stringify(this._history[this._histIdx]));
            this.selectedId = null;
        },
        canUndo() { return this._histIdx > 0; },

        // ── Node lookup ──────────────────────────────────────────────────────
        nodeById(id, node) {
            const n = node !== undefined ? node : this.graph;
            if (!n) return null;
            if (n.id === id) return n;
            for (const c of (n.children || [])) {
                const found = this.nodeById(id, c);
                if (found) return found;
            }
            return null;
        },

        // Returns { parent, index } or null
        parentOf(id, node) {
            const n = node !== undefined ? node : this.graph;
            if (!n) return null;
            for (let i = 0; i < (n.children || []).length; i++) {
                if (n.children[i].id === id) return { parent: n, index: i };
                const found = this.parentOf(id, n.children[i]);
                if (found) return found;
            }
            return null;
        },

        // Flat list used by x-for loops (adds _depth, _parentId to each)
        get flatNodes() {
            const result = [];
            const walk = (node, depth, parentId) => {
                result.push({ id: node.id, type: node.type, _depth: depth, _parentId: parentId });
                if (!node.collapsed) {
                    for (const c of (node.children || [])) walk(c, depth + 1, node.id);
                }
            };
            if (this.graph) walk(this.graph, 0, null);
            return result;
        },

        // Edge list derived from parent→child relationships
        get edges() {
            const result = [];
            const walk = (node) => {
                if (node.collapsed) return;
                for (let i = 0; i < (node.children || []).length; i++) {
                    result.push({
                        id: `${node.id}→${node.children[i].id}`,
                        sourceId: node.id,
                        targetId: node.children[i].id,
                        order: i,
                    });
                    walk(node.children[i]);
                }
            };
            if (this.graph) walk(this.graph);
            return result;
        },

        // ── Mutations ────────────────────────────────────────────────────────
        moveNode(id, x, y) {
            const n = this.nodeById(id);
            if (n) { n.x = Math.round(x); n.y = Math.round(y); }
        },

        deleteNode(id) {
            if (id === this.graph?.id) return; // can't delete root
            const info = this.parentOf(id);
            if (!info) return;
            info.parent.children.splice(info.index, 1);
            if (this.selectedId === id) this.selectedId = null;
            this._snapshot();
        },

        addChild(parentId, childNode) {
            const parent = this.nodeById(parentId);
            if (!parent || !Array.isArray(parent.children)) return;
            parent.children.push(childNode);
            this._snapshot();
        },

        // Make nodeId a child of newParentId (reparent)
        reparentNode(nodeId, newParentId) {
            if (nodeId === this.graph?.id) return;
            if (nodeId === newParentId) return;
            const info = this.parentOf(nodeId);
            if (!info) return;
            const newParent = this.nodeById(newParentId);
            if (!newParent || !Array.isArray(newParent.children)) return;
            // Prevent making node its own descendant
            let check = newParent;
            while (check) {
                if (check.id === nodeId) return;
                const p = this.parentOf(check.id);
                check = p ? p.parent : null;
            }
            const node = info.parent.children.splice(info.index, 1)[0];
            newParent.children.push(node);
            this._snapshot();
        },

        // Move child up/down within its parent's children array
        reorderChild(parentId, fromIdx, toIdx) {
            const parent = this.nodeById(parentId);
            if (!parent?.children) return;
            const [item] = parent.children.splice(fromIdx, 1);
            parent.children.splice(toIdx, 0, item);
            this._snapshot();
        },

        // ── Auto layout ──────────────────────────────────────────────────────
        autoLayout() {
            if (!this.graph) return;
            const NODE_W = 220, NODE_H = 90, GAP_Y = 110, GAP_X = 28;

            const subtreeW = (node) => {
                if (!node.children || node.children.length === 0 || node.collapsed) return NODE_W;
                const childTotal = node.children.reduce((s, c) => s + subtreeW(c), 0);
                return Math.max(NODE_W, childTotal + GAP_X * (node.children.length - 1));
            };

            const assign = (node, cx, y) => {
                node.x = Math.round(cx - NODE_W / 2);
                node.y = Math.round(y);
                if (!node.children || node.children.length === 0 || node.collapsed) return;
                const totalW = node.children.reduce((s, c) => s + subtreeW(c), 0)
                             + GAP_X * (node.children.length - 1);
                let x = cx - totalW / 2;
                for (const child of node.children) {
                    const cw = subtreeW(child);
                    assign(child, x + cw / 2, y + NODE_H + GAP_Y);
                    x += cw + GAP_X;
                }
            };

            assign(this.graph, 500, 60);
        },

        // ── Label helpers ────────────────────────────────────────────────────
        agentName(agentId) {
            if (!agentId) return '(not set)';
            const a = this.agents.find(a => a.id === agentId);
            return a ? a.name : '(unknown)';
        },
        connectorName(connId) {
            if (!connId) return '(not set)';
            const c = this.connectors.find(c => c.id === connId);
            return c ? (c.name || c.id) : '(unknown)';
        },
    });
}

// ─── Canvas Component ─────────────────────────────────────────────────────────
export const alpineflowCanvas = () => ({
    _isPanning: false,
    _panStartX: 0,
    _panStartY: 0,
    _spaceHeld: false,
    _mmHandler: null,
    _muHandler: null,
    _kdHandler: null,
    _kuHandler: null,

    init() {
        Alpine.store('flow').canvasEl = this.$el;

        this._mmHandler = e => this._globalMouseMove(e);
        this._muHandler = e => this._globalMouseUp(e);
        this._kdHandler = e => this._keyDown(e);
        this._kuHandler = e => this._keyUp(e);

        document.addEventListener('mousemove', this._mmHandler);
        document.addEventListener('mouseup',   this._muHandler);
        document.addEventListener('keydown',   this._kdHandler);
        document.addEventListener('keyup',     this._kuHandler);
    },

    destroy() {
        document.removeEventListener('mousemove', this._mmHandler);
        document.removeEventListener('mouseup',   this._muHandler);
        document.removeEventListener('keydown',   this._kdHandler);
        document.removeEventListener('keyup',     this._kuHandler);
        Alpine.store('flow').canvasEl = null;
    },

    _keyDown(e) {
        if (e.code === 'Space' && document.activeElement === document.body) {
            e.preventDefault();
            this._spaceHeld = true;
        }
        if ((e.metaKey || e.ctrlKey) && e.key === 'z') {
            e.preventDefault();
            Alpine.store('flow').undo();
        }
        if (e.key === 'Delete' || e.key === 'Backspace') {
            const s = Alpine.store('flow');
            if (s.selectedId && s.selectedId !== s.graph?.id) {
                if (document.activeElement.tagName !== 'INPUT' &&
                    document.activeElement.tagName !== 'TEXTAREA' &&
                    document.activeElement.tagName !== 'SELECT') {
                    s.deleteNode(s.selectedId);
                }
            }
        }
        // Fit to view (F key)
        if (e.key === 'f' && !e.metaKey && !e.ctrlKey && !e.altKey) {
            if (document.activeElement.tagName !== 'INPUT' &&
                document.activeElement.tagName !== 'TEXTAREA' &&
                document.activeElement.tagName !== 'SELECT') {
                e.preventDefault();
                this.fitToView();
            }
        }
        // Center graph (C key)
        if (e.key === 'c' && !e.metaKey && !e.ctrlKey && !e.altKey) {
            if (document.activeElement.tagName !== 'INPUT' &&
                document.activeElement.tagName !== 'TEXTAREA' &&
                document.activeElement.tagName !== 'SELECT') {
                e.preventDefault();
                this.centerGraph();
            }
        }
    },
    _keyUp(e) {
        if (e.code === 'Space') this._spaceHeld = false;
    },

    get transform() {
        const s = Alpine.store('flow');
        return `translate(${s.panX}px,${s.panY}px) scale(${s.zoom})`;
    },

    get gridBg() {
        const s    = Alpine.store('flow');
        const size = 32 * s.zoom;
        const ox   = ((s.panX % size) + size) % size;
        const oy   = ((s.panY % size) + size) % size;
        return `background-size:${size}px ${size}px;background-position:${ox}px ${oy}px;`;
    },

    screenToCanvas(sx, sy) {
        const s    = Alpine.store('flow');
        const rect = this.$el.getBoundingClientRect();
        return {
            x: (sx - rect.left - s.panX) / s.zoom,
            y: (sy - rect.top  - s.panY) / s.zoom,
        };
    },

    onWheel(e) {
        e.preventDefault();
        const s      = Alpine.store('flow');
        const rect   = this.$el.getBoundingClientRect();
        const mx     = e.clientX - rect.left;
        const my     = e.clientY - rect.top;
        const factor = e.deltaY > 0 ? 0.88 : 1.14;
        const newZoom = Math.max(0.12, Math.min(3.0, s.zoom * factor));
        s.panX = mx - (mx - s.panX) * (newZoom / s.zoom);
        s.panY = my - (my - s.panY) * (newZoom / s.zoom);
        s.zoom = newZoom;
    },

    onMouseDown(e) {
        if (e.button === 1 || (e.button === 0 && this._spaceHeld)) {
            e.preventDefault();
            this._isPanning = true;
            this._panStartX = e.clientX - Alpine.store('flow').panX;
            this._panStartY = e.clientY - Alpine.store('flow').panY;
            return;
        }
        if (e.button === 0) {
            const s = Alpine.store('flow');
            s.selectedId = null;
            if (s.connecting) s.connecting = null;
        }
    },

    _globalMouseMove(e) {
        const s = Alpine.store('flow');
        if (this._isPanning) {
            s.panX = e.clientX - this._panStartX;
            s.panY = e.clientY - this._panStartY;
        }
        if (s.connecting && s.canvasEl) {
            const rect = s.canvasEl.getBoundingClientRect();
            s.connectMouseX = e.clientX - rect.left;
            s.connectMouseY = e.clientY - rect.top;
        }
    },

    _globalMouseUp() {
        if (this._isPanning) this._isPanning = false;
        const s = Alpine.store('flow');
        if (s.connecting) s.connecting = null;
    },

    onDragOver(e) { e.preventDefault(); },

    onDrop(e) {
        e.preventDefault();
        const type = e.dataTransfer.getData('flowNodeType');
        if (!type || !Alpine.store('flow').graph) return;
        const pos  = this.screenToCanvas(e.clientX, e.clientY);
        const node = createFlowNode(type, {
            agent_id:         e.dataTransfer.getData('flowAgentId')     || '',
            label:            e.dataTransfer.getData('flowAgentName')   || '',
            a2a_connector_id: e.dataTransfer.getData('flowConnectorId') || '',
            x: Math.round(pos.x - 110),
            y: Math.round(pos.y - 45),
        });
        Alpine.store('flow').addChild(Alpine.store('flow').graph.id, node);
    },

    // ── Toolbar actions ──────────────────────────────────────────────────────
    zoomIn()    { const s = Alpine.store('flow'); s.zoom = Math.min(3.0,  +(s.zoom * 1.2).toFixed(2)); },
    zoomOut()   { const s = Alpine.store('flow'); s.zoom = Math.max(0.12, +(s.zoom / 1.2).toFixed(2)); },
    zoomReset() { const s = Alpine.store('flow'); s.zoom = 1; s.panX = 60; s.panY = 60; },
    fitAll()    { Alpine.store('flow').autoLayout(); this.zoomReset(); },
    fitToView() {
        const s = Alpine.store('flow');
        if (!s.graph) return;
        
        // Calculate bounding box of all nodes
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        const NODE_W = 220, NODE_H = 90;
        
        const walk = (node) => {
            if (!node) return;
            const x = node.x || 0;
            const y = node.y || 0;
            minX = Math.min(minX, x);
            minY = Math.min(minY, y);
            maxX = Math.max(maxX, x + NODE_W);
            maxY = Math.max(maxY, y + NODE_H);
            if (!node.collapsed && node.children) {
                node.children.forEach(walk);
            }
        };
        walk(s.graph);
        
        // If no nodes or invalid bounds, reset
        if (minX === Infinity) { this.zoomReset(); return; }
        
        // Get canvas element dimensions
        const canvasEl = s.canvasEl;
        if (!canvasEl) return;
        const rect = canvasEl.getBoundingClientRect();
        const canvasW = rect.width;
        const canvasH = rect.height;
        
        // Calculate graph dimensions with padding
        const graphW = maxX - minX + 80;  // 40px padding each side
        const graphH = maxY - minY + 80;
        
        // Calculate zoom to fit
        const padding = 100; // Extra padding around the graph
        const availableW = canvasW - padding * 2;
        const availableH = canvasH - padding * 2;
        const zoomX = availableW / graphW;
        const zoomY = availableH / graphH;
        let newZoom = Math.min(zoomX, zoomY, 2.0); // Cap at 2x zoom
        newZoom = Math.max(0.12, newZoom); // Min zoom
        
        // Calculate center position
        const graphCenterX = (minX + maxX) / 2;
        const graphCenterY = (minY + maxY) / 2;
        
        // Pan to center the graph
        s.zoom = newZoom;
        s.panX = canvasW / 2 - graphCenterX * newZoom;
        s.panY = canvasH / 2 - graphCenterY * newZoom;
    },
    centerGraph() {
        const s = Alpine.store('flow');
        if (!s.graph) return;
        
        // Calculate center of all nodes
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        const NODE_W = 220, NODE_H = 90;
        
        const walk = (node) => {
            if (!node) return;
            const x = node.x || 0;
            const y = node.y || 0;
            minX = Math.min(minX, x);
            minY = Math.min(minY, y);
            maxX = Math.max(maxX, x + NODE_W);
            maxY = Math.max(maxY, y + NODE_H);
            if (!node.collapsed && node.children) {
                node.children.forEach(walk);
            }
        };
        walk(s.graph);
        
        if (minX === Infinity) return;
        
        // Get canvas element dimensions
        const canvasEl = s.canvasEl;
        if (!canvasEl) return;
        const rect = canvasEl.getBoundingClientRect();
        const canvasW = rect.width;
        const canvasH = rect.height;
        
        // Center point of graph
        const graphCenterX = (minX + maxX) / 2;
        const graphCenterY = (minY + maxY) / 2;
        
        // Pan to center (keeping current zoom)
        s.panX = canvasW / 2 - graphCenterX * s.zoom;
        s.panY = canvasH / 2 - graphCenterY * s.zoom;
    },

    get zoomLabel() { return Math.round(Alpine.store('flow').zoom * 100) + '%'; },
    get canUndo()   { return Alpine.store('flow').canUndo(); },

    // ── SVG edges — rendered via x-html to avoid <template> inside SVG ───────
    get edgesSvgContent() {
        const s = Alpine.store('flow');
        if (!s?.edges) return '';

        // Build arrowhead markers for each type
        let out = '<defs>';
        [
            ['seq',  '#7c3aed'], ['par', '#0284c7'], ['loop', '#d97706'],
            ['agent','#059669'], ['rem', '#be123c'], ['sel',  '#818cf8'],
            ['def',  '#475569'],
        ].forEach(([id, col]) => {
            out += `<marker id="af-arr-${id}" markerWidth="6" markerHeight="6" `
                 + `refX="5" refY="3" orient="auto" markerUnits="strokeWidth">`
                 + `<path d="M0,0.5 L0,5.5 L6,3 z" fill="${col}"/></marker>`;
        });
        out += '</defs>';

        const markerId = (srcType, selected) => {
            if (selected) return 'sel';
            return { sequential:'seq', parallel:'par', loop:'loop', conditional:'def',
                     agent:'agent', remote:'rem' }[srcType] || 'def';
        };

        for (const edge of s.edges) {
            const d = this.edgePath(edge.sourceId, edge.targetId);
            if (!d) continue;
            const src     = s.nodeById(edge.sourceId);
            const sel     = s.selectedId === edge.sourceId || s.selectedId === edge.targetId;
            const col     = sel ? '#818cf8' : (typeColor(src?.type) + '60');
            const mid     = `url(#af-arr-${markerId(src?.type, sel)})`;
            const sw      = sel ? 2 : 1.5;
            out += `<path d="${d}" fill="none" stroke="${col}" stroke-width="${sw}" `
                 + `stroke-linecap="round" marker-end="${mid}"/>`;
            // Sequential order badge
            if (src?.type === 'sequential') {
                const mx = this.edgeMidX(edge.sourceId, edge.targetId);
                const my = this.edgeMidY(edge.sourceId, edge.targetId);
                out += `<circle cx="${mx}" cy="${my}" r="9" fill="#1a1a2e" stroke="#4c1d95" stroke-width="1.5"/>`;
                out += `<text x="${mx}" y="${my+4}" text-anchor="middle" fill="#a78bfa" `
                     + `font-size="9" font-weight="700">${edge.order + 1}</text>`;
            }
        }

        // Live wire during connection drag
        const lw = this.liveWirePath;
        if (lw) {
            out += `<path d="${lw}" fill="none" stroke="#818cf8" stroke-width="2" `
                 + `stroke-dasharray="8 4" stroke-linecap="round" opacity="0.8"/>`;
        }
        return out;
    },

    // ── Edge geometry helpers ─────────────────────────────────────────────────
    edgePath(sourceId, targetId) {
        const s   = Alpine.store('flow');
        const src = s.nodeById(sourceId);
        const tgt = s.nodeById(targetId);
        if (!src || !tgt) return '';
        const x1 = (src.x || 0) + 110;  // center of 220px node
        const y1 = (src.y || 0) + 90;   // bottom port
        const x2 = (tgt.x || 0) + 110;
        const y2 = (tgt.y || 0);        // top port — arrow refX stops 5px short
        const dy = Math.max(45, Math.abs(y2 - y1) * 0.45);
        return `M${x1},${y1} C${x1},${y1+dy} ${x2},${y2-dy} ${x2},${y2}`;
    },

    edgeMidX(sourceId, targetId) {
        const s   = Alpine.store('flow');
        const src = s.nodeById(sourceId);
        const tgt = s.nodeById(targetId);
        if (!src || !tgt) return 0;
        return ((src.x || 0) + (tgt.x || 0)) / 2 + 110;
    },

    edgeMidY(sourceId, targetId) {
        const s   = Alpine.store('flow');
        const src = s.nodeById(sourceId);
        const tgt = s.nodeById(targetId);
        if (!src || !tgt) return 0;
        return ((src.y || 0) + 90 + (tgt.y || 0)) / 2;
    },

    get liveWirePath() {
        const s = Alpine.store('flow');
        if (!s.connecting || !s.canvasEl) return '';
        const src = s.nodeById(s.connecting.sourceId);
        if (!src) return '';
        const x1 = (src.x || 0) + 110;
        const y1 = (src.y || 0) + 90;
        const x2 = (s.connectMouseX - s.panX) / s.zoom;
        const y2 = (s.connectMouseY - s.panY) / s.zoom;
        const dy = Math.max(45, Math.abs(y2 - y1) * 0.45);
        return `M${x1},${y1} C${x1},${y1+dy} ${x2},${y2-dy} ${x2},${y2}`;
    },
});

// ─── Node Component ───────────────────────────────────────────────────────────
export const alpineflowNode = (nodeId) => ({
    nodeId,
    _dragging:  false,
    _startX:    0,
    _startY:    0,
    _dragOffX:  0,
    _dragOffY:  0,
    _mmDrag:    null,
    _muDrag:    null,

    get n()           { return Alpine.store('flow').nodeById(this.nodeId) || {}; },
    get isSelected()  { return Alpine.store('flow').selectedId === this.nodeId; },
    get isRoot()      { return Alpine.store('flow').graph?.id === this.nodeId; },
    get isComposite() { return ['sequential','parallel','loop','conditional'].includes(this.n.type); },

    // ── Computed style helpers ────────────────────────────────────────────────
    get typeColor() { return typeColor(this.n.type); },

    get typeChar() {
        return { sequential:'S', parallel:'P', loop:'L', conditional:'?', agent:'A', remote:'R' }[this.n.type] || '?';
    },

    get typeLabel() {
        return { sequential:'Sequential', parallel:'Parallel', loop:'Loop', conditional:'Conditional',
                 agent:'LLM Agent', remote:'Remote' }[this.n.type] || this.n.type;
    },

    get isUnconfigured() {
        const n = this.n;
        return (n.type === 'agent' && !n.agent_id) ||
               (n.type === 'remote' && !n.a2a_connector_id);
    },

    get nodeStyle() {
        const n = this.n;
        return `left:${n.x||0}px;top:${n.y||0}px;width:220px;`;
    },

    // Card wrapper: selection ring + shadow
    get cardStyle() {
        const col = this.typeColor;
        if (this.isSelected) {
            return `border: 2px solid ${col}; box-shadow: 0 0 0 3px ${col}22, 0 8px 32px ${col}30;`;
        }
        return `border: 1.5px solid ${col}30;`;
    },

    // ── Content helpers ───────────────────────────────────────────────────────
    get displayLabel() {
        const n = this.n;
        if (n.type === 'agent')  return n.label || Alpine.store('flow').agentName(n.agent_id);
        if (n.type === 'remote') return n.label || Alpine.store('flow').connectorName(n.a2a_connector_id);
        return n.label || this.typeLabel;
    },

    get bodyText() {
        const n = this.n;
        if (n.type === 'agent') {
            const name = Alpine.store('flow').agentName(n.agent_id);
            return name === '(not set)' ? '' : name;
        }
        if (n.type === 'remote') {
            const name = Alpine.store('flow').connectorName(n.a2a_connector_id);
            return name === '(not set)' ? '' : name;
        }
        const childCount = (n.children || []).length;
        if (n.type === 'loop')
            return `${childCount} step${childCount !== 1 ? 's' : ''} · max ${n.max_iterations || 3}×`;
        if (n.type === 'conditional')
            return `${Math.max(0, childCount - 1)} branch${childCount - 1 !== 1 ? 'es' : ''} + evaluator`;
        return `${childCount} step${childCount !== 1 ? 's' : ''}`;
    },

    get outputKeyBadge() { return this.n.output_key ? '→ ' + this.n.output_key : ''; },

    // ── Drag to reposition (with 4px threshold) ──────────────────────────────
    onHeaderMouseDown(e) {
        if (e.button !== 0) return;
        e.stopPropagation();
        const s = Alpine.store('flow');
        s.selectedId = this.nodeId;   // select immediately on press

        const canvasEl = s.canvasEl;
        if (!canvasEl) return;
        const rect = canvasEl.getBoundingClientRect();
        this._startX  = e.clientX;
        this._startY  = e.clientY;
        this._dragging = false;
        this._dragOffX = (e.clientX - rect.left - s.panX) / s.zoom - (this.n.x || 0);
        this._dragOffY = (e.clientY - rect.top  - s.panY) / s.zoom - (this.n.y || 0);

        this._mmDrag = ev => {
            // Require 4px movement before starting drag — prevents accidental drags on click
            if (!this._dragging) {
                if (Math.hypot(ev.clientX - this._startX, ev.clientY - this._startY) < 4) return;
                this._dragging = true;
            }
            const rect2 = canvasEl.getBoundingClientRect();
            s.moveNode(
                this.nodeId,
                (ev.clientX - rect2.left - s.panX) / s.zoom - this._dragOffX,
                (ev.clientY - rect2.top  - s.panY) / s.zoom - this._dragOffY,
            );
        };
        this._muDrag = () => {
            if (this._dragging) {
                this._dragging = false;
                Alpine.store('flow')._snapshot();
            }
            document.removeEventListener('mousemove', this._mmDrag);
            document.removeEventListener('mouseup',   this._muDrag);
        };
        document.addEventListener('mousemove', this._mmDrag);
        document.addEventListener('mouseup',   this._muDrag);
    },

    // ── Port interaction ──────────────────────────────────────────────────────
    onOutputPortMouseDown(e) {
        e.preventDefault();
        e.stopPropagation();
        const s = Alpine.store('flow');
        s.selectedId = this.nodeId;
        s.connecting = { sourceId: this.nodeId };
        if (s.canvasEl) {
            const rect = s.canvasEl.getBoundingClientRect();
            s.connectMouseX = e.clientX - rect.left;
            s.connectMouseY = e.clientY - rect.top;
        }
    },

    onInputPortMouseUp(e) {
        e.stopPropagation();
        const s = Alpine.store('flow');
        if (!s.connecting) return;
        const srcId = s.connecting.sourceId;
        s.connecting = null;
        if (srcId === this.nodeId) return;
        const srcNode = s.nodeById(srcId);
        if (!srcNode || !Array.isArray(srcNode.children)) return;
        s.reparentNode(this.nodeId, srcId);
    },

    // ── Context actions ───────────────────────────────────────────────────────
    addChildNode(type) {
        const s  = Alpine.store('flow');
        const n  = this.n;
        const cx = (n.x || 0) + (n.children || []).length * 248;
        const cy = (n.y || 0) + 200;
        s.addChild(this.nodeId, createFlowNode(type, { x: cx, y: cy }));
    },

    deleteThisNode() {
        Alpine.store('flow').deleteNode(this.nodeId);
    },

    toggleCollapse() {
        const n = this.n;
        if (n) n.collapsed = !n.collapsed;
    },
});

// ─── Palette Component ────────────────────────────────────────────────────────
export const alpineflowPalette = () => ({
    search: '',

    get filteredAgents() {
        const q = this.search.toLowerCase().trim();
        return (Alpine.store('flow').agents || []).filter(a =>
            !q || a.name.toLowerCase().includes(q) ||
            (a.description || '').toLowerCase().includes(q)
        );
    },

    get connectors() { return Alpine.store('flow').connectors || []; },

    compositeTypes: [
        { type: 'sequential',  char: 'S', color: '#7c3aed', label: 'Sequential',  desc: 'Steps in order'         },
        { type: 'parallel',    char: 'P', color: '#0284c7', label: 'Parallel',    desc: 'Run concurrently'      },
        { type: 'loop',        char: 'L', color: '#d97706', label: 'Loop',        desc: 'Repeat with limit'     },
        { type: 'conditional', char: '?', color: '#dc2626', label: 'Conditional', desc: 'Route by condition'    },
        { type: 'remote',      char: 'R', color: '#be123c', label: 'Remote A2A',  desc: 'External agent'        },
    ],

    onDragStartType(e, type) {
        e.dataTransfer.setData('flowNodeType', type);
        e.dataTransfer.effectAllowed = 'copy';
    },

    onDragStartAgent(e, agent) {
        e.dataTransfer.setData('flowNodeType',  'agent');
        e.dataTransfer.setData('flowAgentId',   agent.id);
        e.dataTransfer.setData('flowAgentName', agent.name);
        e.dataTransfer.effectAllowed = 'copy';
    },

    onDragStartConnector(e, connector) {
        e.dataTransfer.setData('flowNodeType',    'remote');
        e.dataTransfer.setData('flowConnectorId', connector.id);
        e.dataTransfer.effectAllowed = 'copy';
    },
});

// ─── Property Panel Component ─────────────────────────────────────────────────
export const alpineflowPropertyPanel = () => ({
    // node is kept as a plain reactive data property, updated via Alpine.effect in init().
    // This guarantees reactivity regardless of how Alpine tracks getter dependencies.
    node: null,

    init() {
        // Alpine.effect re-runs whenever reactive dependencies change.
        // Accessing store.selectedId and store.graph inside creates the dependency.
        Alpine.effect(() => {
            const id = Alpine.store('flow').selectedId;
            // Also read graph to re-trigger when the graph mutates (node moves, etc.)
            void Alpine.store('flow').graph;
            this.node = id ? Alpine.store('flow').nodeById(id) : null;
        });
    },

    get agents()     { return Alpine.store('flow').agents     || []; },
    get connectors() { return Alpine.store('flow').connectors || []; },
    get isRoot()     { return Alpine.store('flow').graph?.id === this.node?.id; },
    get parentInfo() { return this.node ? Alpine.store('flow').parentOf(this.node.id) : null; },

    get nodeTypeColor() { return typeColor(this.node?.type); },
    get nodeTypeChar() {
        return { sequential:'S', parallel:'P', loop:'L', conditional:'?', agent:'A', remote:'R' }[this.node?.type] || '?';
    },

    get selectedAgent() {
        const n = this.node;
        if (!n || n.type !== 'agent' || !n.agent_id) return null;
        return (Alpine.store('flow').agents || []).find(a => a.id === n.agent_id) || null;
    },

    get selectedConnector() {
        const n = this.node;
        if (!n || n.type !== 'remote' || !n.a2a_connector_id) return null;
        return (Alpine.store('flow').connectors || []).find(c => c.id === n.a2a_connector_id) || null;
    },

    editAgent() {
        const ag = this.selectedAgent;
        if (!ag) return;
        window.dispatchEvent(new CustomEvent('open-agent-designer-node', { detail: ag }));
    },

    set(key, val) {
        const n = this.node;
        if (n) n[key] = val;
    },

    toggleBool(key) {
        const n = this.node;
        if (n) n[key] = !n[key];
    },

    deleteNode() {
        if (!this.node) return;
        Alpine.store('flow').deleteNode(this.node.id);
    },

    moveUp() {
        const info = this.parentInfo;
        if (!info || info.index === 0) return;
        Alpine.store('flow').reorderChild(info.parent.id, info.index, info.index - 1);
    },

    moveDown() {
        const info = this.parentInfo;
        if (!info) return;
        const maxIdx = info.parent.children.length - 1;
        if (info.index >= maxIdx) return;
        Alpine.store('flow').reorderChild(info.parent.id, info.index, info.index + 1);
    },

    get canMoveUp()   { const i = this.parentInfo; return i && i.index > 0; },
    get canMoveDown() { const i = this.parentInfo; return i && i.index < i.parent.children.length - 1; },

    // ── Conditional routing helpers ────────────────────────────────────────
    get isConditionalChild() {
        const info = this.parentInfo;
        return info && info.parent.type === 'conditional';
    },
    get isEvaluator() {
        // First child of a conditional parent is the evaluator
        return this.isConditionalChild && this.parentInfo?.index === 0;
    },
    get isConditionalBranch() {
        return this.isConditionalChild && this.parentInfo?.index > 0;
    },
});
