# ADR 0001: Community combat formula confirmed against observed attacks

- **Status:** Accepted
- **Date:** 2026-09-24
- **Issue:** #2 (Combat formula spike)
- **Build:** `Android 2.17.3.16375`

## Context

The technical design builds combat on the community-documented Polytopia combat formula, with its constants in `rules/v1.yaml`. Combat feeds the threat check, the evaluator and search, so a wrong formula would invalidate most of the engine. Issue #2 checks the formula against real attacks on the pinned build before any engine code depends on it.

## Method

One Imperius game on the pinned build was screen recorded (Samsung SM-S908U, Android 16). Frames immediately before and after five of the player's own attacks were extracted, with exactly one attack between each pair. Unit types, HP, veteran status and defense-bonus shields were read from the frames. Predictions were computed by `tools/combat-spike` and compared to the observed HP after each attack.

## Decision

Use the community formula as the engine's combat model, with these rules:

```
attackForce  = attack  × (attackerHP / attackerMaxHP)
defenseForce = defense × (defenderHP / defenderMaxHP) × defenseBonus
total        = attackForce + defenseForce

damage       = roundHalfUp(attackForce  / total × attack  × 4.5)
retaliation  = roundHalfUp(defenseForce / total × defense × 4.5)
```

- **Rounding is half up.** A raw value of exactly 4.5 becomes 5 (combat_01, combat_05).
- **Retaliation uses the defender's HP from before the attack.** Using post-attack HP would give 3 instead of the observed 5 in combat_01, and 2 instead of 5 in combat_04.
- **Retaliation can kill the attacker.** The attacker is removed, and the defender is credited with the kill (combat_02, combat_03).
- **City defense bonus is 1.5** for a city without walls (combat_03, combat_04). A 4× wall bonus would have given 2 damage in combat_04, not the observed 4.

## Evidence

Output of `go run ./tools/combat-spike`. All units are Imperius or enemy warriors (attack 2, defense 2, max HP 10). "A/D" is attacker HP / defender HP after the attack; 0 means the unit died.

| Case | Situation | Raw damage | Raw retaliation | Predicted A/D | Observed A/D | Result |
| --- | --- | --- | --- | --- | --- | --- |
| combat_01 | Full HP vs full HP, no bonus | 4.50 | 4.50 | 5 / 5 | 5 / 5 | Match |
| combat_02 | Damaged attacker (5) vs full HP, no bonus | 3.00 | 6.00 | 0 / 7 | 0 / 7 | Match |
| combat_03 | Damaged attacker (5) vs full HP, city | 2.25 | 6.75 | 0 / 8 | 0 / 8 | Match |
| combat_04 | Full HP vs damaged defender (8), city | 4.09 | 4.91 | 5 / 4 | 5 / 4 | Match |
| combat_05 | Full HP vs full HP, no bonus (repeat) | 4.50 | 4.50 | 5 / 5 | 5 / 5 | Match |

**5 of 5 observed attacks match. No mismatches.**

Screenshots: `testdata/scenarios/shots/combat_0N_before.jpg` and `combat_0N_after.jpg`. The combat_02 defender's HP (7) was read during the retaliation animation; a later frame should confirm it.

### Also confirmed in passing

- **Veterancy needs 3 kills.** The unit panel shows "Warrior (1/3 kills)."
- **Capture requires starting the turn in the city.** Entering a village shows "Will be ready to capture next turn."

## Still unverified

These keep their `# verify` comments and need observed scenarios (issue #17):

- Wall bonus (4× in the rules file).
- Forest and mountain defense bonuses and the techs they require. combat_05's defender may have been on forest with no shield, which would suggest the bonus depends on the defender's techs, but the frame doesn't confirm the terrain.
- Ranged attacks, including no retaliation against an attacker outside the defender's range.
- An attack that kills the defender (expected: no retaliation).
- Veteran units' HP and combat.
- Stats of any unit other than the warrior.

## Consequences

- **Rules file.** Remove the `# verify` comments from `damage_scale_x10: 45`, `defense_bonus_x10: 15` and `kills_required: 3`. Keep it on `wall_bonus_x10: 40`.
- **Rounding precision in the engine.** HP is stored ×10, but the game rounds damage to whole HP. `ResolveCombat` must round at whole-HP precision and then scale to ×10, so damage is always a multiple of 10 in stored units. Rounding at ×10 precision would turn a raw 4.5 into 45 (4.5 HP) instead of 50, and silently diverge from the game.
- **Retaliation order.** `ResolveCombat` computes both damage values from pre-attack HP. It applies retaliation only if the defender survives; that part is unverified and stays as specified in the design doc.
- **Kill credit.** When retaliation kills the attacker, the defender's `Kills` increments. This needs its own scenario.
- **Scenario format.** The five cases are drafted in a relative-placement format (no tile coordinates), which differs from the design doc's scenario format. They are kept in `testdata/scenarios/pending/` until a separate issue decides whether combat scenarios may omit coordinates.
- **Spike script.** `tools/combat-spike` is throwaway. Delete it once the scenario runner (#16) covers these five cases.
- **Unblocks** #8 (populate `rules/v1.yaml`) and the combat work in #9.
