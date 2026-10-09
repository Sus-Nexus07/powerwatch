# PowerWatch Product Experience Foundation: Localization & Little Spark Mascot

> **Brand Status**: *Provisional Working Title: PowerWatch*. Candidate consumer-facing brands (*UnitDey*, *DeyAhead*) remain under evaluation and are not yet adopted for repository or codebase naming.

---

## 1. Vision & Emotional Architecture

Prepaid electricity in Nigeria is an anxious, opaque experience characterized by sudden meter lockouts, unexpected power cuts, and the constant stress of estimating depletion by guesswork. 

PowerWatch's product experience exists to transform this anxiety into calm predictability. The design system combines mathematical rigor with a warm, trustworthy companion that feels like an informed friend who always looks out for you.

---

## 2. Language Preferences & Localization Strategy

### 2.1 Supported Languages
- **V1 Launch Tier**:
  - `en` — English (Nigerian Standard)
  - `pcm` — Nigerian Pidgin
- **Post-V1 Extension Tier** (to be released following human translation and dialect review):
  - `yo` — Yoruba
  - `ig` — Igbo
  - `ha` — Hausa

### 2.2 First-Launch Language Selection Flow
1. **Initial Screen**: Before onboarding, account creation, or setup steps, the user is presented with a simple, high-clarity language selection screen:
   - English (*"Choose your preferred language"*)
   - Nigerian Pidgin (*"Choose the language wey you prefer"*)
2. **Reassurance Prompt**: Explicit caption stating:
   *"You fit change this language anytime inside Settings." / "You can change your language anytime in Settings."*
3. **No Profiling Invariant**: The system must **never** infer or guess a user's language based on phone number prefix, surname, geo-location, IP address, or estimated DisCo territory. Language selection must always be explicit.

### 2.3 Persistence & Settings Management
- **Persistence**: The chosen language code (`en`, `pcm`) is stored in local client preferences and synced to the user profile metadata table (`preferred_language VARCHAR(10) DEFAULT 'en'`).
- **Settings Control**: A prominent "Language / Tok" row in Settings enables instantaneous switching without app restart or data reload.

### 2.4 Localization Architecture & Translation Fallback
- **Key-Based Message Catalogue**: All UI strings, push notifications, and companion dialogues use structured localization keys (e.g., `spark.warning.early`, `meter.balance.estimated`).
- **Fallback Hierarchy**:
  ```text
  Requested Language (e.g., pcm / yo) ──► English (en) ──► Raw Fallback Token
  ```
  If a localized string is missing or unverified, the system seamlessly defaults to English. No raw template codes or missing string crashes will ever reach the user.

---

## 3. The Little Spark Companion (Concept C)

### 3.1 Visual Identity & Character Design
- **Form**: A compact, radiant spark of warm amber-gold light with subtle electric-blue energetic halos.
- **Features**: Large, clear, expressive eyes and a gentle, reassuring smile.
- **Aesthetic**: Clean, polished, modern, and stylized. Designed with balanced, friendly proportions that appeal to adults managing a household, avoiding juvenile or clownish cartoon tropes.
- **Physical States**:
  - *Normal/Calm*: Steady gentle glow, upright relaxed posture, smiling gaze.
  - *Thinking/Forecasting*: Soft orbiting ember, eyes looking up thoughtfully.
  - *Attentive/Warning*: Intense focused glow, alert expression, pointing clearly toward actionable information.
  - *Celebrating*: Upbeat bounce with subtle spark particles on successful recharge calibration.

### 3.2 Personality & Tone of Voice
- **Core Persona**: A calm, hyper-competent energy assistant blended with the warmth, wit, and streetwise honesty of a reliable Nigerian neighbor.
- **Tone Guidelines**:
  - *Helpful, Never Alarmist*: Delivers warnings early so the user has time to act.
  - *Humble About Predictions*: Never claims omniscience; always frames forecasts as estimates unless verified by the physical meter.
  - *No Jokes in Emergencies*: Playful humor is welcomed during onboarding and routine updates, but strictly prohibited during `CRITICAL` low-unit alerts.

### 3.3 Event-Driven Message Catalogue

| Event Context | English (`en`) | Nigerian Pidgin (`pcm`) | Tone / Safety Rule |
| :--- | :--- | :--- | :--- |
| **Welcome** | "Hello! I'm Little Spark. Let's make sure your light never goes out unawares." | "Welcome! Na me be Little Spark. We go make sure say light no shock you cut unawares." | Warm, welcoming |
| **Initial Setup** | "Tell me about your home appliances so I can learn your light usage." | "Gimme small gist about your appliances make I sabi how your light dey go." | Conversational setup |
| **Normal Status** | "Your balance looks healthy. You have roughly 5 days of light remaining." | "Light dey full ground! You get roughly 5 days before units go down." | Reassuring, calm |
| **Early Warning** (48h safe horizon) | "Heads up: your units may finish in about 2 days. This is an estimate—check your meter before making plans." | "Heads up o: your units fit finish in about 2 days. Na estimate—abeg check your meter make you plan well." | Clear estimate disclaimer |
| **Critical Warning** (24h safe horizon) | "Urgent: your electricity units are very low. Please recharge today to avoid sudden disconnection." | "Urgent notice: your units don critical well-well. Abeg recharge today make light no cut." | **Zero humor. Calm, urgent clarity.** |
| **Uncertain Estimate** (Low confidence) | "I'm still learning your usage pattern. Please check your meter directly to stay safe." | "I still dey study how your appliances dey chop units. Abeg check your meter directly make you dey sure." | Epistemic humility |
| **Recharge Recorded** | "Recharge noted! I've updated your balance and calibrated your consumption model." | "Recharge don enter! I don update your balance and adjust your light forecast." | Confident, appreciative |
| **Error / Connection Issue** | "I couldn't reach the server just now, but your cached forecast is still available." | "Network dey drag small, but your saved forecast still dey here." | Safe, non-panicking |

---

## 4. Trust, Accuracy & Prediction Safety Principles

### 4.1 Strict Demarcation of Estimates vs. Physical Ground Truth
- The application and Little Spark must **never** present an algorithmic prediction as an undisputed fact.
- Every depletion prediction must state or link to an explanation:
  > *"Estimated depletion based on historical usage. Physical meter reading is the ultimate ground truth."*
- When presenting remaining days/hours, confidence bands or explicit ranges must be visible on the dashboard.

### 4.2 Accessibility & Safety Standards
1. **Non-Reliance on Color Alone**: Warning states must never rely solely on amber or red glowing states of the mascot. High-contrast typography, clear icons, and explicit text must accompany every warning.
2. **Screen Reader Compatibility**: Little Spark messages must be tagged with appropriate ARIA live regions and accessibility labels (`aria-live="polite"` for routine updates, `aria-live="assertive"` for critical alerts).
3. **No Sensory Distractions**: No strobe or rapid flashing animations are permitted in the mascot glow effects.
