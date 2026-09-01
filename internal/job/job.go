// Package job
package job

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"slices"
	appContext "sonary/internal/context"
	"sonary/internal/database"
	"sonary/internal/ffmpeg"
	"sonary/internal/track"
	"sonary/internal/websocket"
	"sonary/utils"
	"strconv"
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
	TaskConvertTracks       = "convert_tracks"
	TaskConvertTrack        = "convert_track"
)

type Job struct {
	ID           int
	ParentID     sql.NullInt64
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
	ParentID *int
	TaskType *string
	Payload  *string
	Status   []string
}

type JobPath struct {
	Path string `json:"path"`
}

type JobConvertParams struct {
	Format        string `json:"format"`
	Mode          string `json:"mode"`
	Quality       string `json:"quality"`
	IncludePregap bool   `json:"pregap"`
}

type JobConvertTracks struct {
	UserID   string `json:"user_id"`
	TracksID []int  `json:"tracks"`
	JobConvertParams
}

type JobConvertTrack struct {
	UserID  string `json:"user_id"`
	TrackID int    `json:"track"`
	JobConvertParams
}

// Enqueue inserts a new background task
func Enqueue(db *sql.DB, parentID sql.NullInt64, taskType string, payload any) (int64, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	query := `INSERT INTO jobs (parent_id, task_type, payload, status) VALUES (?, ?, ?, ?)`
	res, err := db.Exec(query, parentID, taskType, string(payloadBytes), StatusPending)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

// Get trying to find recent job
func Get(db *sql.DB, filter JobFilter) (*Job, error) {
	jobs, err := GetJobs(db, filter, true)
	if err != nil {
		return nil, err
	}
	return &jobs[0], nil
}

// Get trying to find recent job
func GetJobs(db *sql.DB, filter JobFilter, isSingle bool) ([]Job, error) {
	query := `
		SELECT id, parent_id, task_type, payload, status, result,
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
		if filter.ParentID != nil {
			conditions = append(conditions, "parent_id = ?")
			args = append(args, *filter.ParentID)
		}

		if filter.TaskType != nil {
			conditions = append(conditions, "task_type = ?")
			args = append(args, *filter.TaskType)
		}

		if filter.Payload != nil {
			conditions = append(conditions, "payload = ?")
			args = append(args, *filter.Payload)
		}

		if len(filter.Status) > 0 {
			// Build a slice of '?' characters matching the length of slice
			placeholders := make([]string, len(filter.Status))
			for i := range placeholders {
				placeholders[i] = "?"
			}
			conditions = append(conditions, "status IN ("+strings.Join(placeholders, ", ")+")")
			for _, v := range filter.Status {
				args = append(args, v)
			}
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	} else {
		return nil, nil
	}

	query += " ORDER BY created_at ASC"
	if isSingle {
		query += " LIMIT 1"
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var jobs []Job

	for rows.Next() {
		var parentID sql.NullInt64
		var payloadStr string
		var resultStr sql.NullString

		var job Job
		err := rows.Scan(
			&job.ID,
			&parentID,
			&job.TaskType,
			&payloadStr,
			&job.Status,
			&resultStr,
			&job.ErrorMessage,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if parentID.Valid {
			job.ParentID = parentID
		}
		job.Payload = json.RawMessage(payloadStr)
		if resultStr.Valid {
			job.Result = json.RawMessage(resultStr.String)
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
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
		SELECT id, parent_id, task_type, payload, status
		FROM jobs
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT 1
	`

	err = tx.QueryRow(query, StatusPending).Scan(&job.ID, &job.ParentID, &job.TaskType, &payloadStr, &job.Status)
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
			defer func() {
				if r := recover(); r != nil {
					//log.Printf("worker %d panic: %v", workerID, r)
					log.Printf("worker %d panic: %v\n%s", workerID, r, debug.Stack())
				}
			}()

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

					result, err := processTask(db, job, workerID)
					if err != nil {
						if errors.Is(err, ErrTaskWait) {
							log.Printf("[Worker %d] Job %d is waiting for other task: %v", workerID, job.ID, err)
							time.Sleep(3 * time.Second) // wait some time
							if err := UpdateStatus(db, job.ID, StatusPending, nil, ""); err != nil {
								log.Printf("UpdateStatus error: %v", err)
							}
						} else if errors.Is(err, ErrTaskWaitChildren) {
							// do nothing
						} else {
							log.Printf("[Worker %d] Job %d Failed: %v", workerID, job.ID, err)
							if err := UpdateStatus(db, job.ID, StatusFailed, nil, err.Error()); err != nil {
								log.Printf("UpdateStatus error: %v", err)
							}
						}
					} else {
						log.Printf("[Worker %d] Job %d Completed", workerID, job.ID)
						if err := UpdateStatus(db, job.ID, StatusCompleted, result, ""); err != nil {
							log.Printf("UpdateStatus error: %v", err)
						}
					}
					if job.TaskType == TaskConvertTrack {
						// check if all convert tasks completed
						trackJobs, err := GetJobs(db, JobFilter{
							ParentID: utils.Ptr(int(job.ParentID.Int64)),
							TaskType: utils.Ptr(TaskConvertTrack),
						}, false)
						if err != nil {
							log.Printf("[Worker %d] Job %v get job list error: %v", workerID, job.ID, err)
							continue
						}
						trackJobsCompeted := make([]Job, 0, len(trackJobs))
						for _, j := range trackJobs {
							if j.Status == StatusCompleted {
								trackJobsCompeted = append(trackJobsCompeted, j)
							}
						}
						if len(trackJobs) == len(trackJobsCompeted) {
							// parent job is completed
							log.Printf("[Worker %d] Job %d Completed", workerID, job.ParentID.Int64)
							if err := UpdateStatus(db, int(job.ParentID.Int64), StatusCompleted, nil, ""); err != nil {
								log.Printf("UpdateStatus error: %v", err)
							}
						}
					}
				}
			}
		}(i)
	}
}

