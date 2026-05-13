/**
 * Per-Agent Workflow Status
 *
 * Single source of truth for the agent-status state machine that drives
 * lamp highlighting in the Activity panel.
 *
 * State machine (per agent, identified by name):
 *
 *   PENDING ──ATS──▶ ACTIVE ──WNC(success)──▶ DONE
 *                            ──WNC(error)────▶ ERROR
 *
 * Inputs — *only* workflow lifecycle events, never stream tokens:
 *
 *   • AGENT_TRANSFER_STARTED   → activeAgentNames[toAgent] = true
 *                                (handled by workflow-tracker.js)
 *   • WORKFLOW_NODE_COMPLETED  → completedNodes[name] or failedNodes[name]
 *                                + scheduleHysteresisFlush(name)
 *   • RUN_FINISHED / RUN_ERROR → runEndedAt set; activeAgentNames cleared
 *
 * UX smoothing — visual only, doesn't change semantics:
 *
 *   Between an agent's WNC and the next agent's ATS in the same
 *   sequential branch there's a microsecond gap. To prevent a one-frame
 *   off→on flicker during the handoff, a freshly completed agent is
 *   reported as ACTIVE for `HYSTERESIS_MS` after its WNC. The bridge is
 *   suppressed once the run is settled — the last agent then snaps to
 *   DONE on RUN_FINISHED so no lamp is left stuck green.
 *
 * Precedence in agentStatusFor (highest first):
 *
 *   1. failedNodes[name]                           → ERROR  (terminal)
 *   2. activeAgentNames[name]                      → ACTIVE
 *   3. completedNodes[name] && !runSettled
 *      && (now − completedTs) < HYSTERESIS_MS      → ACTIVE  (bridge)
 *   4. completedNodes[name]                        → DONE
 *   5. otherwise                                   → PENDING
 *
 * Resilience guarantees:
 *
 *   • Status is a pure function of event-derived state; doesn't depend
 *     on the rendering frame rate or on the activity-tick interval.
 *   • Hysteresis is bounded: even if no events ever follow a WNC, the
 *     scheduled flush settles the lamp within HYSTERESIS_MS.
 *   • RUN_STARTED clears per-run state AND cancels in-flight hysteresis
 *     timers — no leakage between runs.
 *   • Out-of-order WNCs are tolerated (each agent's state is
 *     independent; failedNodes wins permanently).
 *   • Multiple ATS for the same agent (loop iterations) keep it ACTIVE
 *     across the iterations; an interleaving WNC + new ATS within
 *     HYSTERESIS_MS appears as continuous ACTIVE.
 */

export const STATUS = Object.freeze({
    PENDING: 'pending',
    ACTIVE:  'active',
    DONE:    'done',
    ERROR:   'error',
});

/** Visual bridge window between an agent's WNC and the next ATS. */
export const HYSTERESIS_MS = 250;

/** Small safety margin so the scheduled flush lands *after* the window. */
const FLUSH_OVERSHOOT_MS = 50;

export function createWorkflowStatusMethods() {
    return {
        // Pending hysteresis flush handles, keyed by agent name. Each
        // WNC reschedules; RUN_STARTED cancels them all.
        _agentHysteresisTimers: {},

        /**
         * Book a one-shot reactivity nudge so the visual bridge for
         * `name` expires after HYSTERESIS_MS even if no other workflow
         * events follow. Idempotent — re-scheduling for the same agent
         * cancels the previous timer.
         */
        scheduleHysteresisFlush(name) {
            if (!name) return;
            const prev = this._agentHysteresisTimers[name];
            if (prev) clearTimeout(prev);
            const handle = setTimeout(() => {
                delete this._agentHysteresisTimers[name];
                // completedNodes is exposed via a getter on the chat
                // component (it returns __currentState.completedNodes),
                // so the write must go through the underlying state
                // slot. Re-assigning the reference is what triggers
                // Alpine reactivity for everything reading
                // .completedNodes — the contents are unchanged, the
                // bridge has simply expired.
                const state = this.__currentState;
                if (state && state.completedNodes && state.completedNodes[name]) {
                    state.completedNodes = { ...state.completedNodes };
                }
            }, HYSTERESIS_MS + FLUSH_OVERSHOOT_MS);
            this._agentHysteresisTimers[name] = handle;
        },

        /** Cancel every pending hysteresis flush. Called on RUN_STARTED. */
        clearHysteresisTimers() {
            for (const h of Object.values(this._agentHysteresisTimers)) {
                clearTimeout(h);
            }
            this._agentHysteresisTimers = {};
        },

        /**
         * True once the run has settled (RUN_FINISHED or RUN_ERROR).
         * Used to suppress the hysteresis bridge on the final agent so
         * its lamp turns off cleanly instead of staying green forever.
         */
        get isWorkflowRunSettled() {
            return !this.isStreaming && !!this.__currentState?.runEndedAt;
        },

        /**
         * Status for a leaf agent. See module header for the
         * precedence rule.
         */
        agentStatusFor(name) {
            if (!name) return STATUS.PENDING;
            if (this.failedNodes && this.failedNodes[name]) return STATUS.ERROR;
            if (this.activeAgentNames && this.activeAgentNames[name]) return STATUS.ACTIVE;
            const completed = this.completedNodes && this.completedNodes[name];
            if (!completed) return STATUS.PENDING;
            if (this.isWorkflowRunSettled) return STATUS.DONE;
            const ts = completed.timestamp ? new Date(completed.timestamp).getTime() : 0;
            if (ts > 0 && Date.now() - ts < HYSTERESIS_MS) return STATUS.ACTIVE;
            return STATUS.DONE;
        },

        /**
         * One-line preview of the value an agent wrote to session
         * state, shown inline under completed leaves in the Activity
         * tree. Returns null when there's nothing to show.
         */
        agentOutputPreview(row) {
            if (!row || !row.outputKey) return null;
            const out = this.workflowOutputs?.[row.outputKey];
            if (!out || out.value == null) return null;
            let s = typeof out.value === 'string' ? out.value : JSON.stringify(out.value);
            s = s.replace(/\s+/g, ' ').trim();
            if (s.length > 80) s = s.slice(0, 77) + '…';
            return { key: row.outputKey, value: s };
        },
    };
}
