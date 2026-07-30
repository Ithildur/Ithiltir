package dashupdate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	updateRequestName     = "request.env"
	updateTransactionName = "transaction.env"
	updateBlockName       = "update.block"
)

type Phase string

const (
	PhaseQueued    Phase = "queued"
	PhaseLocked    Phase = "locked"
	PhaseDownload  Phase = "download"
	PhaseValidate  Phase = "validate"
	PhasePrepared  Phase = "prepared"
	PhaseStopping  Phase = "stopping"
	PhaseSwitching Phase = "switching"
	PhaseMigrating Phase = "migrating"
	PhaseStarting  Phase = "starting"
	PhaseCleanup   Phase = "cleanup"
	PhaseDone      Phase = "done"
)

type jobRequest struct {
	ID   string
	Unit string
	Plan Plan
}

type updateTransaction struct {
	FormatVersion    int
	JobID            string
	Phase            Phase
	PreviousTarget   string
	CandidateTarget  string
	CandidateDir     string
	TempRoot         string
	BackupDir        string
	Legacy           bool
	ServiceManager   string
	ServiceWasActive bool
	MigrationStarted bool
	CreatedAt        string
}

func statusForRequest(request jobRequest, logPath string, now time.Time) statusFile {
	return statusFile{
		ID:                      request.ID,
		Unit:                    request.Unit,
		Status:                  StatusRunning,
		Action:                  request.Plan.Action,
		Channel:                 request.Plan.Channel,
		Origin:                  request.Plan.Origin,
		TargetVersion:           request.Plan.TargetVersion,
		ExpectedCurrentVersion:  request.Plan.ExpectedCurrentVersion,
		ExpectedInstallRevision: request.Plan.ExpectedInstallRevision,
		Phase:                   PhaseQueued,
		StartedAt:               now.UTC().Format(time.RFC3339),
		LogFile:                 logPath,
	}
}

func statusFromDoneTransaction(item statusFile, txn updateTransaction, now time.Time) (statusFile, error) {
	if txn.Phase != PhaseDone {
		return item, fmt.Errorf("update transaction is not done")
	}
	if item.ID != txn.JobID {
		return item, fmt.Errorf("update status belongs to job %q, want %q", item.ID, txn.JobID)
	}
	code := 1
	item.Status = StatusFailed
	item.FailureCode = "rolled_back"
	if txn.MigrationStarted {
		code = 0
		item.Status = StatusCompleted
		item.Phase = PhaseDone
		item.FailureCode = ""
	}
	item.ExitCode = &code
	item.ExecutorPID = 0
	item.RecoveryPath = ""
	item.FinishedAt = now.UTC().Format(time.RFC3339)
	return item, nil
}

func newUpdateJobID(now time.Time) string {
	return now.UTC().Format("20060102T150405Z") + "-" + strconv.FormatInt(now.UnixNano(), 36)
}

func writeJobRequest(path string, req jobRequest) error {
	if !validJobID(req.ID) {
		return fmt.Errorf("invalid job ID %q", req.ID)
	}
	if err := req.Plan.validate(); err != nil {
		return err
	}
	var b strings.Builder
	writeStatusField(&b, "id", req.ID)
	writeStatusField(&b, "unit", req.Unit)
	writeStatusField(&b, "action", string(req.Plan.Action))
	writeStatusField(&b, "channel", string(req.Plan.Channel))
	writeStatusField(&b, "target_version", req.Plan.TargetVersion)
	writeStatusField(&b, "expected_current_version", req.Plan.ExpectedCurrentVersion)
	writeStatusField(&b, "expected_install_revision", req.Plan.ExpectedInstallRevision)
	writeStatusField(&b, "origin", string(req.Plan.Origin))
	writeStatusField(&b, "lang", req.Plan.Lang)
	writeStatusField(&b, "service_manager", normalizedServiceManager(req.Plan.ServiceManager))
	return writePrivateFile(path, []byte(b.String()))
}

