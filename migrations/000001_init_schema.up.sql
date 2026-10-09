-- PowerWatch V1 Schema Initial Migration
-- PostgreSQL 15+

-- 1. households
CREATE TABLE households (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. household_members
CREATE TABLE household_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    full_name VARCHAR(100) NOT NULL,
    phone_number VARCHAR(20) UNIQUE NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('OWNER', 'MEMBER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. household_profiles
CREATE TABLE household_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL UNIQUE REFERENCES households(id) ON DELETE CASCADE,
    occupants INT NOT NULL CHECK (occupants > 0),
    feeder_band CHAR(1) NOT NULL CHECK (feeder_band IN ('A', 'B', 'C', 'D', 'E', 'U')),
    ac_count INT NOT NULL DEFAULT 0,
    has_freezer BOOLEAN NOT NULL DEFAULT FALSE,
    has_fridge BOOLEAN NOT NULL DEFAULT FALSE,
    has_pumping_machine BOOLEAN NOT NULL DEFAULT FALSE,
    has_electric_cooker BOOLEAN NOT NULL DEFAULT FALSE,
    has_washing_machine BOOLEAN NOT NULL DEFAULT FALSE,
    baseline_prior_kwh_per_day NUMERIC(6, 2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. meters
CREATE TABLE meters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    meter_number VARCHAR(20) UNIQUE NOT NULL,
    disco VARCHAR(50) NOT NULL,
    estimated_daily_depletion_kwh NUMERIC(6, 2) NOT NULL CHECK (estimated_daily_depletion_kwh > 0),
    confidence_level VARCHAR(10) NOT NULL CHECK (confidence_level IN ('LOW', 'MEDIUM', 'HIGH')),
    rolling_mae_days NUMERIC(5, 2) DEFAULT NULL,
    last_notified_state VARCHAR(20) NOT NULL DEFAULT 'NORMAL' CHECK (last_notified_state IN ('NORMAL', 'EARLY_WARNING', 'CRITICAL')),
    last_notification_sent_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 5. balance_anchors
CREATE TABLE balance_anchors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meter_id UUID NOT NULL REFERENCES meters(id) ON DELETE CASCADE,
    units_kwh NUMERIC(10, 2) NOT NULL CHECK (units_kwh >= 0),
    anchor_type VARCHAR(30) NOT NULL CHECK (anchor_type IN ('VERIFIED_METER_READING', 'CONFIRMED_EMPTY', 'USER_REPORTED_RESIDUAL', 'ESTIMATED_MODEL')),
    source VARCHAR(30) NOT NULL CHECK (source IN ('INITIAL_SETUP', 'RECHARGE_EVENT', 'MANUAL_CALIBRATION')),
    recorded_at TIMESTAMPTZ NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE UNIQUE INDEX idx_single_active_anchor_per_meter ON balance_anchors(meter_id) WHERE is_active = TRUE;

-- 6. recharges
CREATE TABLE recharges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key UUID UNIQUE NOT NULL,
    meter_id UUID NOT NULL REFERENCES meters(id) ON DELETE CASCADE,
    units_purchased_kwh NUMERIC(10, 2) NOT NULL CHECK (units_purchased_kwh > 0),
    amount_paid NUMERIC(10, 2) NOT NULL DEFAULT 0.00 CHECK (amount_paid >= 0.00),
    exhaustion_state VARCHAR(30) NOT NULL CHECK (exhaustion_state IN ('CONFIRMED_EMPTY', 'HAD_UNITS_EXACT', 'HAD_UNITS_BRACKET', 'UNKNOWN')),
    residual_bracket VARCHAR(20) CHECK (residual_bracket IS NULL OR residual_bracket IN ('0-5', '5-10', '10-20', '20-40', '40+')),
    residual_units_kwh NUMERIC(10, 2) DEFAULT NULL CHECK (residual_units_kwh IS NULL OR residual_units_kwh >= 0.00),
    is_void BOOLEAN NOT NULL DEFAULT FALSE,
    recharged_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_recharge_residual_consistency CHECK (
        (exhaustion_state = 'CONFIRMED_EMPTY' AND residual_bracket IS NULL AND (residual_units_kwh IS NULL OR residual_units_kwh = 0.00))
        OR
        (exhaustion_state = 'HAD_UNITS_EXACT' AND residual_bracket IS NULL AND residual_units_kwh IS NOT NULL AND residual_units_kwh >= 0.00)
        OR
        (exhaustion_state = 'HAD_UNITS_BRACKET' AND residual_bracket IS NOT NULL AND residual_bracket IN ('0-5', '5-10', '10-20', '20-40', '40+') AND residual_units_kwh IS NULL)
        OR
        (exhaustion_state = 'UNKNOWN' AND residual_bracket IS NULL AND residual_units_kwh IS NULL)
    )
);

-- 7. cycle_audits
CREATE TABLE cycle_audits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meter_id UUID NOT NULL REFERENCES meters(id) ON DELETE CASCADE,
    start_anchor_id UUID REFERENCES balance_anchors(id),
    cycle_duration_days NUMERIC(6, 2) CHECK (cycle_duration_days > 0),
    units_consumed_kwh NUMERIC(10, 2) CHECK (units_consumed_kwh >= 0),
    observed_rate_kwh_per_day NUMERIC(6, 2) CHECK (observed_rate_kwh_per_day >= 0),
    predicted_duration_days NUMERIC(6, 2),
    absolute_error_days NUMERIC(6, 2),
    cycle_training_weight NUMERIC(3, 2),
    was_used_in_training BOOLEAN NOT NULL DEFAULT FALSE,
    anomaly_flag BOOLEAN NOT NULL DEFAULT FALSE,
    outage_flag BOOLEAN NOT NULL DEFAULT FALSE,
    is_invalidated BOOLEAN NOT NULL DEFAULT FALSE,
    evaluated_at TIMESTAMPTZ NOT NULL
);
