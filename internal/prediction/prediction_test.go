package prediction

import (
	"math"
	"testing"
	"time"
)

func TestCalculatePrior(t *testing.T) {
	tests := []struct {
		name     string
		input    ProfileInput
		expected float64
	}{
		{
			name: "Single occupant minimal appliances Band A",
			input: ProfileInput{
				Occupants:  1,
				FeederBand: "A",
			},
			// Base = 1.0 + 0 = 1.0. Appliances = 0. Band A = 1.0. Total = 1.0
			expected: 1.0,
		},
		{
			name: "4 occupants with AC and Fridge Band B",
			input: ProfileInput{
				Occupants:  4,
				FeederBand: "B",
				ACCount:    1,
				HasFridge:  true,
			},
			// Base = 1.0 + 0.4*3 = 2.2. Appliances = 4.5 + 1.5 = 6.0. Sum = 8.2. Band B = 0.85 -> 8.2 * 0.85 = 6.97
			expected: 8.2 * 0.85,
		},
		{
			name: "All appliances Band C",
			input: ProfileInput{
				Occupants:         2,
				FeederBand:        "C",
				ACCount:           2,
				HasFreezer:        true,
				HasFridge:         true,
				HasPumpingMachine: true,
				HasElectricCooker: true,
				HasWashingMachine: true,
			},
			// Base = 1.0 + 0.4 = 1.4
			// Appliances = 2*4.5 (9.0) + 2.5 + 1.5 + 1.0 + 3.5 + 0.8 = 18.3
			// Sum = 19.7. Band C = 0.65 -> 19.7 * 0.65 = 12.805
			expected: 19.7 * 0.65,
		},
		{
			name: "Band D discount",
			input: ProfileInput{
				Occupants:  1,
				FeederBand: "D",
			},
			// Base = 1.0, Band D = 0.50 -> 0.50
			expected: 0.50,
		},
		{
			name: "Band E discount",
			input: ProfileInput{
				Occupants:  1,
				FeederBand: "E",
			},
			// Base = 1.0, Band E = 0.50 -> 0.50
			expected: 0.50,
		},
		{
			name: "Band U unclassified",
			input: ProfileInput{
				Occupants:  1,
				FeederBand: "U",
			},
			// Base = 1.0, Band U = 0.75 -> 0.75
			expected: 0.75,
		},
		{
			name: "Zero occupants clamps to 1 and unknown band defaults to Band U factor",
			input: ProfileInput{
				Occupants:  0,
				FeederBand: "X",
			},
			// Clamped occupants = 1 -> Base = 1.0. Default factor = 0.75 -> 0.75
			expected: 0.75,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculatePrior(tc.input)
			if math.Abs(got-tc.expected) > 1e-9 {
				t.Errorf("CalculatePrior() = %v, expected %v", got, tc.expected)
			}
		})
	}
}

func TestDeriveBalance(t *testing.T) {
	now := time.Now()
	recordedAt := now.Add(-48 * time.Hour) // 2 days ago

	tests := []struct {
		name            string
		anchorUnits     float64
		recordedAt      time.Time
		ratePerDay      float64
		now             time.Time
		expectedBalance float64
		expectedDays    float64
	}{
		{
			name:            "Standard 2 day depletion",
			anchorUnits:     50.0,
			recordedAt:      recordedAt,
			ratePerDay:      10.0,
			now:             now,
			expectedBalance: 30.0,
			expectedDays:    2.0,
		},
		{
			name:            "Full depletion clamps to zero",
			anchorUnits:     15.0,
			recordedAt:      recordedAt,
			ratePerDay:      10.0,
			now:             now,
			expectedBalance: 0.0,
			expectedDays:    2.0,
		},
		{
			name:            "Future timestamp gives zero elapsed days",
			anchorUnits:     40.0,
			recordedAt:      now.Add(24 * time.Hour),
			ratePerDay:      10.0,
			now:             now,
			expectedBalance: 40.0,
			expectedDays:    0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bal, days := DeriveBalance(tc.anchorUnits, tc.recordedAt, tc.ratePerDay, tc.now)
			if math.Abs(bal-tc.expectedBalance) > 1e-9 {
				t.Errorf("balance = %v, expected %v", bal, tc.expectedBalance)
			}
			if math.Abs(days-tc.expectedDays) > 1e-9 {
				t.Errorf("days = %v, expected %v", days, tc.expectedDays)
			}
		})
	}
}