func readJobRequest(path string) (jobRequest, error) {
	fields, err := readStatusFields(path)
	if err != nil {
		return jobRequest{}, err
	}
	id, err := requiredStatusField(fields, "id")
	if err != nil {
		return jobRequest{}, fmt.Errorf("invalid job ID: %w", err)
	}
	if !validJobID(id) {
		return jobRequest{}, fmt.Errorf("invalid job ID %q", id)
	}
	action, ok := ParseAction(Action(fields["action"]))
	if !ok {
		return jobRequest{}, fmt.Errorf("invalid action %q", fields["action"])
	}
	channel, ok := ParseChannel(Channel(fields["channel"]))
	if !ok {
		return jobRequest{}, fmt.Errorf("invalid channel %q", fields["channel"])
	}
	origin, ok := ParseOrigin(Origin(fields["origin"]))
	if !ok {
		return jobRequest{}, fmt.Errorf("invalid origin %q", fields["origin"])
	}
	req := jobRequest{
		ID:   id,
		Unit: strings.TrimSpace(fields["unit"]),
		Plan: Plan{
			Action:                  action,
			Channel:                 channel,
			TargetVersion:           strings.TrimSpace(fields["target_version"]),
			ExpectedCurrentVersion:  strings.TrimSpace(fields["expected_current_version"]),
			ExpectedInstallRevision: strings.TrimSpace(fields["expected_install_revision"]),
			Origin:                  origin,
			Lang:                    strings.TrimSpace(fields["lang"]),
			ServiceManager:          normalizedServiceManager(fields["service_manager"]),
		},
	}
	if req.Unit == "" {
		return jobRequest{}, errors.New("update request unit is required")
	}
	if err := req.Plan.validate(); err != nil {
		return jobRequest{}, err
	}
	return req, nil
}

func transactionData(txn updateTransaction) ([]byte, error) {
	if txn.FormatVersion == 0 {
		txn.FormatVersion = 1
	}
	if txn.FormatVersion != 1 || !validJobID(txn.JobID) {
		return nil, errors.New("invalid update transaction")
	}
	var b strings.Builder
	writeStatusField(&b, "format_version", strconv.Itoa(txn.FormatVersion))
	writeStatusField(&b, "job_id", txn.JobID)
	writeStatusField(&b, "phase", string(txn.Phase))
	writeStatusField(&b, "previous_target", txn.PreviousTarget)
	writeStatusField(&b, "candidate_target", txn.CandidateTarget)
	writeStatusField(&b, "candidate_dir", txn.CandidateDir)
	writeStatusField(&b, "temp_root", txn.TempRoot)
	writeStatusField(&b, "backup_dir", txn.BackupDir)
	writeStatusField(&b, "legacy", strconv.FormatBool(txn.Legacy))
	writeStatusField(&b, "service_manager", txn.ServiceManager)
	writeStatusField(&b, "service_was_active", strconv.FormatBool(txn.ServiceWasActive))
	writeStatusField(&b, "migration_started", strconv.FormatBool(txn.MigrationStarted))
	writeStatusField(&b, "created_at", txn.CreatedAt)
	return []byte(b.String()), nil
}

func writeTransaction(path string, txn updateTransaction) error {
	_, err := writeTransactionState(path, txn)
	return err
}

func writeTransactionState(path string, txn updateTransaction) (bool, error) {
	data, err := transactionData(txn)
	if err != nil {
		return false, err
	}
	return writeAtomicFileState(path, data, 0o600)
}

func markTransactionDone(path string, txn updateTransaction) error {
	txn.Phase = PhaseDone
	if err := writeTransaction(path, txn); err != nil {
		return fmt.Errorf("persist completed update transaction: %w", err)
	}
	return nil
}

func removeDoneTransaction(path, jobID string) error {
	txn, err := readTransaction(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read completed update transaction: %w", err)
	}
	if txn.JobID != jobID {
		return fmt.Errorf("completed update transaction belongs to job %q, want %q", txn.JobID, jobID)
	}
	if txn.Phase != PhaseDone {
		return nil
	}
	if _, err := removeDurable(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove completed update transaction: %w", err)
	}
	return nil
}

