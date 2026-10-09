package prediction

import (
	"math"
	"strings"
	"time"
)

// CalculatePrior computes the baseline prior daily electricity consumption rate (kWh/day)
// based on household demographics, appliance inventory, and feeder reliability band.
// Formula: R_prior = (B_base + sum(W_appliances)) * F_band
// B_base = 1.0 + (0.4 * (occupants - 1))
func CalculatePrior(profile ProfileInput) float64 {
	occupants := profile.Occupants
	if occupants < 1 {
		occupants = 1
	}

	base := 1.0 + (0.4 * float64(occupants-1))

	var applianceSum float64
	if profile.ACCount > 0 {
		applianceSum += 4.5 * float64(profile.ACCount)
	}
	if profile.HasFreezer {
		applianceSum += 2.5
	}
	if profile.HasFridge {
		applianceSum += 1.5
	}
	if profile.HasPumpingMachine {
		applianceSum += 1.0
	}
	if profile.HasElectricCooker {
		applianceSum += 3.5
	}
	if profile.HasWashingMachine {
		applianceSum += 0.8
	}

	var bandFactor float64
	switch strings.ToUpper(strings.TrimSpace(profile.FeederBand)) {
	case string(FeederBandA):
		bandFactor = 1.00
	case string(FeederBandB):
		bandFactor = 0.85
	case string(FeederBandC):
		bandFactor = 0.65
	case string(FeederBandD), string(FeederBandE):
		bandFactor = 0.50
	case string(FeederBandU):
		bandFactor = 0.75
	default:
		bandFactor = 0.75
	}

	return (base + applianceSum) * bandFactor
}

// DeriveBalance determines the elapsed time in days and the current estimated balance in kWh.
// Guarantees non-negativity: estimatedBalance >= 0.0 and elapsedDays >= 0.0.
func DeriveBalance(anchorUnits float64, recordedAt time.Time, currentRatePerDay float64, now time.Time) (float64, float64) {
	if anchorUnits < 0.0 || math.IsNaN(anchorUnits) {
		anchorUnits = 0.0
	}
	if currentRatePerDay < 0.0 || math.IsNaN(currentRatePerDay) || math.IsInf(currentRatePerDay, 0) {
		currentRatePerDay = 0.0
	}

	elapsedHours := now.Sub(recordedAt).Hours()
	if elapsedHours < 0.0 || math.IsNaN(elapsedHours) {
		elapsedHours = 0.0
	}
	elapsedDays := elapsedHours / 24.0

	depletion := currentRatePerDay * elapsedDays
	estimatedBalance := anchorUnits - depletion
	if estimatedBalance < 0.0 || math.IsNaN(estimatedBalance) {
		estimatedBalance = 0.0
	}

	return estimatedBalance, elapsedDays
}

// MapRechargeToAnchor translates a recharge exhaustion state into its database AnchorType
// and its corresponding epistemic AuthorityTier.
// Mapping:
// - CONFIRMED_EMPTY  -> (AnchorTypeConfirmedEmpty, TierFull)
// - HAD_UNITS_EXACT   -> (AnchorTypeUserReportedResidual, TierFull)
// - HAD_UNITS_BRACKET -> (AnchorTypeUserReportedResidual, TierPartial)
// - UNKNOWN           -> (AnchorTypeEstimatedModel, TierZero)
func MapRechargeToAnchor(state RechargeExhaustionState) (AnchorType, AuthorityTier) {
	switch state {
	case ExhaustionStateConfirmedEmpty:
		return AnchorTypeConfirmedEmpty, TierFull
	case ExhaustionStateHadUnitsExact:
		return AnchorTypeUserReportedResidual, TierFull
	case ExhaustionStateHadUnitsBracket:
		return AnchorTypeUserReportedResidual, TierPartial
	case ExhaustionStateUnknown:
		return AnchorTypeEstimatedModel, TierZero
	default:
		return AnchorTypeEstimatedModel, TierZero
	}
}

// AnchorAuthorityTier returns the epistemic authority tier for a given AnchorType.
// Full authority (1.0): Verified meter readings, confirmed empty, exact user residual.
// Partial authority (0.5): Bracketed user residual (or residual anchor where exact provenance is unspecified).
// Zero authority (0.0): Estimated model, unknown.
func AnchorAuthorityTier(t AnchorType) AuthorityTier {
	switch t {
	case AnchorTypeVerifiedMeterReading, AnchorTypeConfirmedEmpty, AnchorTypeHadUnitsExact:
		return TierFull
	case AnchorTypeUserReportedResidual, AnchorTypeHadUnitsBracket:
		return TierPartial
	case AnchorTypeEstimatedModel, AnchorTypeUnknown:
		return TierZero
	default:
		return TierZero
	}
}

