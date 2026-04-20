-- Version: 1.86
-- Description: Drop legacy evaluation tables ahead of the eval subsystem port.
DROP TABLE IF EXISTS evaluation_results CASCADE;
DROP TABLE IF EXISTS evaluation_runs CASCADE;
DROP TABLE IF EXISTS eval_metric_profiles CASCADE;
DROP TABLE IF EXISTS eval_audit_log CASCADE;
DROP TABLE IF EXISTS eval_cases CASCADE;
DROP TABLE IF EXISTS eval_sets CASCADE;
DROP TABLE IF EXISTS rubrics CASCADE;
