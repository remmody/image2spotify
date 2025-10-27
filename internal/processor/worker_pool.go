package processor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"image2spotify/internal/spotify"

	"github.com/rs/zerolog/log"
)

type DownloadTask struct {
	URL     string
	TrackID string
	Result  chan *spotify.ImageData
}

type WorkerPool struct {
	workers       int
	tasks         chan *DownloadTask
	downloader    *spotify.Downloader
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	activeWorkers int32
}

func NewWorkerPool(workers int, imageTimeout time.Duration) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		workers:    workers,
		tasks:      make(chan *DownloadTask, workers*10),
		downloader: spotify.NewDownloader(imageTimeout),
		ctx:        ctx,
		cancel:     cancel,
	}

	pool.start()
	
	log.Info().
		Int("workers", workers).
		Dur("image_timeout", imageTimeout).
		Int("queue_size", workers*10).
		Msg("Worker pool initialized")
	
	return pool
}

func (p *WorkerPool) start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	atomic.AddInt32(&p.activeWorkers, 1)
	defer atomic.AddInt32(&p.activeWorkers, -1)

	log.Debug().Int("worker_id", id).Msg("Worker started")

	for {
		select {
		case <-p.ctx.Done():
			log.Debug().Int("worker_id", id).Msg("Worker stopped")
			return
		case task, ok := <-p.tasks:
			if !ok {
				log.Debug().Int("worker_id", id).Msg("Worker stopped (channel closed)")
				return
			}
			p.processTask(task)
		}
	}
}

func (p *WorkerPool) processTask(task *DownloadTask) {
	maxRetries := 3
	var data []byte
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * 2 * time.Second
			
			log.Debug().
				Str("track_id", task.TrackID).
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("Retrying download")
			
			select {
			case <-time.After(delay):
			case <-p.ctx.Done():
				return
			}
		}

		data, err = p.downloader.Download(p.ctx, task.URL)
		if err == nil && len(data) > 0 {
			log.Debug().
				Str("track_id", task.TrackID).
				Int("size", len(data)).
				Int("attempt", attempt+1).
				Msg("Download successful")
			break
		}

		if err != nil {
			log.Debug().
				Err(err).
				Str("track_id", task.TrackID).
				Int("attempt", attempt+1).
				Msg("Download failed")
		}
	}

	result := &spotify.ImageData{
		URL:     task.URL,
		TrackID: task.TrackID,
		Data:    data,
	}

	if len(data) == 0 {
		log.Warn().
			Str("track_id", task.TrackID).
			Str("url", task.URL).
			Int("max_attempts", maxRetries+1).
			Msg("Failed to download after all retries")
	}

	select {
	case task.Result <- result:
	case <-p.ctx.Done():
		log.Debug().Str("track_id", task.TrackID).Msg("Context cancelled, discarding result")
	case <-time.After(5 * time.Second):
		log.Warn().
			Str("track_id", task.TrackID).
			Msg("Timeout sending result")
	}
}

func (p *WorkerPool) Submit(task *DownloadTask) bool {
	select {
	case p.tasks <- task:
		return true
	case <-p.ctx.Done():
		log.Debug().Str("track_id", task.TrackID).Msg("Task rejected: context cancelled")
		return false
	}
}

func (p *WorkerPool) GetActiveWorkers() int {
	return int(atomic.LoadInt32(&p.activeWorkers))
}

func (p *WorkerPool) GetQueueSize() int {
	return len(p.tasks)
}

func (p *WorkerPool) Shutdown() {
	log.Info().Msg("Shutting down worker pool")
	close(p.tasks)
	p.cancel()
	p.wg.Wait()
	log.Info().Msg("Worker pool stopped")
}
