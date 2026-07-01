// Package job
package job

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sonary/internal/database"
	"sonary/internal/lib"
	"sonary/internal/track"
	"sonary/internal/websocket"
	"sonary/utils"
	"strings"
	"time"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

const (
	TaskSyncDirectories     = "sync_directories"
	TaskScanDirectoryTracks = "scan_directory_tracks"
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

type JobPath struct {
	Path string `json:"path"`
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

var ErrJobCancelled = errors.New("job was cancelled unexpectedly")

// CancelJobs sets all running or pending jobs to error
func CancelJobs(db *sql.DB) error {
	log.Println("Cancelling old jobs ...")
	query := `UPDATE jobs SET status = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP
		WHERE status = ? OR status = ?`
	_, err := db.Exec(query, StatusFailed, ErrJobCancelled.Error(), StatusPending, StatusRunning)
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

					result, err := processTask(db, job)
					if err != nil {
						if errors.Is(err, ErrTaskWait) {
							log.Printf("[Worker %d] Job %d is waiting for other task: %v", workerID, job.ID, err)
							time.Sleep(3 * time.Second) // wait some time
							_ = UpdateStatus(db, job.ID, StatusPending, nil, "")
						} else {
							log.Printf("[Worker %d] Job %d Failed: %v", workerID, job.ID, err)
							_ = UpdateStatus(db, job.ID, StatusFailed, nil, err.Error())
						}
					} else {
						log.Printf("[Worker %d] Job %d Completed", workerID, job.ID)
						_ = UpdateStatus(db, job.ID, StatusCompleted, result, "")
					}
				}
			}
		}(i)
	}
}

var ErrTaskWait = errors.New("job wait for other task")

var Broadcast = make(chan []byte) // Broadcast channel

func processTask(db *sql.DB, job *Job) (any, error) {
	if job.TaskType == TaskSyncDirectories {
		res, err := track.SyncDirectories()
		if err != nil {
			log.Printf("[Job %v] Sync directories error: %v", job.ID, err)
			return nil, err
		}
		log.Printf("[Job %v] Sync directories result: %v", res)

		dirs, err := database.GetDirectories(db)
		if err != nil {
			log.Printf("[Job %v] Load directories error: %v", job.ID, err)
			return nil, err
		}

		total := 0
		for _, dir := range dirs {
			if dir.LastScan != 0 {
				continue
			}

			// create scan job for each directory
			_, err := Enqueue(db, TaskScanDirectoryTracks, JobPath{Path: dir.Path})
			if err != nil {
				log.Printf("[Job %v] Add job error: %v", job.ID, err)
				return nil, err
			}
			total++
		}

		ct := lib.GetImportContext(true)
		ct.Progress.Total = total

		return res, nil
	} else if job.TaskType == TaskScanDirectoryTracks {
		// directory scanning job must be completed
		jobDirScan, err := Get(db, JobFilter{
			TaskType: utils.Ptr(TaskSyncDirectories),
			Status:   utils.Ptr([]string{StatusPending, StatusRunning}),
		})
		if err != nil {
			log.Printf("[Job %v] Get job error: %v", job.ID, err)
			return nil, err
		}
		if jobDirScan != nil {
			return nil, ErrTaskWait
		}

		ct := lib.GetImportContext(false)

		var dirScanPayload JobPath
		err = json.Unmarshal(job.Payload, &dirScanPayload)
		if err != nil {
			log.Printf("[Job %v] JSON unmarshal error: %v", job.ID, err)
			return nil, err
		}

		err = track.ScanTracksInDir(dirScanPayload.Path)
		if err != nil {
			log.Printf("[Job %v] Scan directory tracks error: %v", job.ID, err)
			return nil, err
		}
		newProcessed := int(ct.Progress.Processed.Add(1))
		oldProcessed := newProcessed - 1

		oldPercent := utils.GetPercent(oldProcessed, ct.Progress.Total)
		newPercent := utils.GetPercent(newProcessed, ct.Progress.Total)

		if newPercent > oldPercent {
			hub := websocket.GetHub()
			hub.Broadcast <- websocket.ProgressEvent{
				Type:     lib.EventProgressUpdate,
				Progress: newPercent,
			}
		}

		return nil, nil
	}
	return nil, fmt.Errorf("unsupported task context")
}