func TestEvaluateCycleAuthority(t *testing.T) {
	tests := []struct {
		name      string
		start     AnchorType
		end       AnchorType
		expectedW float64
	}{
		{
			name:      "Full to Full",
			start:     AnchorTypeVerifiedMeterReading,
			end:       AnchorTypeConfirmedEmpty,
			expectedW: 1.0,
		},
		{
			name:      "Full to Partial",
			start:     AnchorTypeVerifiedMeterReading,
			end:       AnchorTypeHadUnitsBracket,
			expectedW: 0.5,
		},
		{
			name:      "Partial to Partial",
			start:     AnchorTypeHadUnitsBracket,
			end:       AnchorTypeHadUnitsBracket,
			expectedW: 0.25,
		},
		{
			name:      "Full to Estimated (Epistemic Zero)",
			start:     AnchorTypeVerifiedMeterReading,
			end:       AnchorTypeEstimatedModel,
			expectedW: 0.0,
		},
		{
			name:      "Unknown to Full (Epistemic Zero)",
			start:     AnchorTypeUnknown,
			end:       AnchorTypeConfirmedEmpty,
			expectedW: 0.0,
		},
		{
			name:      "Exact to Residual",
			start:     AnchorTypeHadUnitsExact,
			end:       AnchorTypeUserReportedResidual,
			expectedW: 0.5,
		},
		{
			name:      "Unrecognized AnchorType returns Zero",
			start:     AnchorType("CUSTOM_TYPE"),
			end:       AnchorTypeConfirmedEmpty,
			expectedW: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := EvaluateCycleAuthority(tc.start, tc.end)
			if math.Abs(w-tc.expectedW) > 1e-9 {
				t.Errorf("EvaluateCycleAuthority(%s, %s) = %v, expected %v", tc.start, tc.end, w, tc.expectedW)
			}
		})
	}
}

func TestCalculateEMA(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("Normal ratio (1.0x) updates normally", func(t *testing.T) {
		currentRate := 10.0
		observedRate := 10.0
		cycleWeight := 1.0

		newRate, wasTrained, isAnomaly := CalculateEMA(currentRate, observedRate, cycleWeight, false, cfg)
		if !wasTrained {
			t.Errorf("expected wasTrained = true")
		}
		if isAnomaly {
			t.Errorf("expected isAnomaly = false")
		}
		if math.Abs(newRate-10.0) > 1e-9 {
			t.Errorf("expected newRate = 10.0, got %v", newRate)
		}
	})

	t.Run("Normal ratio (1.5x) updates with full alpha", func(t *testing.T) {
		currentRate := 10.0
		observedRate := 15.0 // rho = 1.5, NORMAL
		cycleWeight := 1.0

		// alphaEff = 0.35 * 1.0 * 1.0 * 1.0 = 0.35
		// newRate = 0.35 * 15.0 + 0.65 * 10.0 = 5.25 + 6.50 = 11.75
		newRate, wasTrained, isAnomaly := CalculateEMA(currentRate, observedRate, cycleWeight, false, cfg)
		if !wasTrained || isAnomaly {
			t.Fatalf("unexpected flags: trained=%v, anomaly=%v", wasTrained, isAnomaly)
		}
		expectedRate := 11.75
		if math.Abs(newRate-expectedRate) > 1e-9 {
			t.Errorf("got newRate %v, expected %v", newRate, expectedRate)
		}
	})

	t.Run("Unusual low ratio (0.3x) clamps to 0.5*currentRate with dampened influence", func(t *testing.T) {
		currentRate := 10.0
		observedRate := 3.0 // rho = 0.3, UNUSUAL
		cycleWeight := 1.0

		// wInfluence = 0.5, rEff = clamp(3.0, 5.0, 20.0) = 5.0
		// alphaEff = 0.35 * 1.0 * 0.5 * 1.0 = 0.175
		// newRate = 0.175 * 5.0 + 0.825 * 10.0 = 0.875 + 8.25 = 9.125
		newRate, wasTrained, isAnomaly := CalculateEMA(currentRate, observedRate, cycleWeight, false, cfg)
		if !wasTrained || isAnomaly {
			t.Fatalf("unexpected flags: trained=%v, anomaly=%v", wasTrained, isAnomaly)
		}
		expectedRate := 9.125
		if math.Abs(newRate-expectedRate) > 1e-9 {
			t.Errorf("got newRate %v, expected %v", newRate, expectedRate)
		}
	})

	t.Run("Outage dampening halves alpha", func(t *testing.T) {
		currentRate := 10.0
		observedRate := 15.0
		cycleWeight := 1.0

		// alphaEff = 0.35 * 1.0 * 1.0 * 0.50 = 0.175
		// newRate = 0.175 * 15.0 + 0.825 * 10.0 = 2.625 + 8.25 = 10.875
		newRate, wasTrained, isAnomaly := CalculateEMA(currentRate, observedRate, cycleWeight, true, cfg)
		if !wasTrained || isAnomaly {
			t.Fatalf("unexpected flags: trained=%v, anomaly=%v", wasTrained, isAnomaly)
		}
		expectedRate := 10.875
		if math.Abs(newRate-expectedRate) > 1e-9 {
			t.Errorf("got newRate %v, expected %v", newRate, expectedRate)
		}
	})
}