// EvaluateCycleAuthority assigns epistemic trust weight W_cycle to a consumption cycle
// based on the proven provenance of its start and end balance anchors.
// Semantic Tiers:
// - Full authority: VERIFIED_METER_READING, CONFIRMED_EMPTY, HAD_UNITS_EXACT = 1.0 (TierFull)
// - Partial authority: HAD_UNITS_BRACKET, USER_REPORTED_RESIDUAL = 0.5 (TierPartial)
// - Zero authority: ESTIMATED_MODEL, UNKNOWN = 0.0 (TierZero)
// Returns W_cycle = W_start * W_end. If either tier is TierZero, returns 0.0.
func EvaluateCycleAuthority(startType, endType AnchorType) float64 {
	wStart := AnchorAuthorityTier(startType)
	wEnd := AnchorAuthorityTier(endType)

	if wStart == TierZero || wEnd == TierZero {
		return 0.0
	}
	return float64(wStart * wEnd)
}

// EvaluateCycleAuthorityFromTiers calculates cycle training weight directly from explicit authority tiers.
func EvaluateCycleAuthorityFromTiers(startTier, endTier AuthorityTier) float64 {
	if startTier == TierZero || endTier == TierZero {
		return 0.0
	}
	return float64(startTier * endTier)
}

// CalculateCycleAudit calculates the prediction error before applying any EMA rate updates,
// preventing data leakage and contamination during cycle evaluation.
func CalculateCycleAudit(predictedDurationDays, actualDurationDays float64) float64 {
	if math.IsNaN(predictedDurationDays) || math.IsNaN(actualDurationDays) {
		return 0.0
	}
	return math.Abs(predictedDurationDays - actualDurationDays)
}

// CalculateEMA updates the estimated daily consumption rate using Exponential Moving Average
// with bounded anomaly dampening and epistemic gating.
// Classification & Bounded Influence:
// - NORMAL (0.5 <= rho <= 2.0): W_influence = 1.0, R_eff = R_obs
// - UNUSUAL (0.25 <= rho < 0.5 or 2.0 < rho <= 3.5): W_influence = 0.5, R_eff = clamp(R_obs, 0.5*R_current, 2.0*R_current)
// - EXTREME (rho < 0.25 or rho > 3.5): W_influence = 0.0, R_eff = R_current, isAnomaly = true
// Gate check: If cycleWeight == 0.0 or W_influence == 0.0: return (currentRate, false, isAnomaly)
func CalculateEMA(currentRate, observedRate float64, cycleWeight float64, isOutage bool, cfg Config) (newRate float64, wasTrained bool, isAnomaly bool) {
	if math.IsNaN(currentRate) || math.IsInf(currentRate, 0) || currentRate <= 0.0 {
		return currentRate, false, false
	}
	if math.IsNaN(observedRate) || math.IsInf(observedRate, 0) || observedRate < 0.0 {
		return currentRate, false, true
	}
	if math.IsNaN(cycleWeight) || cycleWeight <= 0.0 {
		return currentRate, false, false
	}
	if cycleWeight > 1.0 {
		cycleWeight = 1.0
	}

	rho := observedRate / currentRate

	var wInfluence float64
	var rEff float64

	switch {
	case rho >= 0.5 && rho <= 2.0:
		// NORMAL
		wInfluence = 1.0
		rEff = observedRate
		isAnomaly = false
	case (rho >= 0.25 && rho < 0.5) || (rho > 2.0 && rho <= 3.5):
		// UNUSUAL
		wInfluence = 0.5
		rEff = clamp(observedRate, 0.5*currentRate, 2.0*currentRate)
		isAnomaly = false
	default:
		// EXTREME
		wInfluence = 0.0
		rEff = currentRate
		isAnomaly = true
	}

	if cycleWeight == 0.0 || wInfluence == 0.0 {
		return currentRate, false, isAnomaly
	}

	outageFactor := 1.0
	if isOutage {
		outageFactor = cfg.OutageAffectedInfluence
	}

	alphaEff := cfg.DefaultAlpha * cycleWeight * wInfluence * outageFactor
	newRate = (alphaEff * rEff) + ((1.0 - alphaEff) * currentRate)

	return newRate, true, isAnomaly
}

func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// EstimateDepletionHours computes the unbiased depletion horizon in hours.
// Returns math.Inf(1) safely if ratePerDay <= 0.0 and balance > 0.0.
// Returns 0.0 if balance <= 0.0.
func EstimateDepletionHours(balance float64, ratePerDay float64) float64 {
	if math.IsNaN(balance) || balance <= 0.0 {
		return 0.0
	}
	if math.IsNaN(ratePerDay) || math.IsInf(ratePerDay, 0) || ratePerDay <= 0.0 {
		return math.Inf(1)
	}
	return (balance / ratePerDay) * 24.0
}

