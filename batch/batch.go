package batch

import "sync"

type Executor[J, R any] struct {
	workerCount int
	jobChan     chan J
	resultChan  chan R
	results     []R
	resultMu    sync.Mutex
	workerWg    sync.WaitGroup
}

func NewExecutor[J, R any](workerCount int) Executor[J, R] {
	return Executor[J, R]{
		workerCount: workerCount,
		jobChan:     make(chan J),
		resultChan:  make(chan R),
	}
}

func (e *Executor[J, R]) StartWorkers(handler func(J) R) {
	e.workerWg.Add(e.workerCount)

	// Start our worker goroutines. They'll grab jobs
	// from e.jobChan until it's closed by Wait.
	for range e.workerCount {
		go func(jobs chan J, results chan R) {
			defer e.workerWg.Done()

			for job := range jobs {
				results <- handler(job)
			}
		}(e.jobChan, e.resultChan)
	}

	// If we don't start reading from e.resultChan and collecting the results,
	// the workers will lock up after their first job is completed.
	go func() {
		e.resultMu.Lock()
		defer e.resultMu.Unlock()

		for result := range e.resultChan {
			e.results = append(e.results, result)
		}
	}()
}

func (e *Executor[J, R]) AddJobs(newJobs ...J) {
	for _, job := range newJobs {
		e.jobChan <- job
	}
}

func (e *Executor[J, R]) Wait() []R {
	// We're done submitting jobs.
	close(e.jobChan)

	// When the workers are all done, we can close the results channel.
	e.workerWg.Wait()
	close(e.resultChan)

	// Now we can retrieve the results that Start has been collecting.
	e.resultMu.Lock()
	defer e.resultMu.Unlock()
	return e.results
}
