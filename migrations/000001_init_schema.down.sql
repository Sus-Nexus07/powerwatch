-- PowerWatch V1 Schema Down Migration

DROP INDEX IF EXISTS idx_single_active_anchor_per_meter;
DROP TABLE IF EXISTS cycle_audits CASCADE;
DROP TABLE IF EXISTS recharges CASCADE;
DROP TABLE IF EXISTS balance_anchors CASCADE;
DROP TABLE IF EXISTS meters CASCADE;
DROP TABLE IF EXISTS household_profiles CASCADE;
DROP TABLE IF EXISTS household_members CASCADE;
DROP TABLE IF EXISTS households CASCADE;
