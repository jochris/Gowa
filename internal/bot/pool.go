package bot

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"gowa/internal/command"
	"gowa/internal/util"
)

type commandJob struct {
	cmd     *command.Command
	ctx     *command.Context
	trigger string
}

type WorkerPool struct {
	jobs     chan *commandJob
	wg       sync.WaitGroup
	workers  int
	isStop   atomic.Bool
	stopOnce sync.Once
}

func NewWorkerPool(queueSize int) *WorkerPool {
	workers := runtime.NumCPU()
	if workers < 4 {
		workers = 4
	}
	if workers > 12 {
		workers = 12
	}

	p := &WorkerPool{
		jobs:    make(chan *commandJob, queueSize),
		workers: workers,
	}

	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.workerLoop(i)
	}

	util.Success("Pool", "Worker Pool aktif: %d threads | Queue Buffer: %d", workers, queueSize)
	return p
}

func (p *WorkerPool) Submit(cmd *command.Command, ctx *command.Context, trigger string) bool {
	if p.isStop.Load() {
		return false
	}

	select {
	case p.jobs <- &commandJob{cmd: cmd, ctx: ctx, trigger: trigger}:
		return true
	default:
		go p.executeJob(&commandJob{cmd: cmd, ctx: ctx, trigger: trigger})
		return true
	}
}

func (p *WorkerPool) workerLoop(id int) {
	defer p.wg.Done()

	for job := range p.jobs {
		p.executeJob(job)
	}
}

func (p *WorkerPool) executeJob(job *commandJob) {
	defer func() {
		if r := recover(); r != nil {
			util.Error("PANIC", "Error fatal mengeksekusi %s: %v", job.cmd.Name, r)
			_ = job.ctx.Reply(fmt.Sprintf("*Error:* Terjadi kesalahan internal saat memproses `%s%s`.", job.ctx.BotPrefix, job.trigger))
		}
	}()

	if err := job.cmd.Execute(job.ctx); err != nil {
		util.Warn("CMD", "Eksekusi %s error: %v", job.cmd.Name, err)
		_ = job.ctx.Reply(fmt.Sprintf("*Gagal:* %v", err))
	}
}

func (p *WorkerPool) Stop() {
	p.stopOnce.Do(func() {
		p.isStop.Store(true)
		close(p.jobs)
		p.wg.Wait()
	})
}