func TestEvaluateAlertState(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Now()

	// BetaHigh = 0.85, BetaMedium = 0.70, BetaLow = 0.55
	// ThresholdCriticalEnter = 24.0h
	// ThresholdEarlyEnter    = 48.0h
	// ThresholdEarlyExit     = 60.0h
	// ThresholdCriticalExit  = 32.0h

	t.Run("Transition Normal to Early Warning", func(t *testing.T) {
		// T_safe = 50.0 * 0.85 = 42.5h (<= 48h, > 24h)
		newState, shouldNotify := EvaluateAlertState(50.0, ConfidenceHigh, AlertStateNormal, time.Time{}, now, cfg)
		if newState != AlertStateEarlyWarning {
			t.Errorf("expected EARLY_WARNING, got %v", newState)
		}
		if !shouldNotify {
			t.Errorf("expected shouldNotify = true on state change")
		}
	})

	t.Run("Transition Normal directly to Critical", func(t *testing.T) {
		// T_safe = 20.0 * 0.85 = 17.0h (<= 24h)
		newState, shouldNotify := EvaluateAlertState(20.0, ConfidenceHigh, AlertStateNormal, time.Time{}, now, cfg)
		if newState != AlertStateCritical {
			t.Errorf("expected CRITICAL, got %v", newState)
		}
		if !shouldNotify {
			t.Errorf("expected shouldNotify = true on state change")
		}
	})

	t.Run("Early Warning transitions to Critical when dropping", func(t *testing.T) {
		// T_safe = 20.0 * 0.85 = 17.0h (<= 24h)
		newState, shouldNotify := EvaluateAlertState(20.0, ConfidenceHigh, AlertStateEarlyWarning, time.Time{}, now, cfg)
		if newState != AlertStateCritical || !shouldNotify {
			t.Errorf("expected transition to CRITICAL, got state=%v notify=%v", newState, shouldNotify)
		}
	})

	t.Run("Early Warning transitions to Normal on recovery", func(t *testing.T) {
		// T_safe = 80.0 * 0.85 = 68.0h (>= 60h ThresholdEarlyExit)
		newState, shouldNotify := EvaluateAlertState(80.0, ConfidenceHigh, AlertStateEarlyWarning, time.Time{}, now, cfg)
		if newState != AlertStateNormal || !shouldNotify {
			t.Errorf("expected transition to NORMAL, got state=%v notify=%v", newState, shouldNotify)
		}
	})

	t.Run("Critical transitions to Normal on large recharge", func(t *testing.T) {
		// T_safe = 80.0 * 0.85 = 68.0h (>= 60h ThresholdEarlyExit)
		newState, shouldNotify := EvaluateAlertState(80.0, ConfidenceHigh, AlertStateCritical, time.Time{}, now, cfg)
		if newState != AlertStateNormal || !shouldNotify {
			t.Errorf("expected transition to NORMAL, got state=%v notify=%v", newState, shouldNotify)
		}
	})

	t.Run("Critical transitions to Early Warning on moderate recharge", func(t *testing.T) {
		// T_safe = 40.0 * 0.85 = 34.0h (>= 32h ThresholdCriticalExit, < 60h)
		newState, shouldNotify := EvaluateAlertState(40.0, ConfidenceHigh, AlertStateCritical, time.Time{}, now, cfg)
		if newState != AlertStateEarlyWarning || !shouldNotify {
			t.Errorf("expected transition to EARLY_WARNING, got state=%v notify=%v", newState, shouldNotify)
		}
	})

	t.Run("Unknown current state initializes correctly", func(t *testing.T) {
		newState, _ := EvaluateAlertState(100.0, ConfidenceLow, AlertState("UNKNOWN"), time.Time{}, now, cfg)
		if newState != AlertStateNormal {
			t.Errorf("expected NORMAL for large hours, got %v", newState)
		}

		newStateEarly, _ := EvaluateAlertState(60.0, ConfidenceLow, AlertState("UNKNOWN"), time.Time{}, now, cfg)
		// 60 * 0.55 = 33.0h (<= 48h, > 24h) -> EARLY_WARNING
		if newStateEarly != AlertStateEarlyWarning {
			t.Errorf("expected EARLY_WARNING, got %v", newStateEarly)
		}

		newStateCrit, _ := EvaluateAlertState(20.0, ConfidenceLow, AlertState("UNKNOWN"), time.Time{}, now, cfg)
		// 20 * 0.55 = 11.0h (<= 24h) -> CRITICAL
		if newStateCrit != AlertStateCritical {
			t.Errorf("expected CRITICAL, got %v", newStateCrit)
		}
	})

	t.Run("Critical cooldown deduplication", func(t *testing.T) {
		// T_safe = 17.0h (still critical)
		lastNotified := now.Add(-6 * time.Hour) // only 6h elapsed, cooldown is 12h
		newState, shouldNotify := EvaluateAlertState(20.0, ConfidenceHigh, AlertStateCritical, lastNotified, now, cfg)
		if newState != AlertStateCritical {
			t.Errorf("expected state to remain CRITICAL, got %v", newState)
		}
		if shouldNotify {
			t.Errorf("expected shouldNotify = false within 12h cooldown")
		}

		// After 13 hours
		lastNotifiedOld := now.Add(-13 * time.Hour)
		_, shouldNotifyAfterCooldown := EvaluateAlertState(20.0, ConfidenceHigh, AlertStateCritical, lastNotifiedOld, now, cfg)
		if !shouldNotifyAfterCooldown {
			t.Errorf("expected shouldNotify = true after 12h cooldown elapsed")
		}
	})
}

