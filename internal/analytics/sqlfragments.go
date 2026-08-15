package analytics

// attemptedPredicate is the SQL fragment that decides whether a step
// was attempted. It is a named constant with the rationale quoted above
// it, so that fifteen queries cannot each get it slightly wrong.
//
// Per M5-EPIC: red status ∈ {complete, blocked}. pending, running and
// skipped are NOT attempted.
//
// One query deliberately does not use this: MTTD measures detection latency
// rather than attempt coverage, and counts running executions too — see
// [mttdBegunPredicate] for why. Any other divergence is a bug.
const attemptedPredicate = `execution.status IN ('complete', 'blocked')`

// outcomeCase returns the SQL CASE expression that derives an outcome
// label from detection_category × protection, matching [scoring.DeriveOutcome].
//
// It produces the same answer as [scoring.DeriveOutcome] for every
// category × protection pair, including the nil cases — asserted by the
// drift test in analytics_test.go, which enumerates
// [scoring.AllCategories] × [scoring.AllProtections] and runs the SQL
// against a real database.
//
// The expression is a single block of SQL rather than a function call so
// that every rollup uses the same text, and so that the drift test is
// testing the SQL that actually ships, not a wrapper that could be
// bypassed by copying from the wrapper's source.
const outcomeCase = `
CASE
    -- Nil cases: no score → unscored.
    WHEN execution.detection_category IS NULL OR execution.protection IS NULL THEN 'unscored'
    -- n/a protection → not_applicable regardless of category.
    WHEN execution.protection = 'n/a' THEN 'not_applicable'
    -- blocked or partial → prevented regardless of category.
    WHEN execution.protection IN ('blocked', 'partial') THEN 'prevented'
    -- not_blocked: category decides.
    WHEN execution.detection_category = 'none' THEN 'not_detected'
    ELSE 'detected'
END
`

// Note: the CASE falls through to 'detected' for known non-none
// categories under not_blocked. The CHECK constraints on app.execution
// guarantee the category and protection values are in the closed
// vocabulary, so the ELSE branch cannot produce 'detected' for an
// unknown value — it would have been rejected on write.
