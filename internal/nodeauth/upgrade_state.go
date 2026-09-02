package nodeauth

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

const (
	relationshipUpgradeRetryDelay = time.Hour
	relationshipUpgradeStaleAfter = 10 * time.Minute
	relationshipUpgradeQueueSize  = 128
)

var (
	ErrRelationshipUpgradeAlreadyRunning = errors.New("node authentication upgrade is already running")
	ErrRelationshipUpgradeNotAvailable   = errors.New("node authentication upgrade is not available")

	relationshipUpgradeQueue      = make(chan uint64, relationshipUpgradeQueueSize)
	relationshipUpgradeWorkerOnce sync.Once
)

type authUpgradeFailure struct {
	Step    string
	Code    string
	Message string
}

func StartRelationshipUpgradeWorker(ctx context.Context, controllerInstanceID string) {
	relationshipUpgradeWorkerOnce.Do(func() {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case nodeID := <-relationshipUpgradeQueue:
					attemptCtx, cancel := context.WithTimeout(ctx, time.Minute)
					err := RunLegacyRelationshipUpgrade(attemptCtx, nodeID, controllerInstanceID, time.Now())
					cancel()
					if err != nil && !errors.Is(err, ErrRelationshipUpgradeAlreadyRunning) {
						logger.Warnf("Automatic node authentication upgrade failed for node %d: %v", nodeID, err)
					}
				}
			}
		}()
	})
}

func QueueLegacyRelationshipUpgrade(nodeID uint64) bool {
	select {
	case relationshipUpgradeQueue <- nodeID:
		return true
	default:
		return false
	}
}

