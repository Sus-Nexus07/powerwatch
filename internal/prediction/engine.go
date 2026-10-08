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
	elapsedHours := now.Sub(recordedAt).Hours()
	if elapsedHours < 0.0 {
		elapsedHours = 0.0
	}
	elapsedDays := elapsedHours / 24.0

	depletion := currentRatePerDay * elapsedDays
	estimatedBalance := anchorUnits - depletion
	if estimatedBalance < 0.0 {
		estimatedBalance = 0.0
	}

	return estimatedBalance, elapsedDays
}

// EvaluateCycleAuthority assigns epistemic trust weight W_cycle to a consumption cycle
// based on the proven provenance of its start and end balance anchors.
// Semantic Tiers:
// - VERIFIED_METER_READING, CONFIRMED_EMPTY, HAD_UNITS_EXACT = 1.0 (TIER_FULL)
// - HAD_UNITS_BRACKET, USER_REPORTED_RESIDUAL = 0.5 (TIER_PARTIAL)
// - ESTIMATED_MODEL, UNKNOWN = 0.0 (TIER_ZERO)
// Returns W_cycle = W_start * W_end. If either tier is TIER_ZERO, returns 0.0.
func EvaluateCycleAuthority(startType, endType AnchorType) float64 {
	wStart := getAnchorTierWeight(startType)
	wEnd := getAnchorTierWeight(endType)

	if wStart == 0.0 || wEnd == 0.0 {
		return 0.0
	}
	return wStart * wEnd
}

func getAnchorTierWeight(t AnchorType) float64 {
	switch t {
	case AnchorTypeVerifiedMeterReading, AnchorTypeConfirmedEmpty, AnchorTypeHadUnitsExact:
		return 1.0
	case AnchorTypeHadUnitsBracket, AnchorTypeUserReportedResidual:
		return 0.5
	case AnchorTypeEstimatedModel, AnchorTypeUnknown:
		return 0.0
	default:
		return 0.0
	}
}

// CalculateEMA updates the estimated daily consumption rate using Exponential Moving Average
// with bounded anomaly dampening and epistemic gating.
// Classification & Bounded Influence:
// - NORMAL (0.5 <= rho <= 2.0): W_influence = 1.0, R_eff = R_obs
// - UNUSUAL (0.25 <= rho < 0.5 or 2.0 < rho <= 3.5): W_influence = 0.5, R_eff = clamp(R_obs, 0.5*R_current, 2.0*R_current)
// - EXTREME (rho < 0.25 or rho > 3.5): W_influence = 0.0, R_eff = R_current, isAnomaly = true
// Gate check: If cycleWeight == 0.0 or W_influence == 0.0: return (currentRate, false, isAnomaly)
func CalculateEMA(currentRate, observedRate float64, cycleWeight float64, isOutage bool, cfg Config) (newRate float64, wasTrained bool, isAnomaly bool) {
	if currentRate <= 0.0 {
		return currentRate, false, false
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
	if balance <= 0.0 {
		return 0.0
	}
	if ratePerDay <= 0.0 {
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
// across cycle quantity, authority provenance, error performance, and freshness window.
func ComputeConfidence(
	validCycleCount int,
	avgCycleWeight float64,
	rollingMAE float64,
	avgDuration float64,
	elapsedDays float64,
	maxCycleWindow float64,
) (ConfidenceLevel, float64) {
	if validCycleCount <= 0 || avgCycleWeight <= 0.0 {
		return ConfidenceLow, 0.0
	}

	// 1. Cycle count saturation (5 valid cycles reaches saturation)
	countFactor := math.Min(1.0, float64(validCycleCount)/5.0)

	// 2. Authority provenance factor [0.0, 1.0]
	authFactor := math.Max(0.0, math.Min(1.0, avgCycleWeight))

	// 3. Accuracy factor based on relative error (MAE / avgDuration)
	accuracyFactor := 0.5
	if avgDuration > 0.0 {
		if rollingMAE <= 0.0 {
			accuracyFactor = 1.0
		} else {
			relError := rollingMAE / avgDuration
			accuracyFactor = math.Max(0.0, 1.0-relError)
		}
	}

	// Base score from volume, authority, and accuracy
	baseScore := (0.35 * countFactor) + (0.35 * authFactor) + (0.30 * accuracyFactor)

	// 4. Freshness attenuation
	freshnessFactor := 1.0
	if maxCycleWindow > 0.0 && elapsedDays > 0.0 {
		if elapsedDays >= maxCycleWindow {
			freshnessFactor = 0.0
		} else {
			freshnessFactor = 1.0 - (elapsedDays / maxCycleWindow)
		}
	}

	score := math.Max(0.0, math.Min(1.0, baseScore*freshnessFactor))

	var level ConfidenceLevel
	switch {
	case score >= 0.75:
		level = ConfidenceHigh
	case score >= 0.40:
		level = ConfidenceMedium
	default:
		level = ConfidenceLow
	}

	return level, score
}
