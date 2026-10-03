package main

import (
	"fmt"
	"net/http"
	"sync"
)

func runReviseGateLimit(client *apiClient) error {
	if err := verifyLimitRevisionHistory(client); err != nil {
		return err
	}
	if err := verifyPendingCommandsUnderNewLimit(client); err != nil {
		return err
	}
	if err := verifyActiveGateBlocksRevision(client); err != nil {
		return err
	}
	if err := verifyRevisionReviewConcurrency(client); err != nil {
		return err
	}

	fmt.Println("revise-gate-limit: limit revisions audited, stale reviews blocked, and concurrency has a single winner")
	return nil
}

func verifyLimitRevisionHistory(client *apiClient) error {
	if _, err := fixtureGate(client, "gate-limit-a", "甲号泄洪闸", 4.5, 0.5); err != nil {
		return err
	}

	revised, err := client.post("/gates/gate-limit-a/limit-revisions", map[string]any{
		"new_max_aperture": 5.5,
		"basis":            "structural-review-2026-09",
		"reason":           "结构复审确认允许更高开度",
		"requested_by":     "manager-qin",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(revised, http.StatusCreated, "revise aperture limit with evidence"); err != nil {
		return err
	}
	revision, err := objectField(revised.Body, "revision")
	if err != nil {
		return err
	}
	if sequence, err := floatField(revision, "sequence"); err != nil || sequence != 2 {
		return fmt.Errorf("revision sequence: expected 2, got %v (err=%v)", sequence, err)
	}
	if previous, err := floatField(revision, "previous_max_aperture"); err != nil || previous != 4.5 {
		return fmt.Errorf("revision previous limit: expected 4.5, got %v", previous)
	}
	if next, err := floatField(revision, "new_max_aperture"); err != nil || next != 5.5 {
		return fmt.Errorf("revision new limit: expected 5.5, got %v", next)
	}
	basis, err := stringField(revision, "basis")
	if err != nil || basis != "structural-review-2026-09" {
		return fmt.Errorf("revision basis: expected structural-review-2026-09, got %q", basis)
	}

	gateResult, err := client.get("/gates/gate-limit-a")
	if err != nil {
		return err
	}
	if err := requireStatus(gateResult, http.StatusOK, "read revised gate"); err != nil {
		return err
	}
	gate, err := objectField(gateResult.Body, "gate")
	if err != nil {
		return err
	}
	if max, err := floatField(gate, "max_aperture"); err != nil || max != 5.5 {
		return fmt.Errorf("revised gate limit: expected 5.5, got %v", max)
	}
	if revisionNo, err := floatField(gate, "limit_revision"); err != nil || revisionNo != 2 {
		return fmt.Errorf("revised gate revision: expected 2, got %v", revisionNo)
	}

	historyResult, err := client.get("/gates/gate-limit-a/limit-revisions")
	if err != nil {
		return err
	}
	if err := requireStatus(historyResult, http.StatusOK, "read limit revision history"); err != nil {
		return err
	}
	rawRevisions, ok := historyResult.Body["revisions"].([]any)
	if !ok || len(rawRevisions) != 2 {
		return fmt.Errorf("revision history: expected 2 entries, got %v", historyResult.Body["revisions"])
	}
	first, _ := rawRevisions[0].(map[string]any)
	second, _ := rawRevisions[1].(map[string]any)
	if seq, _ := floatField(first, "sequence"); seq != 1 {
		return fmt.Errorf("baseline revision sequence: expected 1, got %v", seq)
	}
	if b, _ := stringField(first, "basis"); b != "gate_registration" {
		return fmt.Errorf("baseline revision basis: expected gate_registration, got %q", b)
	}
	if seq, _ := floatField(second, "sequence"); seq != 2 {
		return fmt.Errorf("second revision sequence: expected 2, got %v", seq)
	}

	missingBasis, err := client.post("/gates/gate-limit-a/limit-revisions", map[string]any{
		"new_max_aperture": 6.0,
		"basis":            "",
		"reason":           "缺少依据",
		"requested_by":     "manager-qin",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(missingBasis, http.StatusBadRequest, "reject revision without evidence"); err != nil {
		return err
	}

	unchanged, err := client.post("/gates/gate-limit-a/limit-revisions", map[string]any{
		"new_max_aperture": 5.5,
		"basis":            "structural-review-2026-09",
		"reason":           "限值未变化",
		"requested_by":     "manager-qin",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(unchanged, http.StatusBadRequest, "reject unchanged limit revision"); err != nil {
		return err
	}

	belowCurrent, err := client.post("/gates/gate-limit-a/limit-revisions", map[string]any{
		"new_max_aperture": 0.4,
		"basis":            "structural-review-2026-09",
		"reason":           "试图把限值降到当前开度以下",
		"requested_by":     "manager-qin",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(belowCurrent, http.StatusConflict, "reject revision below current aperture"); err != nil {
		return err
	}

	unknown, err := client.get("/gates/gate-missing/limit-revisions")
	if err != nil {
		return err
	}
	if err := requireStatus(unknown, http.StatusNotFound, "revision history for unknown gate"); err != nil {
		return err
	}

	return nil
}

func verifyPendingCommandsUnderNewLimit(client *apiClient) error {
	if _, err := fixtureGate(client, "gate-limit-b", "乙号泄洪闸", 6, 0.5); err != nil {
		return err
	}

	first, err := fixtureCommand(client, "gate-limit-b", 5.0, "按旧限值申请的大开度", "dispatcher-lin")
	if err != nil {
		return err
	}
	firstID, err := stringField(first, "id")
	if err != nil {
		return err
	}
	if revisionNo, err := floatField(first, "limit_revision"); err != nil || revisionNo != 1 {
		return fmt.Errorf("new command revision: expected 1, got %v", revisionNo)
	}

	lowered, err := client.post("/gates/gate-limit-b/limit-revisions", map[string]any{
		"new_max_aperture": 4.2,
		"basis":            "structural-review-2026-10",
		"reason":           "结构复审后下调最大开度",
		"requested_by":     "manager-qin",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(lowered, http.StatusCreated, "lower limit while command pending"); err != nil {
		return err
	}

	// A pending command does not occupy the gate, but its old review basis
	// must not clear through the new limit without re-confirmation.
	stale, err := client.post("/commands/"+firstID+"/interlock", map[string]any{
		"verdict":  "clear",
		"verifier": "safety-wu",
		"note":     "未重新确认新限值",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(stale, http.StatusConflict, "reject stale clear after limit revision"); err != nil {
		return err
	}

	explicitStale, err := client.post("/commands/"+firstID+"/interlock", map[string]any{
		"verdict":        "clear",
		"verifier":       "safety-wu",
		"note":           "显式携带旧修订号",
		"limit_revision": 1,
	})
	if err != nil {
		return err
	}
	if err := requireStatus(explicitStale, http.StatusConflict, "reject explicitly stale clear"); err != nil {
		return err
	}

	infeasible, err := client.post("/commands/"+firstID+"/interlock", map[string]any{
		"verdict":        "clear",
		"verifier":       "safety-wu",
		"note":           "已重新确认但目标超出新限值",
		"limit_revision": 2,
	})
	if err != nil {
		return err
	}
	if err := requireStatus(infeasible, http.StatusConflict, "reject clear for target beyond new limit"); err != nil {
		return err
	}

	// A blocked verdict occupies nothing and can still close the command out.
	blocked, err := client.post("/commands/"+firstID+"/interlock", map[string]any{
		"verdict":        "blocked",
		"verifier":       "safety-wu",
		"note":           "新限值下不再允许该目标",
		"limit_revision": 2,
	})
	if err != nil {
		return err
	}
	if err := requireStatus(blocked, http.StatusOK, "block stale command against current limit"); err != nil {
		return err
	}

	reReview, err := client.post("/commands/"+firstID+"/interlock", map[string]any{
		"verdict":        "clear",
		"verifier":       "safety-wu",
		"note":           "终态命令不可再次复核",
		"limit_revision": 2,
	})
	if err != nil {
		return err
	}
	if err := requireStatus(reReview, http.StatusConflict, "reject review of rejected command"); err != nil {
		return err
	}

	second, err := fixtureCommand(client, "gate-limit-b", 3.0, "新限值下的开度申请", "dispatcher-lin")
	if err != nil {
		return err
	}
	secondID, err := stringField(second, "id")
	if err != nil {
		return err
	}
	if revisionNo, err := floatField(second, "limit_revision"); err != nil || revisionNo != 2 {
		return fmt.Errorf("command under new limit: expected revision 2, got %v", revisionNo)
	}

	raised, err := client.post("/gates/gate-limit-b/limit-revisions", map[string]any{
		"new_max_aperture": 7.0,
		"basis":            "structural-review-2026-11",
		"reason":           "加固完成后上调最大开度",
		"requested_by":     "manager-qin",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(raised, http.StatusCreated, "raise limit while another command pending"); err != nil {
		return err
	}

	staleAgain, err := client.post("/commands/"+secondID+"/interlock", map[string]any{
		"verdict":        "clear",
		"verifier":       "safety-wu",
		"note":           "限值再次变化后的旧依据",
		"limit_revision": 2,
	})
	if err != nil {
		return err
	}
	if err := requireStatus(staleAgain, http.StatusConflict, "reject clear acknowledged against prior revision"); err != nil {
		return err
	}

	approved, err := client.post("/commands/"+secondID+"/interlock", map[string]any{
		"verdict":        "clear",
		"verifier":       "safety-wu",
		"note":           "已按最新限值重新确认",
		"limit_revision": 3,
	})
	if err != nil {
		return err
	}
	if err := requireStatus(approved, http.StatusOK, "clear after re-confirmation"); err != nil {
		return err
	}
	approvedCommand, err := objectField(approved.Body, "command")
	if err != nil {
		return err
	}
	token, err := stringField(approvedCommand, "execution_token")
	if err != nil || token == "" {
		return fmt.Errorf("re-confirmed clear review did not issue a token")
	}

	executed, err := client.post("/commands/"+secondID+"/execute", map[string]any{
		"operator":       "operator-chen",
		"token":          token,
		"simulate_fault": false,
	})
	if err != nil {
		return err
	}
	if err := requireStatus(executed, http.StatusOK, "execute re-confirmed command"); err != nil {
		return err
	}

	// Historical reads keep the frozen facts while exposing the current limit.
	firstRead, err := client.get("/commands/" + firstID)
	if err != nil {
		return err
	}
	if err := requireStatus(firstRead, http.StatusOK, "read rejected command history"); err != nil {
		return err
	}
	firstView, err := objectField(firstRead.Body, "command")
	if err != nil {
		return err
	}
	if status, _ := stringField(firstView, "status"); status != "rejected" {
		return fmt.Errorf("historical rejected command changed to %q", status)
	}
	if frozen, _ := floatField(firstView, "limit_max_aperture"); frozen != 6 {
		return fmt.Errorf("historical command frozen limit: expected 6, got %v", frozen)
	}

	secondRead, err := client.get("/commands/" + secondID)
	if err != nil {
		return err
	}
	if err := requireStatus(secondRead, http.StatusOK, "read completed command history"); err != nil {
		return err
	}
	secondView, err := objectField(secondRead.Body, "command")
	if err != nil {
		return err
	}
	if status, _ := stringField(secondView, "status"); status != "completed" {
		return fmt.Errorf("historical completed command changed to %q", status)
	}
	if frozen, _ := floatField(secondView, "limit_max_aperture"); frozen != 4.2 {
		return fmt.Errorf("completed command frozen limit: expected 4.2, got %v", frozen)
	}
	if current, _ := floatField(secondView, "current_max_aperture"); current != 7 {
		return fmt.Errorf("completed command current limit: expected 7, got %v", current)
	}
	if current, _ := secondView["limit_current"].(bool); current {
		return fmt.Errorf("completed command basis must not be reported as current")
	}

	belowAperture, err := client.post("/gates/gate-limit-b/limit-revisions", map[string]any{
		"new_max_aperture": 2.9,
		"basis":            "structural-review-2026-12",
		"reason":           "当前开度 3.0，不允许下调到其以下",
		"requested_by":     "manager-qin",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(belowAperture, http.StatusConflict, "reject revision below settled aperture"); err != nil {
		return err
	}

	return nil
}

func verifyActiveGateBlocksRevision(client *apiClient) error {
	if _, err := fixtureGate(client, "gate-limit-c", "丙号泄洪闸", 4, 0.5); err != nil {
		return err
	}
	command, err := fixtureCommand(client, "gate-limit-c", 2.0, "占用期间禁止修订", "dispatcher-zhao")
	if err != nil {
		return err
	}
	commandID, err := stringField(command, "id")
	if err != nil {
		return err
	}
	approved, err := fixtureClearReview(client, commandID, "safety-he")
	if err != nil {
		return err
	}
	token, err := stringField(approved, "execution_token")
	if err != nil {
		return err
	}

	whileReserved, err := client.post("/gates/gate-limit-c/limit-revisions", map[string]any{
		"new_max_aperture": 5.0,
		"basis":            "structural-review-2026-13",
		"reason":           "闸门已被活动命令占用",
		"requested_by":     "manager-qin",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(whileReserved, http.StatusConflict, "reject revision while gate reserved"); err != nil {
		return err
	}

	faulted, err := client.post("/commands/"+commandID+"/execute", map[string]any{
		"operator":       "operator-chen",
		"token":          token,
		"simulate_fault": true,
	})
	if err != nil {
		return err
	}
	if err := requireStatus(faulted, http.StatusOK, "simulate fault to latch gate"); err != nil {
		return err
	}

	whileFaulted, err := client.post("/gates/gate-limit-c/limit-revisions", map[string]any{
		"new_max_aperture": 5.0,
		"basis":            "structural-review-2026-14",
		"reason":           "闸门处于故障态",
		"requested_by":     "manager-qin",
	})
	if err != nil {
		return err
	}
	if err := requireStatus(whileFaulted, http.StatusConflict, "reject revision while gate faulted"); err != nil {
		return err
	}

	return nil
}

func verifyRevisionReviewConcurrency(client *apiClient) error {
	const rounds = 40
	var revisionWins, reviewWins int

	for round := 0; round < rounds; round++ {
		gateID := fmt.Sprintf("gate-race-%02d", round)
		if _, err := fixtureGate(client, gateID, "并发验证闸", 6, 0.5); err != nil {
			return err
		}
		command, err := fixtureCommand(client, gateID, 2.0, "修订与复核并发", "dispatcher-lin")
		if err != nil {
			return err
		}
		commandID, err := stringField(command, "id")
		if err != nil {
			return err
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		var mu sync.Mutex
		revisionStatus := 0
		reviewStatuses := make([]int, 0, 6)
		errs := make([]string, 0)

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := client.post("/gates/"+gateID+"/limit-revisions", map[string]any{
				"new_max_aperture": 4.0,
				"basis":            "structural-review-concurrent",
				"reason":           "并发下调限值",
				"requested_by":     "manager-qin",
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err.Error())
				return
			}
			revisionStatus = result.Status
		}()

		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				result, err := client.post("/commands/"+commandID+"/interlock", map[string]any{
					"verdict":        "clear",
					"verifier":       "safety-wu",
					"note":           "并发复核",
					"limit_revision": 1,
				})
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs = append(errs, err.Error())
					return
				}
				reviewStatuses = append(reviewStatuses, result.Status)
			}()
		}

		close(start)
		wg.Wait()

		if len(errs) > 0 {
			return fmt.Errorf("round %d: concurrent requests failed: %v", round, errs)
		}

		approved := 0
		for _, status := range reviewStatuses {
			if status == http.StatusOK {
				approved++
			} else if status != http.StatusConflict {
				return fmt.Errorf("round %d: unexpected review status %d", round, status)
			}
		}
		if approved > 1 {
			return fmt.Errorf("round %d: %d clear reviews won; exactly one winner required", round, approved)
		}

		gateResult, err := client.get("/gates/" + gateID)
		if err != nil {
			return err
		}
		gate, err := objectField(gateResult.Body, "gate")
		if err != nil {
			return err
		}
		commandResult, err := client.get("/commands/" + commandID)
		if err != nil {
			return err
		}
		commandView, err := objectField(commandResult.Body, "command")
		if err != nil {
			return err
		}
		commandStatus, _ := stringField(commandView, "status")
		gateStatus, _ := stringField(gate, "status")
		activeID, _ := stringField(gate, "active_command_id")
		maxAperture, _ := floatField(gate, "max_aperture")

		if approved == 1 {
			reviewWins++
			if revisionStatus != http.StatusConflict {
				return fmt.Errorf("round %d: review won but revision status was %d", round, revisionStatus)
			}
			if gateStatus != "reserved" || activeID != commandID || maxAperture != 6 {
				return fmt.Errorf("round %d: review won but gate is status=%q active=%q max=%v", round, gateStatus, activeID, maxAperture)
			}
			if commandStatus != "approved" {
				return fmt.Errorf("round %d: review won but command status=%q", round, commandStatus)
			}
		} else {
			revisionWins++
			if revisionStatus != http.StatusCreated {
				return fmt.Errorf("round %d: revision won but revision status was %d", round, revisionStatus)
			}
			if gateStatus != "idle" || activeID != "" || maxAperture != 4 {
				return fmt.Errorf("round %d: revision won but gate is status=%q active=%q max=%v", round, gateStatus, activeID, maxAperture)
			}
			if commandStatus != "pending" {
				return fmt.Errorf("round %d: revision won but command status=%q", round, commandStatus)
			}
		}
	}

	if revisionWins == 0 || reviewWins == 0 {
		return fmt.Errorf("concurrency rounds never exercised both orders: revisionWins=%d reviewWins=%d", revisionWins, reviewWins)
	}
	return nil
}
