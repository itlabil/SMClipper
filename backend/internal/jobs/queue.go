package jobs

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Queue struct {
	db               *pgxpool.Pool
	storagePath      string
	whisperBinPath   string
	whisperModelPath string
	jobChan          chan string
}

func NewQueue(db *pgxpool.Pool, storagePath, whisperBinPath, whisperModelPath string, workerCount int) *Queue {
	q := &Queue{
		db:               db,
		storagePath:      storagePath,
		whisperBinPath:   whisperBinPath,
		whisperModelPath: whisperModelPath,
		jobChan:          make(chan string, 100),
	}

	for i := 0; i < workerCount; i++ {
		go q.worker(i)
	}

	return q
}

func (q *Queue) Enqueue(jobID string) {
	q.jobChan <- jobID
}

func (q *Queue) worker(id int) {
	for jobID := range q.jobChan {
		log.Printf("[worker %d] processing job %s", id, jobID)
		q.processJob(jobID)
	}
}

func (q *Queue) processJob(jobID string) {
	ctx := context.Background()

	var jobType, projectID string
	err := q.db.QueryRow(ctx, `SELECT job_type, project_id FROM jobs WHERE id = $1`, jobID).
		Scan(&jobType, &projectID)
	if err != nil {
		log.Printf("failed to fetch job %s: %v", jobID, err)
		return
	}

	q.updateJobStatus(jobID, "processing", 0, nil)

	var runErr error
	switch jobType {
	case "download":
		runErr = q.runDownload(ctx, jobID, projectID)
	case "transcribe":
		runErr = q.runTranscribe(ctx, jobID, projectID)
	case "render":
		runErr = q.runRender(ctx, jobID, projectID)
	default:
		runErr = errUnknownJobType(jobType)
	}

	if runErr != nil {
		msg := runErr.Error()
		log.Printf("job %s failed: %v", jobID, runErr)
		q.updateJobStatus(jobID, "failed", 0, &msg)
		q.db.Exec(ctx, `UPDATE projects SET status = 'failed', updated_at = now() WHERE id = $1`, projectID)
		return
	}

	q.updateJobStatus(jobID, "done", 100, nil)
}

func (q *Queue) updateJobStatus(jobID, status string, progress int, errMsg *string) {
	ctx := context.Background()
	_, err := q.db.Exec(ctx,
		`UPDATE jobs SET status = $1, progress = $2, error_message = $3, updated_at = now() WHERE id = $4`,
		status, progress, errMsg, jobID,
	)
	if err != nil {
		log.Printf("failed to update job status: %v", err)
	}
}

func errUnknownJobType(t string) error {
	return &unknownJobTypeError{t}
}

type unknownJobTypeError struct{ jobType string }

func (e *unknownJobTypeError) Error() string {
	return "unknown job type: " + e.jobType
}