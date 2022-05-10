package bt

import (
	"net/http"
	"sync"

	"wuyrush.io/gtr/bcodec"
)

/*
Bittorrent functionality engine.

shall it be goroutine safe? likely so
*/
type Bter struct {
	HTTP *http.Client
	// TODO factor below to a dedicated entity - JobStore
	Jobs *JobStore
}

/*
TODO

bter behaviors:
    CreateJob
    DelJob
    StartJob
    StopJob
    JobProgress
        - need to monitor and manage the progress of jobs as well
    ListJobs
*/

/*
Creates one or more bittorrent download job.

It queues the jobs for start after creation by default.

For now it just creates and starts the jobs right away.

1. create jobs from torrent, each with an id to be consumed by other components of this software; We shall use a different, more human-friendly set of ids on UI.
    1. avoid duplicated jobs by checking info hash: merge data from given torrents to that of existing job instead
2. submit each job for execution in concurrent.
3. actual job execution
    one job per goroutine
    limit # network connections used to:
        communicate with tracker (announce / scrape)
        exchange bytes with peers

*/
func (bter *Bter) CreateJob(torrents ...*bcodec.Torrent) {

}

// A bittorent download job.
type Job struct {
	*bcodec.Torrent
	ID     string
	Status JobStatus
}

type JobStatus string

const (
	JobStatusQueued      JobStatus = "Queued"
	JobStatusDownlaoding JobStatus = "Downloading"
	JobStatusStopped     JobStatus = "Stopped"
	JobStatusCompleted   JobStatus = "Completed"
)

type JobStore struct {
	// TODO we expect per-job update will be frequent in our case, so maybe switch to https://github.com/orcaman/concurrent-map at some point
	jobs map[string]*Job
	// mutex guarding jobs map
	mtx *sync.Mutex
}

func NewJobStore() *JobStore {
	return &JobStore{
		jobs: make(map[string]*Job),
		mtx:  &sync.Mutex{},
	}
}
