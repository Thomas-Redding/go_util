package fileScheduler

import (
  "container/heap"
  "sync"
)

// A thread safe priority queue for Tasks objects.
// It supports five functions:
// pq = makeTaskPriorityQueue() - creates a new priority queue
// pq.Length() - returns the number of items in the priority queue
// pq.Push(&newTask) - add an task to the priority queue
// pq.Peek - Peek at the top task from the priority queue
// pq.Pop - pop (and return) a task from the priority queue

// Significant code copied from
// https://pkg.go.dev/container/heap#example-package-PriorityQueue
/*
 someTask =
 pq := MakeTaskPriorityQueue(),
 pq.Push(someTask)
 */

type TaskPriorityQueue struct {
  tasks []*Task
  mu sync.Mutex
}

func MakeTaskPriorityQueue() TaskPriorityQueue {
  return TaskPriorityQueue{tasks: make([]*Task, 0)}
}

func (pq TaskPriorityQueue) Length() int {
  pq.mu.Lock()
  defer pq.mu.Unlock()
  return pq.Len()
}

func (pq *TaskPriorityQueue) Push(x interface{}) {
  pq.mu.Lock()
  defer pq.mu.Unlock()
  n := len(pq.tasks)
  task := x.(*Task)
  pq.tasks = append(pq.tasks, task)
  pq.update(task, n)
}

func (pq *TaskPriorityQueue) Peek() interface{} {
  pq.mu.Lock()
  defer pq.mu.Unlock()
  return pq.tasks[len(pq.tasks)-1]
}

func (pq *TaskPriorityQueue) Pop() interface{} {
  pq.mu.Lock()
  defer pq.mu.Unlock()
  old := pq.tasks
  n := len(old)
  task := old[n-1]
  old[n-1] = nil  // avoid memory leak
  pq.tasks = old[0 : n-1]
  return task
}

/********** Private **********/

func (pq TaskPriorityQueue) Len() int {
  return len(pq.tasks)
}

func (pq TaskPriorityQueue) Less(i, j int) bool {
  return pq.tasks[i].Priority > pq.tasks[j].Priority
}

func (pq TaskPriorityQueue) Swap(i, j int) {
  pq.tasks[i], pq.tasks[j] = pq.tasks[j], pq.tasks[i]
}

// Call after adding a Task or modifying its priority.
func (pq *TaskPriorityQueue) update(task *Task, index int) {
  heap.Fix(pq, index)
}
