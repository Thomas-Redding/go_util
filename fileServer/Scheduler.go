package fileServer

import (
  "errors"
  "time"
)

/********** Scheduler **********/

type Scheduler struct {
  lockedPaths PathNode
  fileTrie FileTrie
  startChannel chan *Task
  endChannel chan *Task
  priorityQueue TaskPriorityQueue
}

func MakeScheduler() Scheduler {
  rtn := Scheduler{
    fileTrie: MakeFileTrie(),
    startChannel: make(chan *Task, 0),
    endChannel: make(chan *Task, 0),
    priorityQueue: makeTaskPriorityQueue(),
  }

  // Add tasks to the priority queue
  go func() {
    for task := range rtn.startChannel {
      rtn.priorityQueue.Push(task)
    }
  }()
  // Add task completions to the priority queue
  go func() {
    for task := range rtn.endChannel {
      rtn.priorityQueue.Push(task)
    }
  }()
  go func() {
    for {
      time.Sleep(time.Millisecond) // TODO: Make time.Millisecond
      for rtn.priorityQueue.Length() != 0 {
        task := rtn.priorityQueue.Peek().(*Task)
        if !task.IsComplete {
          // Task needs to be done. Attempt to acquire locks.
          locked := false
          for _, path := range task.Paths {
            if rtn.fileTrie.ContainsPathOrParent(path) {
              locked = true
              break
            }
          }
          if locked {
            break
          }
          for _, path := range task.Paths {
            rtn.fileTrie.Add(path)
          }
          task.ContinueChannel <- true
          rtn.priorityQueue.Pop()
        } else {
          // Task is done. Release all locks.
          for _, path := range task.Paths {
            rtn.fileTrie.Remove(path)
          }
          rtn.priorityQueue.Pop()
        }
      }
    }
  }()
  return rtn
}

func (scheduler *Scheduler) WaitUntilAvailable(path string) bool {
  return scheduler.WaitUntilAllAvailable([]string{path})
}

func (scheduler *Scheduler) WaitUntilAllAvailable(paths []string) bool {
  task := MakeTask(paths, time.Now().UnixNano(), false)
  scheduler.startChannel <- task
  shouldContinue := <- task.ContinueChannel
  return shouldContinue
}

func (scheduler *Scheduler) WaitUntilAllAvailableUrgent(paths []string) error {
  task := MakeTask(paths, -1, false)
  scheduler.startChannel <- task
  shouldContinue := <- task.ContinueChannel
  if shouldContinue {
    return nil
  } else {
    return errors.New("Something went wrong")
  }
}

func (scheduler *Scheduler) Done(path string) {
  scheduler.DoneAll([]string{path})
}

func (scheduler *Scheduler) DoneAll(paths []string) {
  task := MakeTask(paths, 0, true)
  task.IsComplete = true
  scheduler.endChannel <- task
}
