// Package job
package job

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sonary/internal/lib"
	"strings"
	"time"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type Job struct {
	ID           int
	TaskType     string
	Payload      json.RawMessage
	Status       string
	Result       json.RawMessage
	ErrorMessage sql.NullString
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type JobFilter struct {
	ID       *int
	TaskType *string
	Payload  *string
	Status   *[]string
}

// Enqueue inserts a new background task
func Enqueue(db *sql.DB, taskType string, payload any) (int64, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	query := `INSERT INTO jobs (task_type, payload, status) VALUES (?, ?, ?)`
	res, err := db.Exec(query, taskType, string(payloadBytes), StatusPending)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

// Get trying to find recent job
func Get(db *sql.DB, filter JobFilter) (*Job, error) {
	query := `
		SELECT id, task_type, payload, status, result,
			error_message, created_at, updated_at
		FROM jobs
	`

	var (
		conditions []string
		args       []any
	)

	// ID in priority
	if filter.ID != nil {
		conditions = append(conditions, "id = ?")
		args = append(args, *filter.ID)
	} else {
		if filter.TaskType != nil {
			conditions = append(conditions, "task_type = ?")
			args = append(args, *filter.TaskType)
		}

		if filter.Payload != nil {
			conditions = append(conditions, "payload = ?")
			args = append(args, *filter.Payload)
		}

		if filter.Status != nil && len(*filter.Status) > 0 {
			// Build a slice of '?' characters matching the length of slice
			placeholders := make([]string, len(*filter.Status))
			for i := range placeholders {
				placeholders[i] = "?"
			}
			conditions = append(conditions, "status IN ("+strings.Join(placeholders, ", ")+")")
			for _, v := range *filter.Status {
				args = append(args, v)
			}
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	} else {
		return nil, nil
	}

	query += " ORDER BY created_at ASC LIMIT 1"

	var job Job
	var payloadStr string
	var resultStr sql.NullString

	err := db.QueryRow(query, args...).Scan(
		&job.ID,
		&job.TaskType,
		&payloadStr,
		&job.Status,
		&resultStr,
		&job.ErrorMessage,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	job.Payload = json.RawMessage(payloadStr)
	if resultStr.Valid {
		job.Result = json.RawMessage(resultStr.String)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &job, nil
}

// GetNext fetches and safely locks a job using an Immediate Transaction block
func GetNext(db *sql.DB) (*Job, error) {
	// Start an immediate write transaction. This prevents concurrent workers
	// from reading the same 'pending' rows at the exact same time.
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Find the oldest pending job
	var job Job
	var payloadStr string

	query := `
		SELECT id, task_type, payload, status
		FROM jobs
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT 1
	`

	err = tx.QueryRow(query, StatusPending).Scan(&job.ID, &job.TaskType, &payloadStr, &job.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // No jobs available right now
	}
	if err != nil {
		return nil, err
	}
	job.Payload = json.RawMessage(payloadStr)

	// Lock the job right away within the same transaction block
	updateQuery := `UPDATE jobs SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err = tx.Exec(updateQuery, StatusRunning, job.ID)
	if err != nil {
		return nil, err
	}

	// Commit unlocks the database file for other workers
	return &job, tx.Commit()
}

// UpdateStatus records the successful result or execution error
func UpdateStatus(db *sql.DB, id int, status string, result any, errMsg string) error {
	var resultStr sql.NullString
	if result != nil {
		resultBytes, err := json.Marshal(result)
		if err != nil {
			return err
		}
		resultStr = sql.NullString{String: string(resultBytes), Valid: true}
	}

	var sqlErr sql.NullString
	if errMsg != "" {
		sqlErr = sql.NullString{String: errMsg, Valid: true}
	}

	query := `UPDATE jobs SET status = ?, result = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, status, resultStr, sqlErr, id)
	return err
}

func StartWorkerPool(ctx context.Context, db *sql.DB, workerCount int) {
	for i := 1; i <= workerCount; i++ {
		go func(workerID int) {
			ticker := time.NewTicker(1000 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					log.Printf("[Worker %d] Stopping...", workerID)
					return
				case <-ticker.C:
					job, err := GetNext(db)
					if err != nil {
						log.Printf("[Worker %d] Error fetching job: %v", workerID, err)
						continue
					}
					if job == nil {
						continue // Queue empty
					}

					log.Printf("[Worker %d] Picked up job %d (%s)", workerID, job.ID, job.TaskType)

					result, err := processTask(job)
					if err != nil {
						log.Printf("[Worker %d] Job %d Failed: %v", workerID, job.ID, err)
						_ = UpdateStatus(db, job.ID, StatusFailed, nil, err.Error())
					} else {
						log.Printf("[Worker %d] Job %d Completed", workerID, job.ID)
						_ = UpdateStatus(db, job.ID, StatusCompleted, result, "")
					}
				}
			}
		}(i)
	}
}

func processTask(job *Job) (any, error) {
	time.Sleep(100 * time.Second)

	if job.TaskType == lib.TaskIndexTrackScan {
		return map[string]any{"rows_processed": 42}, nil
	}
	return nil, fmt.Errorf("unsupported task context")
}
