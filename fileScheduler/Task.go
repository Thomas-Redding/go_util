package fileScheduler

import (
  "time"
)

type Task struct {
  Paths [][]string // paths to lock
  Channel chan bool // callback in which to complete the task
  EnqueueTime int64
  IsComplete bool
  RoutineId string
  Priority int64 // priority queue
}

func MakeTask(routineId string, paths [][]string, priority int64, isComplete bool) *Task {
  return &Task{
    Paths: paths,
    Channel: make(chan bool),
    EnqueueTime: time.Now().UnixNano(),
    IsComplete: isComplete,
    Priority: priority,
    RoutineId: routineId,
  }
}
