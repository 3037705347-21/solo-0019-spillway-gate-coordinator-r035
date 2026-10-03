package main

import (
	"fmt"
	"net/http"
	"sync"
)

func runReviseGateLimit(client *apiClient) error {
	// --- registration is limit revision 1 ----------------------------------
	if _, err := fixtureGate(client, "gate-limit", "限幅复审闸", 5.0, 1.0); err != nil {
		return err
	}
	history, err := getRevisions(client, "gate-limit")
	if err != nil {
		return err
	}
	if len(history) != 1 {
		return fmt.Errorf("initial history: expected 1 revision, got %d", len(history))
	}
	if revisionOf(history[0]) != 1 || sourceOf(history[0]) != "registration" || maxApertureOf(history[0]) != 5.0 {
		return fmt.Errorf("registration revision mismatch: %v", history[0])
	}

	// --- pending command snapshots the limit basis -------------------------
	pending, err := fixtureCommand(client, "gate-limit", 4.5, "结构复审核查", "dispatcher-qin")
	if err != nil {
		return err
	}
	pendingID, err := stringField(pending, "id")
	if err != nil {
		return err
	}
	if revision, _ := floatField(pending, "basis_limit_revision"); revision != 1 {
		return fmt.Errorf("new command basis revision: expected 1, got %v", revision)
	}
	if current, _ := boolField(pending, "basis_current"); !current {
		return fmt.Errorf("new command should be on the current basis")
	}

	// --- a cited revision lowers the limit ---------------------------------
	if err := reviseLimit(client, "gate-limit", 3.0, "structural-review-2026-01", "结构复审下调限幅", "manager-luo", http.StatusCreated); err != nil {
		return err
	}
	gateBody, err := getGate(client, "gate-limit")
	if err != nil {
		return err
	}
	if max, _ := floatField(gateBody, "max_aperture"); max != 3.0 {
		return fmt.Errorf("gate max after revision: expected 3, got %v", max)
	}
	if revision, _ := floatField(gateBody, "limit_revision"); revision != 2 {
		return fmt.Errorf("gate limit revision: expected 2, got %v", revision)
	}

	// revision requires structural-review evidence
	missingBasis := postRevise(client, "gate-limit", map[string]any{
		"max_aperture": 2.5,
		"reason":       "缺少依据",
		"revised_by":   "manager-luo",
	})
	if err := requireStatus(missingBasis, http.StatusBadRequest, "revision without basis reference"); err != nil {
		return err
	}
	if code, _ := errorCode(missingBasis); code != "invalid_revision_basis" {
		return fmt.Errorf("missing basis error code: got %q", code)
	}

	// --- stale pending command is explained by reads -----------------------
	staleRead, err := getCommand(client, pendingID)
	if err != nil {
		return err
	}
	if current, _ := boolField(staleRead, "basis_current"); current {
		return fmt.Errorf("pending command should report a stale basis after revision")
	}
	if max, _ := floatField(staleRead, "basis_max_aperture"); max != 5.0 {
		return fmt.Errorf("command basis max must keep the frozen value 5, got %v", max)
	}

	// --- old basis cannot clear through the new limit without reconfirm ----
	staleClear := postMust(client, "/commands/"+pendingID+"/interlock", map[string]any{
		"verdict":  "clear",
		"verifier": "safety-wu",
		"note":     "试图绕过新限幅",
	})
	if err := requireStatus(staleClear, http.StatusConflict, "clear review on stale basis"); err != nil {
		return err
	}
	if code, _ := errorCode(staleClear); code != "command_limit_basis_stale" {
		return fmt.Errorf("stale clear error code: got %q", code)
	}

	// --- reconfirm against the lower limit fails for an over-limit target --
	rejectReconfirm := postMust(client, "/commands/"+pendingID+"/reconfirm", map[string]any{
		"confirmed_by": "dispatcher-qin",
	})
	if err := requireStatus(rejectReconfirm, http.StatusBadRequest, "reconfirm target over revised limit"); err != nil {
		return err
	}
	if code, _ := errorCode(rejectReconfirm); code != "invalid_target_aperture" {
		return fmt.Errorf("reconfirm out-of-range error code: got %q", code)
	}

	// a blocked safety verdict stays valid on a stale basis: it rejects the
	// command without reserving the gate
	blocked := postMust(client, "/commands/"+pendingID+"/interlock", map[string]any{
		"verdict":  "blocked",
		"verifier": "safety-wu",
		"note":     "目标超出新限幅",
	})
	if err := requireStatus(blocked, http.StatusOK, "blocked review on stale basis"); err != nil {
		return err
	}
	blockedCommand, err := objectField(blocked.Body, "command")
	if err != nil {
		return err
	}
	if status, _ := stringField(blockedCommand, "status"); status != "rejected" {
		return fmt.Errorf("blocked review status: expected rejected, got %q", status)
	}
	gateBody, err = getGate(client, "gate-limit")
	if err != nil {
		return err
	}
	if status, _ := stringField(gateBody, "status"); status != "idle" {
		return fmt.Errorf("blocked review must not reserve gate, status=%q", status)
	}

	// --- revision history is immutable and complete ------------------------
	history, err = getRevisions(client, "gate-limit")
	if err != nil {
		return err
	}
	if len(history) != 2 {
		return fmt.Errorf("history after revision: expected 2 entries, got %d", len(history))
	}
	second := history[1]
	if revisionOf(second) != 2 || maxApertureOf(second) != 3.0 ||
		sourceOf(second) != "structural_review" || basisOf(second) != "structural-review-2026-01" {
		return fmt.Errorf("revision 2 record mismatch: %v", second)
	}

	// --- a new command and a reconfirmed command work against the new limit
	fresh, err := fixtureCommand(client, "gate-limit", 2.0, "限幅内命令", "dispatcher-qin")
	if err != nil {
		return err
	}
	freshID, _ := stringField(fresh, "id")
	if revision, _ := floatField(fresh, "basis_limit_revision"); revision != 2 {
		return fmt.Errorf("new command basis revision: expected 2, got %v", revision)
	}

	// raising the limit also invalidates an in-range pending basis: the old
	// basis must be reconfirmed before it can execute
	if err := reviseLimit(client, "gate-limit", 4.0, "structural-review-2026-02", "结构复审上调限幅", "manager-luo", http.StatusCreated); err != nil {
		return err
	}
	staleClear = postMust(client, "/commands/"+freshID+"/interlock", map[string]any{
		"verdict":  "clear",
		"verifier": "safety-wu",
		"note":     "上调后未重新确认",
	})
	if err := requireStatus(staleClear, http.StatusConflict, "clear review stale after limit raise"); err != nil {
		return err
	}
	reconfirm := postMust(client, "/commands/"+freshID+"/reconfirm", map[string]any{
		"confirmed_by": "dispatcher-qin",
	})
	if err := requireStatus(reconfirm, http.StatusOK, "reconfirm in-range command"); err != nil {
		return err
	}
	reconfirmedCommand, err := objectField(reconfirm.Body, "command")
	if err != nil {
		return err
	}
	if revision, _ := floatField(reconfirmedCommand, "basis_limit_revision"); revision != 3 {
		return fmt.Errorf("reconfirmed basis revision: expected 3, got %v", revision)
	}
	if current, _ := boolField(reconfirmedCommand, "basis_current"); !current {
		return fmt.Errorf("reconfirmed command should report a current basis")
	}

	// --- revision is refused while the gate is actively occupied -----------
	occupied := postMust(client, "/commands/"+freshID+"/interlock", map[string]any{
		"verdict":  "clear",
		"verifier": "safety-he",
		"note":     "占用闸门",
	})
	if err := requireStatus(occupied, http.StatusOK, "clear reconfirmed command"); err != nil {
		return err
	}
	approvedCommand, err := objectField(occupied.Body, "command")
	if err != nil {
		return err
	}
	token, _ := stringField(approvedCommand, "execution_token")
	if token == "" {
		return fmt.Errorf("approved command must carry a token")
	}
	if err := reviseLimit(client, "gate-limit", 3.5, "structural-review-2026-03", "占用期间修订", "manager-luo", http.StatusConflict); err != nil {
		return err
	}

	// execute the approved command against its frozen limit and aperture
	executed := postMust(client, "/commands/"+freshID+"/execute", map[string]any{
		"operator":       "operator-chen",
		"token":          token,
		"simulate_fault": false,
	})
	if err := requireStatus(executed, http.StatusOK, "execute approved command"); err != nil {
		return err
	}
	executedCommand, err := objectField(executed.Body, "command")
	if err != nil {
		return err
	}
	if status, _ := stringField(executedCommand, "status"); status != "completed" {
		return fmt.Errorf("execution status: expected completed, got %q", status)
	}
	// executed fact keeps its own basis snapshot and is not rewritten
	if revision, _ := floatField(executedCommand, "basis_limit_revision"); revision != 3 {
		return fmt.Errorf("completed command must keep basis revision 3, got %v", revision)
	}
	history, err = getRevisions(client, "gate-limit")
	if err != nil {
		return err
	}
	if len(history) != 3 {
		return fmt.Errorf("refused revisions must not be recorded, got %d entries", len(history))
	}

	// revision cannot go below the physical aperture
	below := postRevise(client, "gate-limit", map[string]any{
		"max_aperture":    1.0,
		"basis_reference": "structural-review-2026-04",
		"reason":          "低于当前开度",
		"revised_by":      "manager-luo",
	})
	if err := requireStatus(below, http.StatusBadRequest, "revision below current aperture"); err != nil {
		return err
	}
	if code, _ := errorCode(below); code != "limit_below_current_aperture" {
		return fmt.Errorf("below-aperture error code: got %q", code)
	}

	// --- faulted gates cannot be revised -----------------------------------
	if _, err := fixtureGate(client, "gate-fault", "故障闸", 4.0, 1.0); err != nil {
		return err
	}
	faultCommand, err := fixtureCommand(client, "gate-fault", 3.0, "故障演练", "dispatcher-qin")
	if err != nil {
		return err
	}
	faultID, _ := stringField(faultCommand, "id")
	faultApproved, err := fixtureClearReview(client, faultID, "safety-he")
	if err != nil {
		return err
	}
	faultToken, _ := stringField(faultApproved, "execution_token")
	faultExec := postMust(client, "/commands/"+faultID+"/execute", map[string]any{
		"operator":       "operator-chen",
		"token":          faultToken,
		"simulate_fault": true,
	})
	if err := requireStatus(faultExec, http.StatusOK, "simulate fault"); err != nil {
		return err
	}
	if err := reviseLimit(client, "gate-fault", 5.0, "structural-review-2026-05", "故障时修订", "manager-luo", http.StatusConflict); err != nil {
		return err
	}

	// --- revision versus review run concurrently: exactly one winner -------
	if err := verifyRevisionReviewConcurrency(client); err != nil {
		return err
	}

	fmt.Println("revise-gate-limit: limit revisions, stale basis, reconfirm, occupancy and history verified")
	return nil
}