func RunLegacyRelationshipUpgrade(ctx context.Context, nodeID uint64, controllerInstanceID string,
	now time.Time,
) error {
	database := model.UseDB()
	if database == nil {
		return errors.New("node authentication database is unavailable")
	}

	var node model.Node
	if err := database.First(&node, nodeID).Error; err != nil {
		return err
	}
	if node.AuthMethod != model.NodeAuthMethodLegacy || !node.Enabled || len(node.EncryptedLegacySecret) == 0 {
		return ErrRelationshipUpgradeNotAvailable
	}

	staleBefore := now.Add(-relationshipUpgradeStaleAfter)
	result := database.Model(&model.Node{}).
		Where("id = ? AND auth_method = ?", nodeID, model.NodeAuthMethodLegacy).
		Where("auth_upgrade_status IS NULL OR auth_upgrade_status <> ? OR auth_upgrade_attempted_at IS NULL OR auth_upgrade_attempted_at < ?",
			model.NodeAuthUpgradeStatusInProgress, staleBefore).
		Updates(map[string]any{
			"auth_upgrade_status":        model.NodeAuthUpgradeStatusInProgress,
			"auth_upgrade_step":          model.NodeAuthUpgradeStepRequest,
			"auth_upgrade_attempt_count": gorm.Expr("auth_upgrade_attempt_count + 1"),
			"auth_upgrade_attempted_at":  now,
			"auth_upgrade_next_retry_at": nil,
			"auth_upgrade_error_code":    "",
			"auth_upgrade_error":         "",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRelationshipUpgradeAlreadyRunning
	}

	reportStep := func(step string) {
		_ = database.Model(&model.Node{}).
			Where("id = ? AND auth_upgrade_status = ?", nodeID, model.NodeAuthUpgradeStatusInProgress).
			Update("auth_upgrade_step", step).Error
	}
	_, err := upgradeLegacyRelationship(ctx, &node, controllerInstanceID, reportStep)
	if err == nil {
		return nil
	}

	failure := classifyAuthUpgradeFailure(err)
	nextRetryAt := now.Add(relationshipUpgradeRetryDelay)
	status := model.NodeAuthUpgradeStatusFailed
	if failure.Code == model.NodeAuthUpgradeErrorTargetUnsupported {
		status = model.NodeAuthUpgradeStatusWaitingTarget
	}
	if updateErr := database.Model(&model.Node{}).Where("id = ?", nodeID).Updates(map[string]any{
		"auth_upgrade_status":        status,
		"auth_upgrade_step":          failure.Step,
		"auth_upgrade_next_retry_at": nextRetryAt,
		"auth_upgrade_error_code":    failure.Code,
		"auth_upgrade_error":         failure.Message,
	}).Error; updateErr != nil {
		return errors.Join(err, updateErr)
	}
	if status == model.NodeAuthUpgradeStatusWaitingTarget {
		return nil
	}
	return err
}

func RetryLegacyRelationshipUpgrade(nodeID uint64, now time.Time) error {
	database := model.UseDB()
	if database == nil {
		return errors.New("node authentication database is unavailable")
	}

	var node model.Node
	if err := database.First(&node, nodeID).Error; err != nil {
		return err
	}
	if node.AuthMethod != model.NodeAuthMethodLegacy || !node.Enabled || len(node.EncryptedLegacySecret) == 0 {
		return ErrRelationshipUpgradeNotAvailable
	}
	if node.AuthUpgradeStatus == model.NodeAuthUpgradeStatusInProgress && node.AuthUpgradeAttemptedAt != nil &&
		node.AuthUpgradeAttemptedAt.After(now.Add(-relationshipUpgradeStaleAfter)) {
		return ErrRelationshipUpgradeAlreadyRunning
	}

	if err := database.Model(&node).Updates(map[string]any{
		"auth_upgrade_status":        model.NodeAuthUpgradeStatusPending,
		"auth_upgrade_step":          model.NodeAuthUpgradeStepQueued,
		"auth_upgrade_next_retry_at": now,
		"auth_upgrade_error_code":    "",
		"auth_upgrade_error":         "",
	}).Error; err != nil {
		return err
	}
	if !QueueLegacyRelationshipUpgrade(nodeID) {
		return errors.New("node authentication upgrade queue is full")
	}
	return nil
}

func classifyAuthUpgradeFailure(err error) authUpgradeFailure {
	failure := authUpgradeFailure{
		Step:    model.NodeAuthUpgradeStepRequest,
		Code:    model.NodeAuthUpgradeErrorInternal,
		Message: "The authentication upgrade failed because of an internal error.",
	}
	var upgradeErr *relationshipUpgradeError
	if errors.As(err, &upgradeErr) {
		failure.Step = upgradeErr.Step
	}
	if errors.Is(err, ErrRelationshipUnsupported) {
		failure.Code = model.NodeAuthUpgradeErrorTargetUnsupported
		failure.Message = "The target node does not support paired signatures yet."
		return failure
	}
	if errors.Is(err, ErrLegacySecretMissing) {
		failure.Code = model.NodeAuthUpgradeErrorMissingLegacySecret
		failure.Message = "The stored legacy node secret is unavailable."
		return failure
	}
	if errors.Is(err, ErrUpgradeProofInvalid) {
		failure.Code = model.NodeAuthUpgradeErrorInvalidConfirmation
		failure.Message = "The target node returned an invalid upgrade confirmation."
		return failure
	}
	var httpErr *relationshipHTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden {
			failure.Code = model.NodeAuthUpgradeErrorAuthenticationRejected
			failure.Message = "The target node rejected the stored node secret."
			return failure
		}
		failure.Code = model.NodeAuthUpgradeErrorTargetRejected
		failure.Message = "The target node rejected the authentication upgrade request."
		return failure
	}
	if errors.Is(err, context.DeadlineExceeded) {
		failure.Code = model.NodeAuthUpgradeErrorTimeout
		failure.Message = "The target node did not respond before the upgrade timed out."
		return failure
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		failure.Code = model.NodeAuthUpgradeErrorConnectionFailed
		failure.Message = "The target node could not be reached."
		return failure
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) || failure.Step == model.NodeAuthUpgradeStepVerify {
		failure.Code = model.NodeAuthUpgradeErrorInvalidResponse
		failure.Message = "The target node returned an invalid pairing response."
		return failure
	}
	if failure.Step == model.NodeAuthUpgradeStepPersist {
		failure.Code = model.NodeAuthUpgradeErrorPersistenceFailed
		failure.Message = "The paired credential could not be saved."
	}
	return failure
}