var ErrTaskWait = errors.New("job wait for other task")
var ErrTaskWaitChildren = errors.New("task is waiting for children to execute")

func processTask(db *sql.DB, job *Job, workerID int) (any, error) {
	if job.TaskType == TaskSyncDirectories {
		res, err := track.SyncDirectories()
		if err != nil {
			log.Printf("[Job %v] Sync directories error: %v", job.ID, err)
			return nil, err
		}
		log.Printf("[Job %v] Sync directories result: %v", job.ID, res)

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
			_, err := Enqueue(db, sql.NullInt64{Int64: int64(job.ID), Valid: true},
				TaskScanDirectoryTracks, JobPath{Path: dir.Path})
			if err != nil {
				log.Printf("[Job %v] Add job error: %v", job.ID, err)
				return nil, err
			}
			total++
		}

		appContext.ResetImportContext()
		ct := appContext.GetImportContext()
		ct.Progress.Total = total

		return res, nil
	} else if job.TaskType == TaskScanDirectoryTracks {
		// directory scanning job must be completed
		if !job.ParentID.Valid {
			return nil, ErrTaskWait
		}
		jobDirScan, err := Get(db, JobFilter{ID: utils.Ptr(int(job.ParentID.Int64))})
		if err != nil {
			log.Printf("[Job %v] Get job error: %v", job.ID, err)
			return nil, err
		}
		if jobDirScan != nil && jobDirScan.TaskType == TaskSyncDirectories &&
			slices.Contains([]string{StatusPending, StatusRunning}, jobDirScan.Status) {
			return nil, ErrTaskWait
		}

		ct := appContext.GetImportContext()

		var jobPayload JobPath
		err = json.Unmarshal(job.Payload, &jobPayload)
		if err != nil {
			log.Printf("[Job %v] JSON unmarshal error: %v", job.ID, err)
			return nil, err
		}

		defer func() {
			newProcessed := int(ct.Progress.Processed.Add(1))
			oldProcessed := newProcessed - 1

			oldPercent := utils.GetPercent(oldProcessed, ct.Progress.Total)
			newPercent := utils.GetPercent(newProcessed, ct.Progress.Total)

			if newPercent > oldPercent {
				hub := websocket.GetHub()
				hub.Send <- websocket.ProgressEvent{
					Type:     appContext.EventImportProgressUpdate,
					Progress: newPercent,
				}
			}
		}()

		err = track.ScanTracksInDir(jobPayload.Path, ffmpeg.NewFFmpeg())
		if err != nil {
			log.Printf("[Job %v] Scan directory tracks error: %v", job.ID, err)
			return nil, err
		}

		return nil, nil
	} else if job.TaskType == TaskConvertTracks {
		var jobPayload JobConvertTracks
		if err := json.Unmarshal(job.Payload, &jobPayload); err != nil {
			log.Printf("[Job %v] JSON unmarshal error: %v", job.ID, err)
			return nil, err
		}

		total := len(jobPayload.TracksID)
		if total == 0 {
			return "No tracks to convert", nil
		}

		for _, trackID := range jobPayload.TracksID {
			// create scan job for each track
			_, err := Enqueue(db, sql.NullInt64{Int64: int64(job.ID), Valid: true},
				TaskConvertTrack, JobConvertTrack{
					UserID:  jobPayload.UserID,
					TrackID: trackID,
					JobConvertParams: JobConvertParams{
						Format:        jobPayload.Format,
						Mode:          jobPayload.Mode,
						Quality:       jobPayload.Quality,
						IncludePregap: jobPayload.IncludePregap,
					},
				})
			if err != nil {
				log.Printf("[Job %v] Add job error: %v", job.ID, err)
				return nil, err
			}
		}

		appContext.ResetConvertProgress()
		ct := appContext.GetConvertContext()
		ct.Progress.Total = total

		return nil, ErrTaskWaitChildren
	} else if job.TaskType == TaskConvertTrack {
		var jobPayload JobConvertTrack
		err := json.Unmarshal(job.Payload, &jobPayload)
		if err != nil {
			log.Printf("[Job %v] JSON unmarshal error: %v", job.ID, err)
			return nil, err
		}

		readDB := database.Reader()
		t, err := database.GetTrack(readDB, jobPayload.TrackID)
		if err != nil {
			log.Printf("[Job %v] Load track error: %v", job.ID, err)
			return nil, err
		}

		// check if audio file exists (cannot use Open, because ffmpeg can open too)
		if _, err := os.Stat(t.Path); os.IsNotExist(err) {
			log.Printf("[Job %v] Source file not found: %v", job.ID, err)
			return nil, err
		}

		ct := appContext.GetConvertContext()

		defer func() {
			newProcessed := int(ct.Progress.Processed.Add(1))
			hub := websocket.GetHub()
			hub.Send <- websocket.ProgressConvertEvent{
				BaseEvent: websocket.BaseEvent{UserID: jobPayload.UserID},
				Type:      appContext.EventConvertProgressUpdate,
				Total:     ct.Progress.Total,
				Processed: newProcessed,
				Progress:  utils.GetPercent(newProcessed, ct.Progress.Total),
			}
		}()

		var tmpPath string
		if t.FileType == "MP3" && jobPayload.Format == "mp3" {
			tmpFile, err := os.CreateTemp("", "track_"+strconv.Itoa(t.ID)+"_direct_*.mp3")
			if err != nil {
				log.Printf("[Job %v] Failed to create temp file: %v", job.ID, err)
				return nil, err
			}
			tmpPath = tmpFile.Name()

			srcFile, err := os.Open(t.Path)
			if err != nil {
				tmpFile.Close()
				os.Remove(tmpPath)
				log.Printf("[Job %v] Failed to open source file: %v", job.ID, err)
				return nil, err
			}

			_, err = io.Copy(tmpFile, srcFile)
			srcFile.Close()
			tmpFile.Close()

			if err != nil {
				os.Remove(tmpPath)
				log.Printf("[Job %v] Direct copy failed: %v", job.ID, err)
				return nil, err
			}

			hub := websocket.GetHub()
			hub.Send <- websocket.ProgressTrackConvertEvent{
				BaseEvent:  websocket.BaseEvent{UserID: jobPayload.UserID},
				Type:       appContext.EventConvertTrackProgressUpdate,
				Progress:   100,
				Status:     websocket.ConvertStatusCompleted,
				TrackID:    t.ID,
				TrackTitle: t.Title,
			}

			log.Printf("[Job %v] Track is already MP3. Copied directly without FFmpeg.", job.ID)
		} else {
			ff := ffmpeg.NewFFmpeg()
			tmpPath, err = ff.ConvertFile(t, track.ConvertParams{
				Format:        jobPayload.Format,
				Mode:          jobPayload.Mode,
				Quality:       jobPayload.Quality,
				IncludePregap: jobPayload.IncludePregap,
			}, jobPayload.UserID)
			if err != nil {
				log.Printf("[Job %v] Conversion method failed: %v", job.ID, err)
			}
		}

		ct.Jobs.Store(job.ID, tmpPath)

		return nil, nil
	}
	return nil, fmt.Errorf("unsupported task context")
}