func TestComputeConfidence(t *testing.T) {
	t.Run("Zero cycles produces LOW confidence and zero score", func(t *testing.T) {
		level, score := ComputeConfidence(0, 1.0, 0.5, 10.0, 1.0, 30.0)
		if level != ConfidenceLow || score != 0.0 {
			t.Errorf("got %v, %v; expected LOW, 0.0", level, score)
		}
	})

	t.Run("5 mature cycles with high authority and low error yields HIGH", func(t *testing.T) {
		level, score := ComputeConfidence(5, 1.0, 0.2, 10.0, 0.5, 30.0)
		if level != ConfidenceHigh {
			t.Errorf("expected HIGH confidence, got %v (score=%v)", level, score)
		}
		if score < 0.75 {
			t.Errorf("expected score >= 0.75, got %v", score)
		}
	})

	t.Run("Moderate cycles and partial authority yields MEDIUM", func(t *testing.T) {
		// countFactor = 2/5 = 0.40, authFactor = 0.5, relError = 1.0/10 = 0.1 -> acc = 0.9
		// baseScore = 0.35*0.4 + 0.35*0.5 + 0.30*0.9 = 0.14 + 0.175 + 0.27 = 0.585
		// freshness = 1.0 - 5/30 = 25/30 = 0.833 -> score = 0.585 * 0.833 = 0.487 -> MEDIUM
		level, score := ComputeConfidence(2, 0.5, 1.0, 10.0, 5.0, 30.0)
		if level != ConfidenceMedium {
			t.Errorf("expected MEDIUM confidence, got %v (score=%v)", level, score)
		}
	})

	t.Run("Stale data beyond maxCycleWindow degrades confidence", func(t *testing.T) {
		level, score := ComputeConfidence(5, 1.0, 0.2, 10.0, 35.0, 30.0)
		if level != ConfidenceLow || score != 0.0 {
			t.Errorf("expected LOW confidence (score 0.0) for expired cycle window, got %v, %v", level, score)
		}
	})

	t.Run("Zero elapsed days gives full freshness factor", func(t *testing.T) {
		level, score := ComputeConfidence(5, 1.0, 0.0, 10.0, 0.0, 30.0)
		if level != ConfidenceHigh || score != 1.0 {
			t.Errorf("expected HIGH confidence and 1.0 score, got %v, %v", level, score)
		}
	})
}

