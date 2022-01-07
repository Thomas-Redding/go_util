package fileServer

import (
  "errors"
  "log"
  "time"
)

/*
 * Scheduler supports recursive locks, but every lock must be matched by an unlock.
 */

/********** Scheduler **********/

type Scheduler struct {
  fileTrie FileTrie
  startChannel chan *Task
  endChannel chan *Task
  priorityQueue TaskPriorityQueue
  loggingEnabled uint
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
      if rtn.loggingEnabled > 2 {
        log.Println("Scheduler.go startChannel", task.IsComplete, task.Priority, task.Paths)
      }
      rtn.priorityQueue.Push(task)
    }
  }()
  // Add task completions to the priority queue
  go func() {
    for task := range rtn.endChannel {
      if rtn.loggingEnabled > 2 {
        log.Println("Scheduler.go endChannel", task.IsComplete, task.Priority, task.Paths)
      }
      rtn.priorityQueue.Push(task)
    }
  }()
  go func() {
    counter := 0
    for {
      counter = (counter + 1) % 1000
      time.Sleep(time.Millisecond)
      if rtn.loggingEnabled > 2 {
        log.Println("Scheduler.go loop", rtn.priorityQueue.Length(), rtn.fileTrie.Length())
      }
      for rtn.priorityQueue.Length() != 0 {
        task := rtn.priorityQueue.Peek().(*Task)
        if !task.IsComplete {
          // Task needs to be done. Attempt to acquire locks.
          blocked := false
          for _, path := range task.Paths {
            count, routineId := rtn.fileTrie.ContainsPathOrParent(path)
            if count > 0 {
              if routineId == task.RoutineId {
                if rtn.loggingEnabled > 1 {
                  log.Println("Scheduler.go Share Lock", path)
                }
              } else  {
                // File is locked by a different routine
                if rtn.loggingEnabled > 1 {
                  log.Println("Scheduler.go Locked", path)
                }
                blocked = true
                break
              }
            }
          }
          if blocked {
            if counter == 0 {
              if rtn.loggingEnabled > 2 {
                log.Println("Scheduler.go blocked")
              }
            }
            break
          }
          for _, path := range task.Paths {
            if rtn.loggingEnabled > 1 {
              log.Println("Scheduler.go Add", path)
            }
            rtn.fileTrie.Add(path, task.RoutineId)
          }
          task.ContinueChannel <- true
          rtn.priorityQueue.Pop()
        } else {
          // Task is done. Release all locks.
          for _, path := range task.Paths {
            if rtn.loggingEnabled > 1 {
              log.Println("Scheduler.go Remove", path)
            }
            err := rtn.fileTrie.RemoveWhileExpectingValue(path, task.RoutineId)
            if err != nil && rtn.loggingEnabled > 1 {
              log.Println("Scheduler.go Unlocking Problem", err)
            }
          }
          rtn.priorityQueue.Pop()
        }
      }
    }
  }()
  return rtn
}

func (scheduler *Scheduler) WaitUntilAvailable(routineId string, path string) bool {
  return scheduler.WaitUntilAllAvailable(routineId, []string{path})
}

func (scheduler *Scheduler) WaitUntilAllAvailable(routineId string, paths []string) bool {
  if scheduler.loggingEnabled > 1 {
    log.Println("Scheduler.go WaitUntilAllAvailable", paths)
  }
  task := MakeTask(routineId, paths, time.Now().UnixNano(), false)
  scheduler.startChannel <- task
  shouldContinue := <- task.ContinueChannel
  return shouldContinue
}

func (scheduler *Scheduler) WaitUntilAllAvailableUrgent(routineId string, paths []string) error {
  if scheduler.loggingEnabled > 1 {
    log.Println("Scheduler.go WaitUntilAllAvailableUrgent", paths)
  }
  task := MakeTask(routineId, paths, time.Now().UnixNano() / 2, false)
  scheduler.startChannel <- task
  shouldContinue := <- task.ContinueChannel
  if shouldContinue {
    return nil
  } else {
    return errors.New("Something went wrong")
  }
}

func (scheduler *Scheduler) Done(routineId string, path string) {
  scheduler.DoneAll(routineId, []string{path})
}

func (scheduler *Scheduler) DoneAll(routineId string, paths []string) {
  if scheduler.loggingEnabled > 1 {
    log.Println("Scheduler.go DoneAll", paths)
  }
  task := MakeTask(routineId, paths, -1, true)
  task.IsComplete = true
  scheduler.endChannel <- task
}
