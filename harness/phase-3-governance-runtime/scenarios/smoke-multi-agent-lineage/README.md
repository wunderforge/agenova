# Scenario: smoke-multi-agent-lineage

Verifies that a parent claim can spawn child claims, that all three receive governance
through the Tool Gateway and Model Gateway, and that child claims become out-of-scope
when the parent terminates.

## Phase

phase-3-governance-runtime

## Expected Outcomes

1. Parent (orchestrator) claim authorized for tool and model calls while Running.
2. Child (worker-a, worker-b) claims authorized for tool and model calls while parent is Running.
3. Facts recorded under the correct claim ID (not cross-attributed).
4. Lineage reflects orchestrator -> [worker-a, worker-b].
5. After orchestrator terminates, worker-a and worker-b are denied (out-of-scope).

## Check

MultiAgentLineageCheck