// ---------------------------------------------------------------------
// MANDATORY BEHAVIORAL INVARIANT TESTS
// ---------------------------------------------------------------------

// 1. Non-Negativity Invariant: DeriveBalance must never return negative units,
// even when elapsed time is years into the future.
func TestInvariant_NonNegativity(t *testing.T) {
	now := time.Now()
	longPast := now.Add(-365 * 24 * 5 * time.Hour) // 5 years ago

	balance, elapsedDays := DeriveBalance(100.0, longPast, 25.0, now)
	if balance < 0.0 {
		t.Fatalf("Non-Negativity Invariant violated: balance %v < 0", balance)
	}
	if elapsedDays < 0.0 {
		t.Fatalf("Non-Negativity Invariant violated: elapsedDays %v < 0", elapsedDays)
	}
	if balance != 0.0 {
		t.Fatalf("Expected balance to be 0.0 after 5 years of depletion, got %v", balance)
	}
}

// 2. Epistemic Isolation Invariant: If startType or endType is ESTIMATED_MODEL or UNKNOWN,
// cycleWeight must be exactly 0.0, and CalculateEMA must return wasTrained = false with rate unchanged.
func TestInvariant_EpistemicIsolation(t *testing.T) {
	cfg := DefaultConfig()
	initialRate := 12.50
	observedRate := 14.00

	isolatedTypes := []AnchorType{AnchorTypeEstimatedModel, AnchorTypeUnknown}
	allOtherTypes := []AnchorType{
		AnchorTypeVerifiedMeterReading,
		AnchorTypeConfirmedEmpty,
		AnchorTypeHadUnitsExact,
		AnchorTypeHadUnitsBracket,
		AnchorTypeUserReportedResidual,
	}

	for _, iso := range isolatedTypes {
		for _, other := range allOtherTypes {
			// Test startType as isolated
			w1 := EvaluateCycleAuthority(iso, other)
			if w1 != 0.0 {
				t.Fatalf("Epistemic Isolation Invariant violated: weight %v != 0 for (%s, %s)", w1, iso, other)
			}
			newRate1, trained1, _ := CalculateEMA(initialRate, observedRate, w1, false, cfg)
			if trained1 {
				t.Fatalf("Epistemic Isolation Invariant violated: wasTrained should be false for weight 0.0")
			}
			if newRate1 != initialRate {
				t.Fatalf("Epistemic Isolation Invariant violated: rate changed from %v to %v", initialRate, newRate1)
			}

			// Test endType as isolated
			w2 := EvaluateCycleAuthority(other, iso)
			if w2 != 0.0 {
				t.Fatalf("Epistemic Isolation Invariant violated: weight %v != 0 for (%s, %s)", w2, other, iso)
			}
			newRate2, trained2, _ := CalculateEMA(initialRate, observedRate, w2, false, cfg)
			if trained2 {
				t.Fatalf("Epistemic Isolation Invariant violated: wasTrained should be false for weight 0.0")
			}
			if newRate2 != initialRate {
				t.Fatalf("Epistemic Isolation Invariant violated: rate changed from %v to %v", initialRate, newRate2)
			}
		}
	}
}

