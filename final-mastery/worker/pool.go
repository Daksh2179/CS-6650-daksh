package worker

import (
	"context"
	"log"
	"net/http"

	"final-mastery/store"
)

type Job struct {
	PhotoID     string
	AlbumID     string
	Data        []byte
	ContentType string
	Bucket      string
}

type Pool struct {
	jobs    chan Job
	dynamo  *store.DynamoClient
	s3      *store.S3Client
	bucket  string
	workers int
}

func NewPool(dynamo *store.DynamoClient, s3 *store.S3Client, bucket string, workers int) *Pool {
	return &Pool{
		jobs:    make(chan Job, 500),
		dynamo:  dynamo,
		s3:      s3,
		bucket:  bucket,
		workers: workers,
	}
}

func (p *Pool) Start() {
	for i := 0; i < p.workers; i++ {
		go p.run()
	}
}

func (p *Pool) Submit(job Job) {
	p.jobs <- job
}

func (p *Pool) run() {
	for job := range p.jobs {
		ctx := context.Background()

		ct := job.ContentType
		if ct == "" {
			ct = http.DetectContentType(job.Data)
		}

		url, err := p.s3.Upload(ctx, job.Bucket, job.AlbumID, job.PhotoID, job.Data, ct)
		if err != nil {
			log.Printf("s3 upload failed photo=%s: %v", job.PhotoID, err)
			_ = p.dynamo.UpdatePhotoStatus(ctx, job.PhotoID, "failed", "")
			continue
		}

		_ = p.dynamo.UpdatePhotoStatusIfExists(ctx, job.PhotoID, "completed", url)
	}
}