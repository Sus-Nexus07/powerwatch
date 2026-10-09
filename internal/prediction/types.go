package prediction

// Config holds configurable parameters for the prediction engine.
type Config struct {
	DefaultAlpha            float64 // 0.35
	MinRateMultiplier       float64 // 0.25
	MaxRateMultiplier       float64 // 3.50
	OutageAffectedInfluence float64 // 0.50
	BetaHigh                float64 // 0.85
	BetaMedium              float64 // 0.70
	BetaLow                 float64 // 0.55
	ThresholdEarlyEnter     float64 // 48.0 hrs
	ThresholdEarlyExit      float64 // 60.0 hrs
	ThresholdCriticalEnter  float64 // 24.0 hrs
	ThresholdCriticalExit   float64 // 32.0 hrs
	CriticalCooldownHours   float64 // 12.0 hrs
	FreshnessMultiplier     float64 // 1.50
	StaleExpiryDays         float64 // 7.0 days
}

// DefaultConfig returns the standard configuration parameters for PowerWatch V1.
func DefaultConfig() Config {
	return Config{
		DefaultAlpha:            0.35,
		MinRateMultiplier:       0.25,
		MaxRateMultiplier:       3.50,
		OutageAffectedInfluence: 0.50,
		BetaHigh:                0.85,
		BetaMedium:              0.70,
		BetaLow:                 0.55,
		ThresholdEarlyEnter:     48.0,
		ThresholdEarlyExit:      60.0,
		ThresholdCriticalEnter:  24.0,
		ThresholdCriticalExit:   32.0,
		CriticalCooldownHours:   12.0,
		FreshnessMultiplier:     1.50,
		StaleExpiryDays:         7.0,
	}
}

// FeederBand represents standard DisCo feeder reliability categorizations.
type FeederBand string

const (
	FeederBandA FeederBand = "A"
	FeederBandB FeederBand = "B"
	FeederBandC FeederBand = "C"
	FeederBandD FeederBand = "D"
	FeederBandE FeederBand = "E"
	FeederBandU FeederBand = "U"
)

// ProfileInput models the structural characteristics of a household used to derive prior rates.
type ProfileInput struct {
	Occupants         int
	FeederBand        string
	ACCount           int
	HasFreezer        bool
	HasFridge         bool
	HasPumpingMachine bool
	HasElectricCooker bool
	HasWashingMachine bool
}

// AnchorType represents the persisted balance_anchors.anchor_type values in the PostgreSQL schema.
type AnchorType string

const (
	AnchorTypeVerifiedMeterReading AnchorType = "VERIFIED_METER_READING"
	AnchorTypeConfirmedEmpty       AnchorType = "CONFIRMED_EMPTY"
	AnchorTypeUserReportedResidual AnchorType = "USER_REPORTED_RESIDUAL"
	AnchorTypeEstimatedModel       AnchorType = "ESTIMATED_MODEL"

	// Aliases maintained for evaluation context compatibility
	AnchorTypeHadUnitsExact   AnchorType = "HAD_UNITS_EXACT"
	AnchorTypeHadUnitsBracket AnchorType = "HAD_UNITS_BRACKET"
	AnchorTypeUnknown         AnchorType = "UNKNOWN"
)

// RechargeExhaustionState represents the persisted recharges.exhaustion_state values in the PostgreSQL schema.
type RechargeExhaustionState string

const (
	ExhaustionStateConfirmedEmpty  RechargeExhaustionState = "CONFIRMED_EMPTY"
	ExhaustionStateHadUnitsExact   RechargeExhaustionState = "HAD_UNITS_EXACT"
	ExhaustionStateHadUnitsBracket RechargeExhaustionState = "HAD_UNITS_BRACKET"
	ExhaustionStateUnknown         RechargeExhaustionState = "UNKNOWN"
)

// AuthorityTier represents the epistemic authority tier for cycle calibration.
type AuthorityTier float64

const (
	TierFull    AuthorityTier = 1.0 // Verified meter readings, confirmed empty, exact user residual
	TierPartial AuthorityTier = 0.5 // Bracketed user residual
	TierZero    AuthorityTier = 0.0 // Estimated model, unknown
)

// ConfidenceLevel captures model certainty tiers.
type ConfidenceLevel string

const (
	ConfidenceLow    ConfidenceLevel = "LOW"
	ConfidenceMedium ConfidenceLevel = "MEDIUM"
	ConfidenceHigh   ConfidenceLevel = "HIGH"
)

// AlertState represents notification hysteresis states.
type AlertState string

const (
	AlertStateNormal       AlertState = "NORMAL"
	AlertStateEarlyWarning AlertState = "EARLY_WARNING"
	AlertStateCritical     AlertState = "CRITICAL"
)