func verifyRevisionReviewConcurrency(client *apiClient) error {
	for iteration := 0; iteration < 8; iteration++ {
		gateID := fmt.Sprintf("gate-race-%d", iteration)
		if _, err := fixtureGate(client, gateID, "并发修订闸", 5.0, 1.0); err != nil {
			return err
		}
		command, err := fixtureCommand(client, gateID, 4.5, "并发竞争", "dispatcher-qin")
		if err != nil {
			return err
		}
		commandID, _ := stringField(command, "id")

		var wg sync.WaitGroup
		wg.Add(2)
		var revisionResult, reviewResult apiResult
		go func() {
			defer wg.Done()
			revisionResult = postRevise(client, gateID, map[string]any{
				"max_aperture":    4.0,
				"basis_reference": fmt.Sprintf("race-review-%d", iteration),
				"reason":          "与复核并发",
				"revised_by":      "manager-luo",
			})
		}()
		go func() {
			defer wg.Done()
			reviewResult = postMust(client, "/commands/"+commandID+"/interlock", map[string]any{
				"verdict":  "clear",
				"verifier": "safety-wu",
				"note":     "与修订并发",
			})
		}()
		wg.Wait()

		revisionWon := revisionResult.Status == http.StatusCreated
		reviewWon := reviewResult.Status == http.StatusOK
		if revisionWon == reviewWon {
			return fmt.Errorf(
				"iteration %d: expected exactly one winner, revision=%d review=%d",
				iteration, revisionResult.Status, reviewResult.Status,
			)
		}
		if revisionWon {
			// The clear review must have been refused for staleness and the
			// gate must remain idle.
			if code, _ := errorCode(reviewResult); code != "command_limit_basis_stale" {
				return fmt.Errorf("iteration %d: losing review code was %q", iteration, code)
			}
			gateBody, err := getGate(client, gateID)
			if err != nil {
				return err
			}
			if status, _ := stringField(gateBody, "status"); status != "idle" {
				return fmt.Errorf("iteration %d: gate should stay idle after revision wins", iteration)
			}
		} else {
			// The review won and reserved the gate; revision must be refused
			// and the approved command must hold the reservation.
			if code, _ := errorCode(revisionResult); code != "gate_not_idle" {
				return fmt.Errorf("iteration %d: losing revision code was %q", iteration, code)
			}
			reviewBody, err := objectField(reviewResult.Body, "command")
			if err != nil {
				return err
			}
			if status, _ := stringField(reviewBody, "status"); status != "approved" {
				return fmt.Errorf("iteration %d: winning review status was %q", iteration, status)
			}
		}
	}
	return nil
}

