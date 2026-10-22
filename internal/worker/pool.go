package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var ErrPoolFull = errors.New("heavy task worker pool is at capacity, please try again shortly")

// TaskPool represents a bounded concurrent worker pool for resource-heavy operations
// such as Chromedp PDF vector export and AI mindmap rendering.
type TaskPool struct {
	tasks      chan func(ctx context.Context) error
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	maxWorkers int
}

var (
	globalPDFTaskPool *TaskPool
	poolOnce          sync.Once
)

// GetPDFTaskPool returns the global bounded worker pool for heavy PDF/Image export tasks.
// Default capacity: Max 4 concurrent headless PDF engines with a queue size of 20 tasks.
func GetPDFTaskPool() *TaskPool {
	poolOnce.Do(func() {
		globalPDFTaskPool = NewTaskPool(4, 20)
	})
	return globalPDFTaskPool
}

// NewTaskPool initializes a worker pool with a fixed worker count and queued buffer channel.
func NewTaskPool(maxWorkers, queueCapacity int) *TaskPool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &TaskPool{
		tasks:      make(chan func(ctx context.Context) error, queueCapacity),
		ctx:        ctx,
		cancel:     cancel,
		maxWorkers: maxWorkers,
	}

	for i := 0; i < maxWorkers; i++ {
		pool.wg.Add(1)
		workerID := i + 1
		go pool.worker(workerID)
	}

	return pool
}

func (p *TaskPool) worker(id int) {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.tasks:
			if !ok {
				return
			}
			taskCtx, cancel := context.WithTimeout(p.ctx, 60*time.Second)
			if err := task(taskCtx); err != nil {
				log.Printf("Worker Pool %d task error: %v", id, err)
			}
			cancel()
		}
	}
}

// Submit enqueues a heavy task. Returns ErrPoolFull if queue capacity is exceeded.
func (p *TaskPool) Submit(task func(ctx context.Context) error) error {
	select {
	case p.tasks <- task:
		return nil
	default:
		return ErrPoolFull
	}
}

// Shutdown gracefully waits for running tasks to complete.
func (p *TaskPool) Shutdown() {
	p.cancel()
	close(p.tasks)
	p.wg.Wait()
}
