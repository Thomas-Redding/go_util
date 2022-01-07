package fileServer

import (
  "time"
)

type Task struct {
  Paths []string
  ContinueChannel chan bool
  EnqueueTime int64
  IsComplete bool
  Priority int64
  RoutineId string
}

func MakeTask(routineId string, paths []string, priority int64, isComplete bool) *Task {
  return &Task{
    Paths: paths,
    ContinueChannel: make(chan bool),
    EnqueueTime: time.Now().UnixNano(),
    IsComplete: isComplete,
    Priority: priority,
    RoutineId: routineId,
  }
}