func readTransaction(path string) (updateTransaction, error) {
	fields, err := readStatusFields(path)
	if err != nil {
		return updateTransaction{}, err
	}
	format, err := strconv.Atoi(fields["format_version"])
	if err != nil || format != 1 {
		return updateTransaction{}, fmt.Errorf("invalid transaction format %q", fields["format_version"])
	}
	jobID, err := requiredStatusField(fields, "job_id")
	if err != nil {
		return updateTransaction{}, fmt.Errorf("invalid transaction job ID: %w", err)
	}
	if !validJobID(jobID) {
		return updateTransaction{}, fmt.Errorf("invalid transaction job ID %q", jobID)
	}
	legacy, err := strconv.ParseBool(fields["legacy"])
	if err != nil {
		return updateTransaction{}, fmt.Errorf("invalid transaction legacy flag: %w", err)
	}
	wasActive, err := strconv.ParseBool(fields["service_was_active"])
	if err != nil {
		return updateTransaction{}, fmt.Errorf("invalid transaction service state: %w", err)
	}
	migrationStarted, err := strconv.ParseBool(fields["migration_started"])
	if err != nil {
		return updateTransaction{}, fmt.Errorf("invalid transaction migration state: %w", err)
	}
	txn := updateTransaction{
		FormatVersion:    format,
		JobID:            jobID,
		Phase:            Phase(fields["phase"]),
		PreviousTarget:   strings.TrimSpace(fields["previous_target"]),
		CandidateTarget:  strings.TrimSpace(fields["candidate_target"]),
		CandidateDir:     cleanOptionalPath(fields["candidate_dir"]),
		TempRoot:         cleanOptionalPath(fields["temp_root"]),
		BackupDir:        cleanOptionalPath(fields["backup_dir"]),
		Legacy:           legacy,
		ServiceManager:   strings.TrimSpace(fields["service_manager"]),
		ServiceWasActive: wasActive,
		MigrationStarted: migrationStarted,
		CreatedAt:        strings.TrimSpace(fields["created_at"]),
	}
	if txn.CreatedAt != "" {
		if _, err := time.Parse(time.RFC3339, txn.CreatedAt); err != nil {
			return updateTransaction{}, fmt.Errorf("invalid transaction creation time: %w", err)
		}
	}
	if !validUpdatePhase(txn.Phase) {
		return updateTransaction{}, fmt.Errorf("invalid transaction phase %q", txn.Phase)
	}
	if txn.CandidateTarget != "" && !validReleaseTarget(txn.CandidateTarget) {
		return updateTransaction{}, fmt.Errorf("invalid candidate target %q", txn.CandidateTarget)
	}
	if txn.PreviousTarget != "" && !validReleaseTarget(txn.PreviousTarget) {
		return updateTransaction{}, fmt.Errorf("invalid previous target %q", txn.PreviousTarget)
	}
	return txn, nil
}

func cleanOptionalPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return filepath.Clean(raw)
}

func writePrivateFile(path string, data []byte) error {
	return writeAtomicFile(path, data, 0o600)
}

func writeAtomicFile(path string, data []byte, mode os.FileMode) error {
	_, err := writeAtomicFileState(path, data, mode)
	return err
}

func writeAtomicFileState(path string, data []byte, mode os.FileMode) (bool, error) {
	return writeAtomicFileWithSync(path, data, mode, syncDirectory)
}

func writeAtomicFileWithSync(path string, data []byte, mode os.FileMode, syncDir func(string) error) (bool, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return false, err
	}
	return true, syncDir(dir)
}

func removeDurable(path string) (bool, error) {
	return removeDurableWithSync(path, syncDirectory)
}

func removeDurableWithSync(path string, syncDir func(string) error) (bool, error) {
	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, syncDir(filepath.Dir(path))
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

func validJobID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func normalizedServiceManager(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "auto"
	}
	return strings.TrimSpace(raw)
}

func validUpdatePhase(phase Phase) bool {
	return updatePhaseOrder(phase) >= 0
}

func updatePhaseOrder(phase Phase) int {
	switch phase {
	case PhaseQueued:
		return 0
	case PhaseLocked:
		return 1
	case PhaseDownload:
		return 2
	case PhaseValidate:
		return 3
	case PhasePrepared:
		return 4
	case PhaseStopping:
		return 5
	case PhaseSwitching:
		return 6
	case PhaseMigrating:
		return 7
	case PhaseStarting:
		return 8
	case PhaseCleanup:
		return 9
	case PhaseDone:
		return 10
	default:
		return -1
	}
}