func postMust(client *apiClient, path string, payload map[string]any) apiResult {
	result, err := client.post(path, payload)
	if err != nil {
		return apiResult{Status: 0, Body: map[string]any{"code": "transport_error", "message": err.Error()}}
	}
	return result
}

func postRevise(client *apiClient, gateID string, payload map[string]any) apiResult {
	return postMust(client, "/gates/"+gateID+"/limit-revisions", payload)
}

func reviseLimit(client *apiClient, gateID string, maxAperture float64, basis, reason, revisedBy string, expectedStatus int) error {
	result := postRevise(client, gateID, map[string]any{
		"max_aperture":    maxAperture,
		"basis_reference": basis,
		"reason":          reason,
		"revised_by":      revisedBy,
	})
	return requireStatus(result, expectedStatus, "revise gate limit")
}

func getRevisions(client *apiClient, gateID string) ([]any, error) {
	result, err := client.get("/gates/" + gateID + "/limit-revisions")
	if err != nil {
		return nil, err
	}
	if err := requireStatus(result, http.StatusOK, "read limit revisions"); err != nil {
		return nil, err
	}
	value, exists := result.Body["revisions"]
	if !exists {
		return nil, fmt.Errorf("limit revisions response missing revisions field")
	}
	revisions, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("limit revisions field is not an array")
	}
	return revisions, nil
}

func getGate(client *apiClient, gateID string) (map[string]any, error) {
	result, err := client.get("/gates/" + gateID)
	if err != nil {
		return nil, err
	}
	if err := requireStatus(result, http.StatusOK, "read gate"); err != nil {
		return nil, err
	}
	return objectField(result.Body, "gate")
}

func getCommand(client *apiClient, commandID string) (map[string]any, error) {
	result, err := client.get("/commands/" + commandID)
	if err != nil {
		return nil, err
	}
	if err := requireStatus(result, http.StatusOK, "read command"); err != nil {
		return nil, err
	}
	return objectField(result.Body, "command")
}

func revisionOf(entry any) float64 {
	object := entry.(map[string]any)
	value, _ := floatField(object, "revision")
	return value
}

func maxApertureOf(entry any) float64 {
	object := entry.(map[string]any)
	value, _ := floatField(object, "max_aperture")
	return value
}

func sourceOf(entry any) string {
	object := entry.(map[string]any)
	value, _ := stringField(object, "source")
	return value
}

func basisOf(entry any) string {
	object := entry.(map[string]any)
	value, _ := stringField(object, "basis_reference")
	return value
}
