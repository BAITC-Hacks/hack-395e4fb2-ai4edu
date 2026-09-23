#!/usr/bin/env python3
"""Independent re-implementation of docs/scoring.md in exact fractions.

Shares no code with the Go engine: data and formula are copied from the
specification. Recomputes the reference scenarios and fails if a result
differs from the expected value. `--optimum` also brute-forces every valid
scenario (about 30 s) and checks the best one.

Usage: python3 scripts/verify_scores.py [--optimum]
"""
import sys
from fractions import Fraction as F
from itertools import combinations, product

INDICATORS = ["T1", "T2", "E1", "E2", "S1", "S2", "B1", "B2", "C1", "C2"]
WEIGHTS = dict(zip(INDICATORS, [F(w, 100) for w in (10, 10, 9, 11, 11, 11, 9, 9, 10, 10)]))
HORIZON = 8
BUDGET = 100
CRITICAL = 40

# district: (population share, baseline indicators in INDICATORS order)
DISTRICTS = {
    "esil": (F(27, 100), [45, 62, 68, 72, 48, 55, 78, 60, 75, 70]),
    "almaty": (F(24, 100), [40, 75, 50, 55, 60, 65, 62, 52, 50, 60]),
    "saryarka": (F(20, 100), [50, 70, 42, 40, 62, 68, 58, 55, 45, 55]),
    "baikonur": (F(13, 100), [52, 68, 55, 50, 58, 60, 52, 58, 55, 58]),
    "nura": (F(16, 100), [55, 40, 45, 65, 38, 35, 55, 50, 60, 50]),
}

# measure: (category, scope, cost, lag, full effects)
MEASURES = {
    "M1": ("Transport", "district", 18, 2, {"T1": 6, "T2": 9}),
    "M2": ("Transport", "city", 22, 2, {"T1": 4, "B2": 3}),
    "M3": ("Transport", "district", 30, 4, {"T1": 16, "T2": 20, "E2": 4}),
    "M4": ("Ecology", "district", 15, 2, {"E1": 12, "E2": 3, "B1": 2}),
    "M5": ("Ecology", "district", 25, 3, {"E2": 14, "C1": 4}),
    "M6": ("Ecology", "city", 20, 4, {"E1": 5, "E2": 3}),
    "M7": ("Social", "district", 24, 3, {"S1": 16}),
    "M8": ("Social", "district", 20, 3, {"S2": 14}),
    "M9": ("Social", "district", 10, 1, {"S1": 3, "S2": 3, "B1": 3}),
    "M10": ("Safety", "district", 12, 1, {"B1": 12, "B2": 2}),
    "M11": ("Safety", "district", 10, 1, {"B2": 12, "T1": -2}),
    "M12": ("Services", "city", 14, 1, {"C2": 5}),
    "M13": ("Services", "district", 28, 4, {"C1": 18, "E2": 2}),
    "M14": ("Services", "city", 16, 1, {"C1": 5, "C2": 2}),
}

# (pair, measure whose district receives the bonus, bonus without lag)
SYNERGIES = [(("M1", "M2"), "M1", {"T1": 2}), (("M10", "M12"), "M10", {"B1": 2}), (("M5", "M6"), "M5", {"E2": 2})]
GLOBAL_CONFLICTS = [("M1", "M3")]
SAME_DISTRICT_CONFLICTS = [("M4", "M7"), ("M5", "M13")]


def is_valid(plan):
    """plan: {measure_id: district_id or None}; dict keys already forbid duplicates."""
    if len(plan) != 5 or sum(MEASURES[m][2] for m in plan) > BUDGET:
        return False
    categories = [MEASURES[m][0] for m in plan]
    if any(categories.count(c) > 2 for c in categories):
        return False
    for m, district in plan.items():
        if (MEASURES[m][1] == "city") != (district is None):
            return False
    if any(a in plan and b in plan for a, b in GLOBAL_CONFLICTS):
        return False
    return not any(a in plan and b in plan and plan[a] == plan[b] for a, b in SAME_DISTRICT_CONFLICTS)


def score(plan, num=F):
    """Exact by default; num=float only speeds up the optimum sweep."""
    values = {d: dict(zip(INDICATORS, map(num, base))) for d, (_, base) in DISTRICTS.items()}
    for m, district in plan.items():
        _, scope, _, lag, effects = MEASURES[m]
        for d in DISTRICTS if scope == "city" else [district]:
            for k, v in effects.items():
                values[d][k] += v * num(HORIZON - lag) / num(HORIZON)
    for pair, target, bonus in SYNERGIES:
        if all(m in plan for m in pair):
            for k, v in bonus.items():
                values[plan[target]][k] += num(v)
    for d in values:
        for k in INDICATORS:
            values[d][k] = min(num(100), max(num(0), values[d][k]))
    district_scores = {d: sum(num(WEIGHTS[k]) * values[d][k] for k in INDICATORS) for d in values}
    average = sum(num(DISTRICTS[d][0]) * s for d, s in district_scores.items())
    critical = sum(1 for d in values for k in INDICATORS if values[d][k] < CRITICAL)
    return num(F(7, 10)) * average + num(F(3, 10)) * min(district_scores.values()) - critical, critical


REFERENCE = [
    ("base", {}, "52.55768", 2),
    ("golden", {"M7": "nura", "M8": "nura", "M10": "nura", "M12": None, "M5": "saryarka"}, "56.54307", 0),
    ("optimum", {"M2": None, "M14": None, "M3": "nura", "M8": "nura", "M9": "nura"}, "57.236735", 0),
    ("skew", {"M1": "esil", "M10": "esil", "M13": "esil", "M4": "esil", "M8": "esil"}, "53.5768625", 2),
    ("trap", {"M11": "almaty", "M12": None, "M5": "saryarka", "M7": "nura", "M8": "nura"}, "55.14404", 1),
]


def all_valid_plans():
    for combo in combinations(MEASURES, 5):
        if sum(MEASURES[m][2] for m in combo) > BUDGET:
            continue
        local = [m for m in combo if MEASURES[m][1] == "district"]
        for districts in product(DISTRICTS, repeat=len(local)):
            plan = dict.fromkeys(combo)
            plan.update(zip(local, districts))
            if is_valid(plan):
                yield plan


def main():
    failed = False
    for name, plan, expected, expected_critical in REFERENCE:
        got, critical = score(plan)
        ok = got == F(expected) and critical == expected_critical and (not plan or is_valid(plan))
        failed |= not ok
        print(f"{'PASS' if ok else 'FAIL'} {name:8} score={float(got):.7f} critical={critical} expected={expected}")
    if "--optimum" in sys.argv:
        count, best = 0, None
        for plan in all_valid_plans():
            count += 1
            s, _ = score(plan, float)
            if best is None or s > best[0]:
                best = (s, plan)
        exact, _ = score(best[1])  # confirm the float winner exactly
        ok = count == 694395 and exact == F("57.236735")
        failed |= not ok
        print(f"{'PASS' if ok else 'FAIL'} optimum  valid={count} best={float(exact):.7f} plan={best[1]}")
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