// 3. Safety Non-Pollution Invariant: Verify that T_safe evaluated in the Decision Layer
// does not alter the unbiased T_depletion point estimate.
func TestInvariant_SafetyNonPollution(t *testing.T) {
	cfg := DefaultConfig()
	balance := 20.0
	rate := 10.0

	unbiasedHours := EstimateDepletionHours(balance, rate)
	expectedUnbiased := (20.0 / 10.0) * 24.0 // 48.0 hours

	if math.Abs(unbiasedHours-expectedUnbiased) > 1e-9 {
		t.Fatalf("Expected unbiased hours %v, got %v", expectedUnbiased, unbiasedHours)
	}

	// Decision layer evaluates alert state with safety discounting
	now := time.Now()
	alertState, _ := EvaluateAlertState(unbiasedHours, ConfidenceMedium, AlertStateNormal, time.Time{}, now, cfg)

	// Invariant: unbiased point estimate variable must remain completely unaffected
	if math.Abs(unbiasedHours-expectedUnbiased) > 1e-9 {
		t.Fatalf("Safety Non-Pollution Invariant violated: unbiased hours mutated to %v", unbiasedHours)
	}

	// Medium confidence beta is 0.70 -> T_safe = 48.0 * 0.70 = 33.6h <= 48h -> EARLY_WARNING
	if alertState != AlertStateEarlyWarning {
		t.Fatalf("Expected alert state EARLY_WARNING from T_safe=33.6h, got %v", alertState)
	}
}

// 4. Bounded Anomaly Invariant: A 3.0x surge updates the rate with dampened influence (0.5),
// whereas a 4.0x surge flags an anomaly and leaves the rate unchanged.
func TestInvariant_BoundedAnomaly(t *testing.T) {
	cfg := DefaultConfig()
	currentRate := 10.0
	cycleWeight := 1.0

	// 1. 3.0x surge: UNUSUAL (2.0 < rho <= 3.5), influence = 0.5, clamp to 2.0*R_current = 20.0
	surge3x := 30.0
	newRate3x, trained3x, anomaly3x := CalculateEMA(currentRate, surge3x, cycleWeight, false, cfg)

	if !trained3x {
		t.Fatalf("Bounded Anomaly Invariant violated: 3.0x surge should be trained")
	}
	if anomaly3x {
		t.Fatalf("Bounded Anomaly Invariant violated: 3.0x surge should not be flagged as extreme anomaly")
	}
	// alphaEff = 0.35 * 1.0 * 0.5 * 1.0 = 0.175
	// rEff = clamp(30.0, 5.0, 20.0) = 20.0
	// expected = 0.175 * 20.0 + 0.825 * 10.0 = 3.5 + 8.25 = 11.75
	expected3xRate := 11.75
	if math.Abs(newRate3x-expected3xRate) > 1e-9 {
		t.Fatalf("Bounded Anomaly Invariant violated: expected 3.0x rate %v, got %v", expected3xRate, newRate3x)
	}

	// 2. 4.0x surge: EXTREME (rho > 3.5), influence = 0.0, isAnomaly = true, rate untouched
	surge4x := 40.0
	newRate4x, trained4x, anomaly4x := CalculateEMA(currentRate, surge4x, cycleWeight, false, cfg)

	if trained4x {
		t.Fatalf("Bounded Anomaly Invariant violated: 4.0x surge should not train")
	}
	if !anomaly4x {
		t.Fatalf("Bounded Anomaly Invariant violated: 4.0x surge must be flagged as anomaly")
	}
	if newRate4x != currentRate {
		t.Fatalf("Bounded Anomaly Invariant violated: 4.0x surge must leave rate unchanged (%v != %v)", newRate4x, currentRate)
	}
}