// EvaluateAlertState evaluates depletion horizon safety with anti-flapping hysteresis
// and notification deduplication with critical state cooldown.
// Computes safety-discounted horizon: T_safe = unbiasedHoursRemaining * beta(confidence).
func EvaluateAlertState(
	unbiasedHoursRemaining float64,
	confidence ConfidenceLevel,
	currentState AlertState,
	lastNotificationSentAt time.Time,
	now time.Time,
	cfg Config,
) (newState AlertState, shouldNotify bool) {
	if math.IsNaN(unbiasedHoursRemaining) || unbiasedHoursRemaining < 0.0 {
		unbiasedHoursRemaining = 0.0
	}

	var beta float64
	switch confidence {
	case ConfidenceHigh:
		beta = cfg.BetaHigh
	case ConfidenceMedium:
		beta = cfg.BetaMedium
	case ConfidenceLow:
		beta = cfg.BetaLow
	default:
		beta = cfg.BetaLow
	}

	tSafe := unbiasedHoursRemaining * beta

	// Hysteresis Transitions
	newState = currentState
	switch currentState {
	case AlertStateNormal:
		if tSafe <= cfg.ThresholdCriticalEnter {
			newState = AlertStateCritical
		} else if tSafe <= cfg.ThresholdEarlyEnter {
			newState = AlertStateEarlyWarning
		}
	case AlertStateEarlyWarning:
		if tSafe <= cfg.ThresholdCriticalEnter {
			newState = AlertStateCritical
		} else if tSafe >= cfg.ThresholdEarlyExit {
			newState = AlertStateNormal
		}
	case AlertStateCritical:
		if tSafe >= cfg.ThresholdEarlyExit {
			newState = AlertStateNormal
		} else if tSafe >= cfg.ThresholdCriticalExit {
			newState = AlertStateEarlyWarning
		}
	default:
		if tSafe <= cfg.ThresholdCriticalEnter {
			newState = AlertStateCritical
		} else if tSafe <= cfg.ThresholdEarlyEnter {
			newState = AlertStateEarlyWarning
		} else {
			newState = AlertStateNormal
		}
	}

	// Notification Deduplication
	if newState != currentState {
		shouldNotify = true
	} else if newState == AlertStateCritical && currentState == AlertStateCritical {
		if lastNotificationSentAt.IsZero() || now.Sub(lastNotificationSentAt).Hours() >= cfg.CriticalCooldownHours {
			shouldNotify = true
		} else {
			shouldNotify = false
		}
	} else {
		shouldNotify = false
	}

	return newState, shouldNotify
}

// ComputeConfidence calculates the composite confidence score S in [0.0, 1.0] and tier (LOW, MEDIUM, HIGH)
// according to the frozen PowerWatch V1 specification:
//
// S = (0.25*M_count + 0.35*M_quality + 0.40*M_stability) * P_freshness
//
// Where:
// - M_count = min(1, N_valid / 5)
// - M_quality = average cycle-authority weight across the last N valid cycles [0.0, 1.0]
// - M_stability = max(0, 1 - RollingMAE(days) / mean_cycle_duration(days))
// - P_freshness = 1.0 while elapsed <= MaxCycleWindow
//   Beyond MaxCycleWindow: max(0.2, 1 - 0.1 * ((elapsed - MaxCycleWindow) / MaxCycleWindow))
//
// Tiers:
// - HIGH: S >= 0.75 AND N_valid >= 3
// - MEDIUM: 0.45 <= S < 0.75 (or S >= 0.75 with N_valid < 3)
// - LOW: S < 0.45
func ComputeConfidence(
	validCycleCount int,
	avgCycleWeight float64,
	rollingMAE float64,
	avgDuration float64,
	elapsedDays float64,
	maxCycleWindow float64,
) (ConfidenceLevel, float64) {
	if validCycleCount <= 0 || avgCycleWeight <= 0.0 || math.IsNaN(avgCycleWeight) {
		return ConfidenceLow, 0.0
	}

	// 1. M_count: Volume saturation (up to 5 valid cycles)
	mCount := math.Min(1.0, float64(validCycleCount)/5.0)

	// 2. M_quality: Average authority weight clamped in [0.0, 1.0]
	mQuality := clamp(avgCycleWeight, 0.0, 1.0)

	// 3. M_stability: Stability from error relative to cycle duration
	mStability := 0.0
	if avgDuration > 0.0 && !math.IsNaN(avgDuration) && !math.IsInf(avgDuration, 0) {
		mae := rollingMAE
		if mae < 0.0 || math.IsNaN(mae) {
			mae = 0.0
		}
		relError := mae / avgDuration
		mStability = math.Max(0.0, 1.0-relError)
	}

	// Base score combining volume (0.25), quality (0.35), and stability (0.40)
	baseScore := (0.25 * mCount) + (0.35 * mQuality) + (0.40 * mStability)

	// 4. P_freshness: No decay within window; gradual decay with 0.2 floor beyond window
	pFreshness := 1.0
	if maxCycleWindow > 0.0 && !math.IsNaN(maxCycleWindow) {
		if elapsedDays > maxCycleWindow {
			decay := 0.1 * ((elapsedDays - maxCycleWindow) / maxCycleWindow)
			pFreshness = math.Max(0.2, 1.0-decay)
		}
	} else if maxCycleWindow <= 0.0 && elapsedDays > 0.0 {
		pFreshness = 0.2
	}

	score := clamp(baseScore*pFreshness, 0.0, 1.0)

	var level ConfidenceLevel
	switch {
	case score >= 0.75-1e-9 && validCycleCount >= 3:
		level = ConfidenceHigh
	case score >= 0.45-1e-9:
		level = ConfidenceMedium
	default:
		level = ConfidenceLow
	}

	return level, score
}
