// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package facts

import "fmt"

// ReserveModelCall serializes the quota check and durable reservation. A failed
// append never dispatches a model. Reservations count attempted requests even
// when a later provider/recording step fails. Existing recovery is fail-closed:
// interrupted Work is reconciled, never re-executed with a new empty counter.
func (j *Journal) ReserveModelCall(ref, claim, id string, limit int) (bool, error) {
	j.budgetMu.Lock()
	defer j.budgetMu.Unlock()
	if id == "" || limit < 1 || limit > 1000 {
		return false, fmt.Errorf("invalid model budget reservation")
	}
	count := 0
	for _, f := range j.ForRequest(ref) {
		if f.Kind == "ModelBudgetReservation" {
			if f.InvocationID == id || f.ModelCallLimit != limit {
				return false, fmt.Errorf("duplicate or changed model budget reservation")
			}
			count++
		}
	}
	if count >= limit {
		return false, nil
	}
	_, err := j.Append(Fact{Kind: "ModelBudgetReservation", RequestRef: ref, ClaimID: claim, InvocationID: id, Operation: "model.invoke", ReasonCode: "model-call-reserved", ModelCallCount: count + 1, ModelCallLimit: limit})
	return err == nil, err
}