// 5. Anti-Flapping Hysteresis Invariant: Simulate a signal oscillating between 46h and 52h;
// verify the state does not flap between EARLY_WARNING and NORMAL.
func TestInvariant_AntiFlappingHysteresis(t *testing.T) {
	// Configure high beta = 1.0 so T_safe matches the exact signal hours for clear boundary evaluation
	cfg := DefaultConfig()
	cfg.BetaHigh = 1.0
	cfg.ThresholdEarlyEnter = 48.0
	cfg.ThresholdEarlyExit = 60.0

	now := time.Now()
	state := AlertStateNormal

	// Step 1: Signal drops to 46h (<= 48h Enter threshold) -> Enters EARLY_WARNING
	signal := 46.0
	state, shouldNotify := EvaluateAlertState(signal, ConfidenceHigh, state, time.Time{}, now, cfg)
	if state != AlertStateEarlyWarning || !shouldNotify {
		t.Fatalf("Failed to transition from NORMAL to EARLY_WARNING at 46h: got state=%s notify=%v", state, shouldNotify)
	}

	// Oscillate 10 times between 52h and 46h
	for i := 0; i < 10; i++ {
		// Signal rises to 52h: In EARLY_WARNING, exit requires >= 60h, so it MUST NOT return to NORMAL
		signal = 52.0
		state, shouldNotify = EvaluateAlertState(signal, ConfidenceHigh, state, time.Time{}, now, cfg)
		if state != AlertStateEarlyWarning {
			t.Fatalf("Anti-Flapping Invariant violated at cycle %d: flapped back to %s at 52h", i, state)
		}
		if shouldNotify {
			t.Fatalf("Anti-Flapping Invariant violated at cycle %d: redundant notification sent while in %s", i, state)
		}

		// Signal dips back to 46h: Stays EARLY_WARNING
		signal = 46.0
		state, shouldNotify = EvaluateAlertState(signal, ConfidenceHigh, state, time.Time{}, now, cfg)
		if state != AlertStateEarlyWarning {
			t.Fatalf("Anti-Flapping Invariant violated at cycle %d: unexpected state %s at 46h", i, state)
		}
		if shouldNotify {
			t.Fatalf("Anti-Flapping Invariant violated at cycle %d: redundant notification sent while in %s", i, state)
		}
	}
}

// 6. Zero-Division Invariant: Rate = 0.0 or elapsed time = 0.0 must return safe boundary results
// (Inf or 0) without panicking.
func TestInvariant_ZeroDivision(t *testing.T) {
	// A. EstimateDepletionHours with Rate = 0.0
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Zero-Division Invariant violated: panicked with %v", r)
		}
	}()

	hoursZeroRate := EstimateDepletionHours(50.0, 0.0)
	if !math.IsInf(hoursZeroRate, 1) {
		t.Fatalf("Expected math.Inf(1) for rate=0.0, got %v", hoursZeroRate)
	}

	hoursZeroBalance := EstimateDepletionHours(0.0, 10.0)
	if hoursZeroBalance != 0.0 {
		t.Fatalf("Expected 0.0 for balance=0.0, got %v", hoursZeroBalance)
	}

	// B. DeriveBalance with elapsed time = 0.0
	now := time.Now()
	balanceZeroElapsed, elapsedDays := DeriveBalance(100.0, now, 10.0, now)
	if elapsedDays != 0.0 || balanceZeroElapsed != 100.0 {
		t.Fatalf("Expected (100.0, 0.0), got (%v, %v)", balanceZeroElapsed, elapsedDays)
	}

	// C. CalculateEMA with Rate = 0.0
	cfg := DefaultConfig()
	newRate, trained, anomaly := CalculateEMA(0.0, 10.0, 1.0, false, cfg)
	if trained || anomaly || newRate != 0.0 {
		t.Fatalf("Safe boundary expected for currentRate=0.0 in EMA, got newRate=%v, trained=%v, anomaly=%v", newRate, trained, anomaly)
	}

	// D. ComputeConfidence with 0 avgDuration or 0 maxCycleWindow
	level, score := ComputeConfidence(3, 1.0, 1.0, 0.0, 0.0, 0.0)
	if level == "" || math.IsNaN(score) {
		t.Fatalf("ComputeConfidence returned invalid values on zero boundary: %v, %v", level, score)
	}
}

// 7. Idempotent Prior Invariant: Running identical household inputs produces bit-exact identical baseline priors.
func TestInvariant_IdempotentPrior(t *testing.T) {
	household := ProfileInput{
		Occupants:         5,
		FeederBand:        "B",
		ACCount:           3,
		HasFreezer:        true,
		HasFridge:         true,
		HasPumpingMachine: true,
		HasElectricCooker: false,
		HasWashingMachine: true,
	}

	firstRun := CalculatePrior(household)
	firstBits := math.Float64bits(firstRun)

	for i := 0; i < 1000; i++ {
		subsequentRun := CalculatePrior(household)
		subsequentBits := math.Float64bits(subsequentRun)
		if firstBits != subsequentBits {
			t.Fatalf("Idempotent Prior Invariant violated: iteration %d produced %v (bits %x) vs initial %v (bits %x)",
				i, subsequentRun, subsequentBits, firstRun, firstBits)
		}
	}
}
